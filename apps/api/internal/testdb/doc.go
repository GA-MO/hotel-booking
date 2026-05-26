// Package testdb provides helpers for repository-level integration tests
// that exercise real Postgres rather than mocking the pgx layer.
//
// Tests using [Open] skip with t.Skip() when TEST_DATABASE_URL is unset,
// so `go test ./...` stays fast and dep-free in regular CI. Set the env
// var (and apply migrations) before running them:
//
//   docker compose up -d postgres
//   make -C apps/api migrate-up
//   TEST_DATABASE_URL='postgres://hotel:hotel@localhost:5432/hotel_booking?sslmode=disable' \
//     make -C apps/api test-integration
//
// The Makefile target adds `-p 1` so Postgres-level TRUNCATE in parallel
// tests across packages can't deadlock each other. Within a package,
// tests run serially (no t.Parallel() in integration tests).
//
// [SeedBase] inserts the minimum graph used by most tests: one account,
// one user, one live hotel, one room type with configurable inventory.
package testdb
