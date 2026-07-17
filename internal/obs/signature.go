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
