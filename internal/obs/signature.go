package obs

import (
	"regexp"
	"strings"
)

// reHex and reDigits deliberately drop the \b word-boundary anchor: an
// underscore or hyphen id prefix (sp_9f8e7d6c5b4a) is itself a \w character
// adjacent to the hex run, so \b never fires there. The character classes
// already stop at the first non-matching rune, which is boundary enough.
var (
	reQuoted = regexp.MustCompile(`"[^"]*"|'[^']*'`)
	reUUID   = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	rePath   = regexp.MustCompile(`\b[\w.\-]+(?:/[\w.\-]+)+\b`)
	reHex    = regexp.MustCompile(`[0-9a-fA-F]{6,}`)
	reDigits = regexp.MustCompile(`\d{4,}`)
	reSpaces = regexp.MustCompile(`\s+`)
)

// normalizeSignature strips run-specific detail — quoted strings, UUIDs,
// paths, hex ids, long digit runs — from error text so the same underlying
// failure produces the same signature across runs. It is the cluster's
// cross-run identity: case is preserved, only volatile substrings are cut.
func normalizeSignature(s string) string {
	s = reQuoted.ReplaceAllString(s, "<str>")
	s = reUUID.ReplaceAllString(s, "<uuid>")
	s = rePath.ReplaceAllString(s, "<path>")
	s = reHex.ReplaceAllString(s, "<hex>")
	s = reDigits.ReplaceAllString(s, "<num>")
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// clusterKeyPrefixLen bounds how much of a normalized signature two errors
// must share to be treated as one cluster. A shared prefix this long is a
// deliberate merge, not a coincidence: the reported scout-block hook family
// ("PreToolUse:Bash hook error: ... NOTE: This block is intentional ...
// Pattern: (^|\/)<name>...") shares an identical 151-char preamble up to and
// including "Pattern: (^|\/)" and only diverges in the specific blocked
// directory name after it (node_modules vs vendor vs .git ...) — that
// divergence is the same category of volatile mid-message detail
// normalizeSignature already strips for paths/ids, just past where a fixed
// set of regexes can chase it. 140 sits below the measured 151-char
// boundary with margin, so the family merges without the window reaching
// into the differing pattern names themselves (which would silently
// over-merge two genuinely distinct blocked directories, e.g. vendor vs
// venv share a "ven" prefix). A short, distinct error is well under this
// length and unaffected.
const clusterKeyPrefixLen = 140

// clusterKey is the grouping identity derived from a normalized signature:
// its first clusterKeyPrefixLen runes, or the whole signature when shorter.
// It is also the cluster's reported Signature — the truncated form is what
// every member actually shares, so displaying anything longer would show
// content some members in the cluster don't have.
func clusterKey(normalized string) string {
	r := []rune(normalized)
	if len(r) <= clusterKeyPrefixLen {
		return normalized
	}
	return string(r[:clusterKeyPrefixLen])
}
