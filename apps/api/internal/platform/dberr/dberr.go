// Package dberr holds the small set of error-classification helpers that
// every repository was previously copy-pasting (unique-violation detection,
// future siblings as we discover them).
package dberr

import "errors"

// pgErrIface matches the pgx-typed errors that expose SQLState. We use a
// local interface instead of importing pgx so this package stays dep-free.
type pgErrIface interface{ SQLState() string }

// IsUniqueViolation returns true if err originated from Postgres with
// SQLSTATE 23505 (unique_violation).
func IsUniqueViolation(err error) bool {
	const uniqueViolation = "23505"
	var pe pgErrIface
	if errors.As(err, &pe) {
		return pe.SQLState() == uniqueViolation
	}
	return false
}
