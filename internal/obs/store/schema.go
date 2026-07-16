package store

import (
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schemaSQL string

// schemaVersion is stamped into PRAGMA user_version. Bump it on any schema change:
// obs.sqlite is a derived cache, so a mismatch drops and rebuilds rather than migrates.
// schemaVersion doubles as the parser-semantics version: obs.sqlite is derived,
// so a change in how transcripts are billed invalidates it exactly like a
// column change does. v2: turns.id namespaced by session. v3: streaming usage
// billed at final totals per message. v4: codex duplicate detection compares
// last AND total, so distinct-but-identical-looking requests bill.
const schemaVersion = 4

func ensureSchema(db *sql.DB) error {
	var have int
	if err := db.QueryRow("PRAGMA user_version").Scan(&have); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if have != 0 && have != schemaVersion {
		if err := dropAll(db); err != nil {
			return fmt.Errorf("rebuild schema from v%d: %w", have, err)
		}
		have = 0
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	if have != schemaVersion {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			return fmt.Errorf("stamp user_version: %w", err)
		}
	}
	return nil
}

func dropAll(db *sql.DB) error {
	tables := []string{
		"ingest_state", "skill_invocations", "hook_execs",
		"tool_span_payloads", "tool_spans", "turn_payloads", "turns", "sessions",
	}
	for _, t := range tables {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + t); err != nil {
			return fmt.Errorf("drop %s: %w", t, err)
		}
	}
	return nil
}
