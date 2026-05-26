// Package auth implements hotel-staff authentication: signup (creating an
// account + first owner user in one transaction), login, refresh-token
// rotation, and a `RequireAuth` middleware that injects an [Identity] into
// the request context.
//
// Passwords are hashed with argon2id (params live in password.go: m=64MiB,
// t=3, p=4) — never store plaintext
// and never log the password field. Access tokens are short-lived HS256 JWTs
// (~15m); refresh tokens are opaque random strings, SHA-256-hashed before
// storage in the sessions table. Rotation on every refresh allows detection
// of stolen-and-reused tokens — the second use of an already-rotated token
// is treated as theft and revokes every session for that user.
//
// Cross-module integration: pass a callback to [Service.SetAccountInit] to
// run side effects after a successful signup (e.g. creating the
// subscription row). Errors from the callback are logged but do not fail
// the signup — they're best-effort and a worker safety-net is expected.
package auth
