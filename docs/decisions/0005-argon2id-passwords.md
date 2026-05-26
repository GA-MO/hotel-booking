# ADR-0005 — argon2id for password hashing

- **Status:** Accepted
- **Date:** 2026-05-26
- **Deciders:** team
- **Related:** plan.md §10.2 (Security & Compliance), code: `apps/api/internal/auth/password.go`

## Context

The platform stores credentials for hotel staff users (Owner, Manager,
Front Desk, Read-only). Guests do not have accounts in Phase 1 — they
identify themselves per booking by `reference + email`. So the password
hashing concern is bounded to hotel staff, but it is non-negotiable: a leak
of those hashes would be a PDPA-reportable incident.

The modern field of password hashing is small. The serious choices are
bcrypt (1999, OpenBSD), scrypt (2009, Tarsnap), PBKDF2 (1996, NIST/RFC),
and argon2 (2015, winner of the Password Hashing Competition, with the
`id` variant recommended by OWASP).

The constraints:

- **Side-channel resistance.** We accept logins on the same VPS as the rest of the workload; a timing-leaky algorithm in a shared address space is a real concern.
- **GPU/ASIC resistance.** Adversaries who get the hash table will throw GPUs at it; bcrypt is now meaningfully GPU-crackable, PBKDF2 catastrophically so.
- **Tunable cost.** We must be able to bump parameters as hardware improves without a schema migration.
- **Test ergonomics.** Hashing is intentionally slow. If our test suite calls it on every fixture, the suite becomes unusable.

## Decision

Use **argon2id** with parameters `memory = 64 MiB`, `iterations = 3`,
`parallelism = 4`, `saltLength = 16 bytes`, `keyLength = 32 bytes`. Hashes
are stored in PHC string format (`$argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>`)
so the parameters travel with each row and can be migrated per-user.
Verification uses `crypto/subtle.ConstantTimeCompare`.

## Consequences

### Positive
- argon2id is the OWASP-recommended default and the PHC competition winner — defensible in any post-incident review.
- The `id` variant resists both GPU attacks (memory-hard) and side-channel attacks (data-independent for the first pass).
- PHC encoding means we can raise the cost factor later without a synchronised migration — old hashes verify with their original parameters and get rehashed on next successful login.

### Negative / trade-offs
- A single hash takes ~100 ms wall-clock on the production CPX21. Login is fine but **the test suite must call argon2 sparingly** — fixtures should reuse one pre-hashed password across test cases (see `internal/auth/integration_test.go`).
- 64 MiB × parallelism=4 means a login costs ~256 MiB peak working set per attempt. Acceptable on an 8 GB box at our scale but it sets a ceiling on parallel login attempts.
- The `golang.org/x/crypto/argon2` package is a dependency outside the Go standard library.

### Neutral
- Salt and hash are stored together in the PHC string; we do not keep a separate salt column.

## Alternatives considered

### Alt 1: bcrypt (`golang.org/x/crypto/bcrypt`)
Rejected. Mature and battle-tested but increasingly GPU-friendly, no memory hardness, and parameter encoding is less self-describing than PHC.

### Alt 2: scrypt
Rejected. Memory-hard but the parameter space (`N, r, p`) is less reviewed and library support is uneven. argon2 is its direct successor.

### Alt 3: PBKDF2-HMAC-SHA256
Rejected. CPU-bound only, GPUs eat it. OWASP only suggests it as a fallback when nothing better is available (e.g., FIPS-constrained environments).

## Notes

- Tuning: re-benchmark `m`, `t`, `p` annually. The PHC-encoded hash makes it safe to raise these without touching old rows.
- If we ever expose guest accounts, the same params apply.
