package storage

import (
	"errors"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/AhmedEnnaime/AgentLens/internal/domain"
)

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() {
		case sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY, sqlite3.SQLITE_CONSTRAINT_UNIQUE:
			return domain.ErrConflict
		case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
			return domain.ErrNotFound
		}
	}
	msg := err.Error()
	if strings.Contains(msg, "SQLITE_CONSTRAINT") {
		return domain.ErrConflict
	}
	return err
}
