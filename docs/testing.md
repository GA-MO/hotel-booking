# Testing strategy

What to test, where, and how — for both humans and AI assistants.

---

## Layers

```
e2e        scripts/smoke-test.sh           — runs against a real running stack
 │
integration  internal/<mod>/integration_test.go      — real Postgres, no HTTP
                                              package=<mod>_test
 │
unit       internal/<mod>/{*,handler,service}_test.go — no DB, no network
                                              package=<mod>
 │
fast feedback ← → broad coverage
```

---

## Unit tests

**Where:** alongside source — `service_test.go`, `handler_test.go`, `engine_test.go`, etc. Use `package <module>` (not `<module>_test`) so internal helpers are accessible.

**What's testable as a unit:**

- Pure functions: pricing engine, password hash/verify, JWT round-trip, validation helpers.
- HTTP handler error mapping: pass a sentinel error to `writeError(w, err)` and assert status + JSON code.
- Middleware composition: `requireRole(...)` with a fake `auth.Identity` injected via `auth.WithIdentity(ctx, ...)`.
- Helpers: `parseUUIDParam`, `decodeJSON`, request context setters.

**What's NOT a unit (defer to integration):**

- Anything touching the pgx pool — even "should INSERT" assertions.
- Cross-table joins.
- Transactions, locks, race conditions.

**Conventions:**

- Table-driven where natural. Parallel sub-tests with `t.Parallel()` when there's no shared state.
- No testify, no mocking framework. Use stdlib `testing` and hand-written interfaces where needed.
- Argon2 is slow on purpose — keep hash calls to 1-2 per test file.

```bash
make -C apps/api test                       # all unit tests, integration tests skip
make -C apps/api test                       # also runs vet via Make
go test ./internal/booking/... -run TestRepo_CreatePending -race -count=1 -v
```

---

## Integration tests

**Where:** `internal/<mod>/integration_test.go`. Use `package <module>_test` (external — to enforce only-public-API testing of the repository / service surface).

**Setup:** `internal/testdb` package supplies `Open(t)`, `Truncate(t, pool)`, `SeedBase(t, pool, inventory)`. All tests using `Open` skip with `t.Skip()` if `TEST_DATABASE_URL` is unset.

```bash
# 1. ensure DB is up + migrations applied
docker compose up -d postgres
make -C apps/api migrate-up

# 2. run
make -C apps/api test-integration
```

**The `-p 1` gotcha:** the Makefile target adds `-p 1` because parallel packages running `TRUNCATE` against the same DB deadlock. Within a package, tests run serially (no `t.Parallel()` on integration tests).

**Cleanup:** each test calls `testdb.Truncate(t, pool)` first. `t.Cleanup` is not needed for table state — the next test will truncate.

**What to test at this layer:**

- Repository methods that issue real SQL (every CRUD + every transition).
- Tenant isolation: try cross-account access → expect 404-equivalent sentinel.
- Concurrency: spawn goroutines, race against `FOR UPDATE` / `SKIP LOCKED`. The flagship is `booking.TestRepo_CreatePending_RaceNoOversell` — 10 contenders, capacity=3, exactly 3 succeed, DB invariant holds afterwards.
- State-machine transitions end-to-end (e.g. pending_payment → confirmed → checked_in → checked_out).

**Migrations are NOT applied by tests.** Apply them once via `migrate-up`; tests assume the schema exists. Use `make migrate-down` or recreate the database if you need a clean slate.

---

## End-to-end smoke test

`scripts/smoke-test.sh` exercises the full happy path against a running API:
signup → hotel → room type → landing publish → pricing rule → walk-in booking → confirm → check-in/out → refresh token → overbook 409.

```bash
docker compose up -d
make -C apps/api migrate-up
make -C apps/api dev &
./scripts/smoke-test.sh
```

If a step fails, the script prints the offending HTTP response. Best to fix the underlying handler/SQL and re-run rather than weaken the assertion.

---

## Server route registration test

`internal/platform/server/server_test.go` constructs `New(...)` with nil pool/redis (safe — handlers store but don't dereference at construction) and walks the chi router asserting every expected route is registered. Catches `chi.Mount("/")` conflicts and missing wirings at test time instead of in production.

When you add a route, **add it to this test** — it's the closest thing we have to a contract test for the URL surface.

---

## What we don't test (yet)

- **Frontend** — booking-web + admin-web are scaffold-only Phase 0. Will need component / Playwright e2e in Phase 2.
- **Resend send path against the real API.** `LogSender` is the default in tests; the wire format of the POST is hand-rolled so a Resend API change won't be caught until a real send fails.
- **Cloudflare / Caddy / TLS** — verified manually per `infra/README.md` after each prod deploy.
- **Performance regressions / load tests** — not yet.

---

## CI

`.github/workflows/ci.yml`:

- `api` job: `go vet ./...`, `go build ./...`, `go test ./... -race -cover` (unit only — no `TEST_DATABASE_URL`).
- `web` job: `pnpm lint`, `pnpm typecheck`, `pnpm build`.

Integration tests are intentionally not in CI yet — they need a Postgres service container. To add: spin up `postgres:16-alpine` as a service, run `migrate up`, then `make test-integration`.

---

## When you add new tests

| If you wrote… | Also add… |
|---|---|
| A new repository method | An integration test exercising at least the success + one failure path |
| A new HTTP handler | A handler test for the error envelope mapping |
| A new service-layer validation | A table-driven unit test |
| A new route | Update `server_test.go`'s expected-routes list |
| A pricing rule type | Engine test covering applied + not-applied + edge of range |
| A new state in a state machine | `CanTransition` table case + an integration test transitioning into and out of it |
