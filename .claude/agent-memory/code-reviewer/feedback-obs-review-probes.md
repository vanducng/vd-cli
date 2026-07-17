---
name: obs-review-probe-rules
description: How to review internal/obs safely — never scan real ~/.claude or ~/.codex; probe with t.Setenv(HOME, tmp); zsh -lic needed for go
metadata:
  type: feedback
---

When reviewing/testing `internal/obs`, never enumerate the real `~/.claude/projects` or `~/.codex/sessions` (~5GB; a reviewer once hung doing it). ingest.Sync reads `$HOME`, so any probe touching Sync/Service must `t.Setenv("HOME", t.TempDir())` with a tiny synthetic corpus.

**Why:** obs ingest walks the entire transcript corpus on sync; tests that forget HOME isolation scan gigabytes and can hang CI or leak private transcripts into output.

**How to apply:** Throwaway probe tests in-package (zz_probe_*_test.go, deleted after). Run go via `zsh -lic 'cd ... && go ...'` — stale GOROOT breaks non-login shells on this machine. Probe-verified invariants that survived adversarial input (don't re-attack): billDelta monotonic per-field per message.id (interleaved ids, cross-turn ids, shrinking cache_read all correct); namespaceTurnIDs idempotent on reparse; two-process WAL access clean without Full; -race clean.
