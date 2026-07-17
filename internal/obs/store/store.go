// Package store persists parsed agent transcripts in a local SQLite cache.
//
// The cache is derived: every row is rebuildable from the JSONL under ~/.claude
// and ~/.codex, which is why schema changes drop and rebuild instead of migrating.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver; the binary must build with CGO_ENABLED=0
)

// Config locates the cache file.
type Config struct {
	Path string
}

// Store owns the database handle. Unlike inventory.Service it holds a real
// resource, so callers must Close it.
type Store struct {
	db *sql.DB
}

// DefaultPath is ~/.vd/obs/obs.sqlite unless VD_OBS_DB overrides it.
func DefaultPath() (string, error) {
	if p := os.Getenv("VD_OBS_DB"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".vd", "obs", "obs.sqlite"), nil
}

// New opens the cache, applying connection pragmas via the DSN and enabling WAL
// once, then ensures the schema.
func New(cfg Config) (*Store, error) {
	if cfg.Path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		cfg.Path = p
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, fmt.Errorf("create obs dir: %w", err)
	}

	db, err := openAt(cfg.Path)
	if err == nil {
		return &Store{db: db}, nil
	}
	// The cache is rebuildable, so an actually-corrupt file self-heals: delete and
	// recreate once. Only for real corruption — a busy/locked error means another
	// live vd process holds the file, and deleting it out from under that process
	// silently forks the two onto different inodes until restart.
	if cfg.Path == ":memory:" || !isCorruption(err) {
		return nil, err
	}
	for _, p := range []string{cfg.Path, cfg.Path + "-wal", cfg.Path + "-shm"} {
		_ = os.Remove(p)
	}
	db, err2 := openAt(cfg.Path)
	if err2 != nil {
		return nil, fmt.Errorf("obs cache unusable (removed and rebuild also failed): %w", err2)
	}
	return &Store{db: db}, nil
}

// isCorruption matches SQLITE_CORRUPT / SQLITE_NOTADB, which modernc surfaces in
// the error text; busy/locked and permission errors are not corruption.
func isCorruption(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not a database") ||
		strings.Contains(msg, "database disk image is malformed") ||
		strings.Contains(msg, "sqlite_corrupt") || strings.Contains(msg, "sqlite_notadb")
}

func openAt(path string) (*sql.DB, error) {
	// modernc applies _pragma= on every pooled connection open, and orders
	// busy_timeout first itself. Setting these by hand per-connection is what
	// goclaw's connector does, and it swallows pragma errors doing it.
	dsn := "file:" + path +
		"?_txlock=immediate" +
		"&_pragma=busy_timeout(15000)" +
		"&_pragma=synchronous(NORMAL)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open obs db: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4) // keep all four warm; churn hurts under vd web's read bursts

	if err := enableWAL(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// enableWAL sets journal_mode once per database rather than per connection:
// journal_mode is persistent in the file header, and SQLite does not run the busy
// handler for journal-mode changes, so busy_timeout cannot help here. Two vd
// processes first-opening the same fresh file (vd web + vd obs sync) collide, so
// retry briefly and accept the mode another process already set.
func enableWAL(db *sql.DB) error {
	var lastErr error
	for i := 0; i < 10; i++ {
		var mode string
		if err := db.QueryRow("PRAGMA journal_mode=WAL").Scan(&mode); err == nil {
			if strings.EqualFold(mode, "wal") {
				return nil
			}
			lastErr = fmt.Errorf("journal_mode is %q, want wal", mode)
		} else {
			lastErr = err
		}
		var mode2 string
		if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode2); err == nil && strings.EqualFold(mode2, "wal") {
			return nil
		}
		time.Sleep(time.Duration(20*(i+1)) * time.Millisecond)
	}
	return fmt.Errorf("enable wal: %w", lastErr)
}

// Close releases the database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// truncateMid keeps the head and tail of an oversized payload and marks the cut.
// Head-only truncation drops the conclusion, which is the part worth reading.
func truncateMid(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	marker := "\n… [truncated] …\n"
	keep := max - len(marker)
	if keep <= 0 {
		return s[:max]
	}
	head := keep * 2 / 3
	tail := keep - head
	for head > 0 && !utf8RuneStart(s[head]) {
		head--
	}
	start := len(s) - tail
	for start < len(s) && !utf8RuneStart(s[start]) {
		start++
	}
	out := s[:head] + marker + s[start:]
	// Rune backoff only shrinks head/grows start, so out is always <= max here;
	// guard anyway so the cap is a hard invariant, not an inference.
	if len(out) > max {
		return out[:max]
	}
	return out
}

func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }
