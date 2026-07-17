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
const schemaVersion = 5 // v5: seq on hook/skill PKs so repeats within a turn don't collapse

func ensureSchema(db *sql.DB) error {
	var have int
	if err := db.QueryRow("PRAGMA user_version").Scan(&have); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	// Already current: the tables exist (CREATE IF NOT EXISTS ran at this version),
	// so skip re-parsing all DDL on every open.
	if have == schemaVersion {
		return nil
	}
	return rebuildSchema(db)
}

// rebuildSchema drops any existing tables, recreates them, and stamps the version
// — all in one transaction, so a concurrent WAL reader sees either the old schema
// or the new one, never the tableless gap.
func rebuildSchema(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin schema tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := dropAll(tx); err != nil {
		return fmt.Errorf("drop for rebuild: %w", err)
	}
	if _, err := tx.Exec(schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("stamp user_version: %w", err)
	}
	return tx.Commit()
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func dropAll(db execer) error {
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
