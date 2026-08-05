package apierror

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

// WritePostgresError inspects a Postgres error and writes an appropriate HTTP status and message.
// Returns true if it recognized and handled the error and false if the caller should fall back to a generic 500.
func WritePostgresError(w http.ResponseWriter, err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	switch pgErr.Code {
	case "23505": // unique_violation
		http.Error(w, "a record with this value already exists", http.StatusConflict)
	case "23503": // foreign_key_violation
		http.Error(w, "referenced record does not exist", http.StatusBadRequest)
	case "23001": // restrict_violation (ON DELETE/UPDATE RESTRICT)
		http.Error(w, "cannot delete: record is still referenced by other data", http.StatusConflict)
	case "23514": // check_violation
		http.Error(w, "invalid data: failed a validation rule", http.StatusBadRequest)
	case "23P01": // exclusion_violation
		http.Error(w, "conflicts with an existing record", http.StatusConflict)
	case "40P01": // deadlock_detected
		http.Error(w, "temporary conflict, please retry", http.StatusConflict)
	default:
		return false
	}
	return true
}
