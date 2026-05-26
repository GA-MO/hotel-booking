package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWT struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWT(secret string, accessTTL, refreshTTL time.Duration) *JWT {
	return &JWT{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (j *JWT) AccessTTL() time.Duration  { return j.accessTTL }
func (j *JWT) RefreshTTL() time.Duration { return j.refreshTTL }

// AccessClaims are the JWT claims for access tokens.
type AccessClaims struct {
	AccountID string `json:"acc"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

// SignAccess signs an access token for the given identity.
func (j *JWT) SignAccess(identity Identity) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		AccountID: identity.AccountID.String(),
		Role:      identity.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   identity.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

// ParseAccess verifies and returns the identity from an access token.
func (j *JWT) ParseAccess(tokenString string) (Identity, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil || !token.Valid {
		return Identity{}, errors.Join(ErrInvalidToken, err)
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: bad sub", ErrInvalidToken)
	}
	accountID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: bad acc", ErrInvalidToken)
	}
	return Identity{
		UserID:    userID,
		AccountID: accountID,
		Role:      claims.Role,
	}, nil
}
