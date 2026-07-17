package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
	"github.com/vanducng/vd-cli/v2/internal/obs/store"
)

// DefaultSince bounds the first sync. The local corpus is ~5GB across ~5400
// transcripts; a cold scan of all of it is a minute-plus of work nobody asked for.
const DefaultSince = 30 * 24 * time.Hour

// SyncOptions selects what to (re)ingest.
type SyncOptions struct {
	Full   bool
	Agents []string
	Since  time.Time
}

// SyncStats is what the CLI prints so drift and skips are visible rather than silent.
type SyncStats struct {
	FilesScanned   int
	FilesParsed    int
	Sessions       int
	Turns          int
	UnknownRecords int
	Skipped        int
	Errored        int
	Elapsed        time.Duration
}

func (o SyncOptions) wants(agent string) bool {
	if len(o.Agents) == 0 {
		return true
	}
	for _, a := range o.Agents {
		if a == agent {
			return true
		}
	}
	return false
}

// Sync brings the cache up to date with the transcripts on disk.
//
// Single-threaded on purpose: read paths call this from vd web's handlers, and
// net/http recovers panics per connection but never inside goroutines a handler
// spawns — one malformed transcript in a worker pool would take down the portal.
// SQLite has one writer regardless.
func Sync(ctx context.Context, st *store.Store, opts SyncOptions) (SyncStats, error) {
	start := time.Now()
	var stats SyncStats

	if opts.Full {
		if err := st.Reset(ctx); err != nil {
			return stats, err
		}
	}
	if opts.Since.IsZero() && !opts.Full {
		opts.Since = time.Now().Add(-DefaultSince)
	}

	var files []fileJob
	if opts.wants(model.AgentClaude) {
		claude, err := claudeJobs()
		if err != nil {
			return stats, err
		}
		files = append(files, claude...)
	}
	if opts.wants(model.AgentCodex) {
		codex, err := codexJobs()
		if err != nil {
			return stats, err
		}
		files = append(files, codex...)
	}

	for _, job := range files {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		stats.FilesScanned++

		info, err := os.Stat(job.path)
		if err != nil {
			continue // vanished between enumeration and stat
		}
		if !opts.Since.IsZero() && info.ModTime().Before(opts.Since) {
			stats.Skipped++
			continue
		}

		mtime, size := info.ModTime().UnixMilli(), info.Size()
		if !opts.Full {
			w, seen, err := st.GetWatermark(ctx, job.path)
			if err != nil {
				return stats, err
			}
			// The watermark's only job is skipping unchanged files. Changed files are
			// reparsed whole: see phase-05 / decisions.md — a mid-file resume re-emits a
			// turn holding only post-offset tokens and the store's upsert replaces
			// rather than merges, so totals would shrink.
			if seen && w.MTime == mtime && w.Size == size {
				stats.Skipped++
				continue
			}
		}

		rec, off, err := job.parse()
		if err != nil {
			// One unreadable file (perm denied, deleted mid-scan) must not abort the
			// whole run — the rest of the corpus still ingests. Count it, keep going.
			stats.Errored++
			continue
		}
		stats.UnknownRecords += job.st.unknownTotal()
		// A record with no session id would upsert a phantom row that ListSessions
		// then returns, and implicit turn ids would collapse onto each other. Its
		// drift is already folded into UnknownRecords above.
		if rec.Session.ID == "" {
			continue
		}
		stats.FilesParsed++
		stats.Sessions++
		stats.Turns += len(rec.Turns)

		w := store.Watermark{Path: job.path, ByteOffset: off, MTime: mtime, Size: size}
		if err := st.IngestFile(ctx, rec, w); err != nil {
			return stats, err
		}
	}

	stats.Elapsed = time.Since(start)
	return stats, nil
}

type fileJob struct {
	path  string
	st    *scanStateRef
	parse func() (model.Record, int64, error)
}

type scanStateRef struct{ s *ScanState }

func (r *scanStateRef) unknownTotal() int {
	n := 0
	for _, v := range r.s.UnknownTypes {
		n += v
	}
	return n
}

func claudeJobs() ([]fileJob, error) {
	roots, err := claudeRoots()
	if err != nil {
		return nil, err
	}
	var out []fileJob
	for _, root := range roots {
		top, subs, err := EnumerateClaude(root)
		if err != nil {
			return nil, fmt.Errorf("enumerate claude %s: %w", root, err)
		}
		for _, p := range append(top, subs...) {
			out = append(out, claudeJob(p))
		}
	}
	return out, nil
}

// claudeRoots returns every Claude home that exists: ~/.claude plus the XDG
// location — Claude Code writes to both across versions, and ccusage reads both.
func claudeRoots() ([]string, error) {
	primary, err := ClaudeRoot()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	var roots []string
	seen := map[string]bool{}
	for _, r := range []string{primary, filepath.Join(home, ".config", "claude")} {
		if _, err := os.Stat(filepath.Join(r, "projects")); err != nil {
			continue
		}
		// Resolve symlinks so ~/.config/claude pointing at ~/.claude is not
		// enumerated twice (distinct path strings never share a watermark).
		canon := r
		if c, err := filepath.EvalSymlinks(r); err == nil {
			canon = c
		}
		if seen[canon] {
			continue
		}
		seen[canon] = true
		roots = append(roots, r)
	}
	return roots, nil
}

func claudeJob(path string) fileJob {
	ref := &scanStateRef{s: &ScanState{}}
	return fileJob{
		path: path,
		st:   ref,
		parse: func() (model.Record, int64, error) {
			return ParseClaudeFile(path, ref.s)
		},
	}
}

func codexJobs() ([]fileJob, error) {
	root, err := CodexSessionsPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(root); err != nil {
		// missing is fine (no codex on this machine); a real error (perms) surfaces
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat codex root %s: %w", root, err)
	}
	paths, err := EnumerateCodex(root)
	if err != nil {
		return nil, fmt.Errorf("enumerate codex: %w", err)
	}
	out := make([]fileJob, 0, len(paths))
	for _, p := range paths {
		out = append(out, codexJob(p))
	}
	return out, nil
}

func codexJob(path string) fileJob {
	ref := &scanStateRef{s: &ScanState{}}
	return fileJob{
		path: path,
		st:   ref,
		parse: func() (model.Record, int64, error) {
			f, err := os.Open(path)
			if err != nil {
				return model.Record{}, 0, fmt.Errorf("open rollout %s: %w", path, err)
			}
			defer func() { _ = f.Close() }()
			return ParseCodex(f, ref.s)
		},
	}
}
