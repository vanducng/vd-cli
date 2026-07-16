package store

// One source of truth for limits and clamps. The CLI flag default, the HTTP
// param default and the service clamp all read these, so they cannot drift.
const (
	DefaultSessionLimit = 50
	MaxSessionLimit     = 500
	DefaultUsageGroup   = UsageGroupDaily

	// MaxPayloadBytes caps stored prompt/response/tool payloads. The local corpus
	// is ~5GB of transcripts; obs.sqlite is a derived cache, not an archive.
	MaxPayloadBytes = 32 << 10

	// MinPrefixLen guards session-id prefix lookups. Codex ids are UUIDv7 and all
	// share a `019` prefix, so anything shorter matches thousands of sessions.
	MinPrefixLen = 8
)

// Wire values for the group/sort params, shared by the store, the service and
// both frontends so no site hardcodes a literal.
const (
	UsageGroupDaily   = "daily"
	UsageGroupMonthly = "monthly"
	SortNewest        = "newest"
	SortOldest        = "oldest"
)

// ClampLimit resolves a caller-supplied page size to the value actually applied,
// so an envelope can report the real limit rather than what was asked for.
func ClampLimit(n int) int {
	if n <= 0 {
		return DefaultSessionLimit
	}
	if n > MaxSessionLimit {
		return MaxSessionLimit
	}
	return n
}
