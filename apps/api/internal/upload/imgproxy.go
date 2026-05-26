package upload

// imgproxy URL signing — produces a signed delivery URL of the form:
//
//	{base}/{signature}/{processing_options}/{base64url(source_url)}
//
// where signature = base64url(HMAC-SHA256(key, salt || path)) with `path`
// being everything after `{base}/{signature}` — i.e. the leading "/"
// followed by the options and encoded source.
//
// When IMGPROXY_KEY or IMGPROXY_SALT is empty we emit the literal
// "insecure" placeholder, matching imgproxy's `IMGPROXY_DEVELOPMENT_ERRORS_MODE`
// dev workflow. This is intentional: forgetting to configure key/salt should
// not block local dev, but in production both MUST be set.
//
// Reference: https://docs.imgproxy.net/usage/signing_url

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// ImgproxyConfig is the subset of [config.Config] this signer needs.
type ImgproxyConfig struct {
	BaseURL string // e.g. http://imgproxy:8080 (no trailing slash)
	Key     string // hex-encoded; empty = "insecure" mode
	Salt    string // hex-encoded; empty = "insecure" mode
}

// Imgproxy is safe for concurrent use; holds no per-request state.
type Imgproxy struct {
	cfg    ImgproxyConfig
	key    []byte // hex-decoded; nil when in insecure mode
	salt   []byte // hex-decoded; nil when in insecure mode
}

// NewImgproxy returns a configured signer. Errors only on malformed hex
// in key/salt (an empty key/salt is valid — "insecure" mode).
func NewImgproxy(cfg ImgproxyConfig) (*Imgproxy, error) {
	i := &Imgproxy{cfg: cfg}
	if cfg.Key != "" {
		k, err := hex.DecodeString(cfg.Key)
		if err != nil {
			return nil, fmt.Errorf("decode IMGPROXY_KEY hex: %w", err)
		}
		i.key = k
	}
	if cfg.Salt != "" {
		s, err := hex.DecodeString(cfg.Salt)
		if err != nil {
			return nil, fmt.Errorf("decode IMGPROXY_SALT hex: %w", err)
		}
		i.salt = s
	}
	return i, nil
}

// Transform describes the imgproxy processing options. Zero values are
// omitted from the URL (lets imgproxy use its defaults).
type Transform struct {
	Width  int    // px, 0 = default
	Height int    // px, 0 = default
	Resize string // "fit" (default), "fill", "auto"
	Format string // "webp", "jpg", "png" — appended as `.ext` on the source-URL segment
}

// Sign returns a fully-formed delivery URL. sourceURL is the raw storage URL
// (the `public_url` from PresignResponse, or any URL imgproxy can fetch).
//
// The encoded source URL uses URL-safe base64 with no padding, per imgproxy's
// spec.
func (i *Imgproxy) Sign(sourceURL string, t Transform) string {
	options := buildProcessingOptions(t)
	encoded := base64.RawURLEncoding.EncodeToString([]byte(sourceURL))
	if t.Format != "" {
		encoded += "." + t.Format
	}

	// path is everything that goes into the HMAC: leading "/", options, "/",
	// encoded source URL.
	var pathBuilder strings.Builder
	pathBuilder.WriteByte('/')
	pathBuilder.WriteString(options)
	pathBuilder.WriteByte('/')
	pathBuilder.WriteString(encoded)
	path := pathBuilder.String()

	var sig string
	if len(i.key) == 0 || len(i.salt) == 0 {
		sig = "insecure"
	} else {
		mac := hmac.New(sha256.New, i.key)
		mac.Write(i.salt)
		mac.Write([]byte(path))
		sig = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}

	base := strings.TrimRight(i.cfg.BaseURL, "/")
	return base + "/" + sig + path
}

// buildProcessingOptions assembles the imgproxy "options" segment. We use
// the short-form syntax: `rs:fit:W:H/q:80/...`. If nothing is set, returns
// the no-op `plain` marker so the URL is still well-formed.
func buildProcessingOptions(t Transform) string {
	parts := make([]string, 0, 3)
	if t.Width > 0 || t.Height > 0 {
		resize := t.Resize
		if resize == "" {
			resize = "fit"
		}
		w := ""
		h := ""
		if t.Width > 0 {
			w = strconv.Itoa(t.Width)
		}
		if t.Height > 0 {
			h = strconv.Itoa(t.Height)
		}
		parts = append(parts, fmt.Sprintf("rs:%s:%s:%s", resize, w, h))
	}
	if len(parts) == 0 {
		// `plain` is imgproxy's "no transform" option group; keeps the URL
		// shape consistent.
		return "plain"
	}
	return strings.Join(parts, "/")
}
