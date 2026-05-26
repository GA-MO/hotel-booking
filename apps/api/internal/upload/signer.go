package upload

// AWS Signature Version 4 — query-string presigning for S3-compatible
// storage (MinIO + Backblaze B2). Hand-rolled so we don't pull
// aws-sdk-go-v2 (~30 MB, ~50 transitive deps) just to sign a URL.
//
// Spec: https://docs.aws.amazon.com/IAM/latest/UserGuide/create-signed-request.html
//
// Test vectors live in signer_test.go and reproduce the canonical examples
// from the AWS "Signing AWS requests with Signature Version 4" guide.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// SignerConfig carries everything needed to produce a presigned PUT URL.
type SignerConfig struct {
	// Endpoint is the storage endpoint (e.g. "http://minio:9000" in dev,
	// "https://s3.eu-central-003.backblazeb2.com" in prod). No trailing slash.
	Endpoint string
	// Region is the SigV4 region scope. MinIO accepts anything; B2 wants the
	// region embedded in its endpoint (e.g. "eu-central-003").
	Region string
	// Bucket is the destination bucket. We use path-style addressing
	// (`endpoint/bucket/key`) so MinIO works without DNS gymnastics.
	Bucket string
	// AccessKey + SecretKey are the IAM credentials.
	AccessKey string
	SecretKey string
}

// Signer is safe for concurrent use. It holds no per-request state.
type Signer struct {
	cfg SignerConfig
}

// NewSigner returns a configured Signer. Returns nil if cfg.AccessKey or
// cfg.SecretKey is empty; callers (e.g. the upload handler) should treat
// that as "uploads disabled" and surface 503-style errors.
func NewSigner(cfg SignerConfig) *Signer {
	if cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Endpoint == "" {
		return nil
	}
	return &Signer{cfg: cfg}
}

// PresignPut returns a query-presigned URL for an HTTP PUT against
// `{endpoint}/{bucket}/{objectKey}`. The browser MUST send the returned
// headers verbatim or the signature will fail.
//
// `expires` is clamped to AWS's max of 7 days. `contentType` is included as
// a signed header — without it, S3 stores the file as
// application/octet-stream and imgproxy refuses to process it.
//
// `now` is injected for testability (production callers pass time.Now()).
func (s *Signer) PresignPut(
	objectKey, contentType string,
	expires time.Duration,
	now time.Time,
) (uploadURL string, signedHeaders map[string]string, err error) {
	if s == nil {
		return "", nil, ErrNotConfigured
	}
	if contentType == "" {
		return "", nil, ErrInvalidContentType
	}
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	if max := 7 * 24 * time.Hour; expires > max {
		expires = max
	}

	// Build the canonical URI for path-style S3: "/" + bucket + "/" + key.
	canonicalURI := "/" + s.cfg.Bucket + "/" + uriEncodePath(objectKey)

	// Sign host + content-type so the browser can't swap content-type.
	headers := map[string]string{
		"content-type": contentType,
	}
	uploadURL, err = s.presignWithMethod("PUT", canonicalURI, headers, expires, now)
	if err != nil {
		return "", nil, err
	}
	return uploadURL, map[string]string{
		"Content-Type": contentType,
	}, nil
}

// presignWithMethod is the SigV4 core; PresignPut delegates here and so do
// tests that need to reproduce AWS-published vectors (which use GET).
//
// `canonicalURI` MUST already be uriEncode'd (path style). `extraHeaders`
// are extra headers to sign on top of `host` — names lowercased.
func (s *Signer) presignWithMethod(
	method, canonicalURI string,
	extraHeaders map[string]string,
	expires time.Duration,
	now time.Time,
) (string, error) {
	now = now.UTC()
	amzDate := now.Format("20060102T150405Z") // ISO 8601 basic
	dateStamp := now.Format("20060102")       // YYYYMMDD
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, s.cfg.Region, "s3")

	endpoint, err := url.Parse(s.cfg.Endpoint)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	host := endpoint.Host

	signed := make(map[string]string, len(extraHeaders)+1)
	signed["host"] = host
	for k, v := range extraHeaders {
		signed[strings.ToLower(k)] = v
	}
	signedNames := sortedKeys(signed)
	signedHeadersStr := strings.Join(signedNames, ";")

	// Canonical query string (alphabetical by encoded key). X-Amz-Signature
	// is NOT included — we append it after computing.
	query := url.Values{}
	query.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	query.Set("X-Amz-Credential", s.cfg.AccessKey+"/"+credentialScope)
	query.Set("X-Amz-Date", amzDate)
	query.Set("X-Amz-Expires", fmt.Sprintf("%d", int64(expires.Seconds())))
	query.Set("X-Amz-SignedHeaders", signedHeadersStr)
	canonicalQuery := canonicalQueryString(query)

	// Canonical headers block: "name:trimmed-value\n", sorted by name.
	var canonicalHeaders strings.Builder
	for _, name := range signedNames {
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.TrimSpace(signed[name]))
		canonicalHeaders.WriteByte('\n')
	}

	// UNSIGNED-PAYLOAD — the client computes no body hash for a PUT, and S3
	// accepts the literal string.
	payloadHash := "UNSIGNED-PAYLOAD"

	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders.String(),
		signedHeadersStr,
		payloadHash,
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hexSHA256([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(s.cfg.SecretKey, dateStamp, s.cfg.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	return fmt.Sprintf(
		"%s%s?%s&X-Amz-Signature=%s",
		strings.TrimRight(s.cfg.Endpoint, "/"),
		canonicalURI,
		canonicalQuery,
		signature,
	), nil
}

// ----- AWS-style canonicalization helpers -----

// uriEncodePath is RFC 3986 percent-encoding except "/" is preserved.
// (AWS calls this "uriEncode(path, false)".)
func uriEncodePath(p string) string {
	var b strings.Builder
	b.Grow(len(p))
	for i := 0; i < len(p); i++ {
		c := p[i]
		if isUnreserved(c) || c == '/' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// uriEncodeQuery is RFC 3986 percent-encoding with NO exceptions
// (everything non-unreserved is encoded, including "/").
func uriEncodeQuery(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// isUnreserved per RFC 3986 §2.3.
func isUnreserved(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z':
		return true
	case c >= 'a' && c <= 'z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '-' || c == '_' || c == '.' || c == '~':
		return true
	}
	return false
}

// canonicalQueryString sorts by encoded key, then by encoded value.
func canonicalQueryString(v url.Values) string {
	type kv struct{ k, v string }
	pairs := make([]kv, 0, len(v))
	for k, vals := range v {
		ek := uriEncodeQuery(k)
		for _, vv := range vals {
			pairs = append(pairs, kv{k: ek, v: uriEncodeQuery(vv)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].k != pairs[j].k {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v < pairs[j].v
	})
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = p.k + "=" + p.v
	}
	return strings.Join(parts, "&")
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func hexSHA256(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// deriveSigningKey implements the SigV4 key-derivation chain:
//
//	kDate    = HMAC("AWS4" + secret, dateStamp)
//	kRegion  = HMAC(kDate,    region)
//	kService = HMAC(kRegion,  service)
//	kSigning = HMAC(kService, "aws4_request")
func deriveSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}
