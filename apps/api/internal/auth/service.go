package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"net/netip"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// AccountInitFn is invoked once with the new account's id immediately after
// signup completes. Used to wire up downstream resources (subscription record
// etc.) without auth depending on those packages. Best-effort: a failure
// here is logged but does NOT fail the signup.
type AccountInitFn func(ctx context.Context, accountID uuid.UUID) error

type Service struct {
	repo        *Repository
	jwt         *JWT
	accountInit AccountInitFn
}

func NewService(repo *Repository, jwt *JWT) *Service {
	return &Service{repo: repo, jwt: jwt}
}

// SetAccountInit attaches a post-signup hook. Returns the service to allow
// fluent chaining at construction time. Pass nil to clear.
func (s *Service) SetAccountInit(fn AccountInitFn) *Service {
	s.accountInit = fn
	return s
}

// RequestMeta is request-time context recorded with sessions (audit + abuse detection).
type RequestMeta struct {
	UserAgent string
	IP        netip.Addr
}

// Signup creates an account + first user (owner) and returns initial tokens.
func (s *Service) Signup(ctx context.Context, req SignupRequest, meta RequestMeta) (*AuthResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(req.Password); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.CreateAccountWithOwner(ctx, email, hash, name, req.Locale, req.Country)
	if err != nil {
		return nil, err
	}

	// Best-effort post-signup hook (subscription init etc.). Failure here
	// must not block signup; the worker's safety-net sweep can backfill later.
	if s.accountInit != nil {
		if hookErr := s.accountInit(ctx, user.AccountID); hookErr != nil {
			slog.Warn("accountInit hook failed", "account_id", user.AccountID, "err", hookErr)
		}
	}

	return s.issueTokens(ctx, *user, meta)
}

// Login authenticates by email + password and issues new tokens.
func (s *Service) Login(ctx context.Context, req LoginRequest, meta RequestMeta) (*AuthResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	user, hash, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	ok, err := VerifyPassword(hash, req.Password)
	if err != nil || !ok {
		return nil, ErrInvalidCredentials
	}

	if err := s.repo.UpdateLastLogin(ctx, user.ID); err != nil {
		// non-fatal — login still succeeds
		_ = err
	}
	return s.issueTokens(ctx, *user, meta)
}

// Refresh rotates a refresh token. Detects reuse of an already-rotated token as theft.
func (s *Service) Refresh(ctx context.Context, refreshToken string, meta RequestMeta) (*AuthResponse, error) {
	if refreshToken == "" {
		return nil, ErrInvalidToken
	}
	hash := hashToken(refreshToken)
	session, err := s.repo.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	// Token theft detection: a rotated token being presented again means someone
	// kept a copy. Revoke ALL sessions for this user as a defensive measure.
	if session.RotatedTo != nil {
		_ = s.repo.RevokeAllUserSessions(ctx, session.UserID)
		return nil, ErrSessionRevoked
	}
	if session.RevokedAt != nil {
		return nil, ErrSessionRevoked
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	resp, err := s.issueTokens(ctx, *user, meta)
	if err != nil {
		return nil, err
	}

	// Record rotation on the old session (points to the new one).
	newSessionID, _ := s.resolveLastSessionID(ctx, resp.RefreshToken)
	_ = s.repo.RevokeSession(ctx, session.ID, newSessionID)

	return resp, nil
}

// Logout revokes the session associated with the supplied refresh token.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return ErrInvalidToken
	}
	hash := hashToken(refreshToken)
	session, err := s.repo.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return err
	}
	if session.RevokedAt != nil {
		return nil
	}
	return s.repo.RevokeSession(ctx, session.ID, nil)
}

// Me returns the user associated with an authenticated identity.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (*User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// issueTokens creates a new session row and signs an access token.
func (s *Service) issueTokens(ctx context.Context, user User, meta RequestMeta) (*AuthResponse, error) {
	refreshTok, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("gen refresh token: %w", err)
	}
	expiresAt := time.Now().Add(s.jwt.RefreshTTL())
	_, err = s.repo.CreateSession(ctx, user.ID, hashToken(refreshTok), meta.UserAgent, meta.IP, expiresAt)
	if err != nil {
		return nil, err
	}

	identity := Identity{UserID: user.ID, AccountID: user.AccountID, Role: user.Role}
	accessTok, err := s.jwt.SignAccess(identity)
	if err != nil {
		return nil, fmt.Errorf("sign access: %w", err)
	}

	return &AuthResponse{
		User:                 user,
		AccessToken:          accessTok,
		RefreshToken:         refreshTok,
		AccessTokenExpiresIn: int(s.jwt.AccessTTL().Seconds()),
	}, nil
}

// resolveLastSessionID looks up the session row id of a just-issued refresh token.
// Used to set the rotated_to pointer on the old session.
func (s *Service) resolveLastSessionID(ctx context.Context, refreshToken string) (*uuid.UUID, error) {
	sess, err := s.repo.GetSessionByTokenHash(ctx, hashToken(refreshToken))
	if err != nil {
		return nil, err
	}
	return &sess.ID, nil
}

// ----- validation helpers -----

func normalizeEmail(in string) (string, error) {
	in = strings.TrimSpace(strings.ToLower(in))
	addr, err := mail.ParseAddress(in)
	if err != nil {
		return "", ErrInvalidEmail
	}
	return addr.Address, nil
}

func validatePassword(p string) error {
	if len(p) < 8 {
		return ErrPasswordTooWeak
	}
	var hasLetter, hasDigit bool
	for _, r := range p {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrPasswordTooWeak
	}
	return nil
}
