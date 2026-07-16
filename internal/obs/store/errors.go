package store

import (
	"database/sql"
	"errors"
)

// Sentinel errors the HTTP layer maps to status codes.
var (
	ErrSessionNotFound = errors.New("obs: session not found")
	ErrAmbiguousPrefix = errors.New("obs: session id prefix matches multiple sessions")
	ErrPrefixTooShort  = errors.New("obs: session id prefix too short")
)

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }
