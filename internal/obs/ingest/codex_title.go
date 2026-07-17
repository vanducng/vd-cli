package ingest

import (
	"regexp"
	"strings"
)

// maxDerivedTitleLen is the word-boundary truncation cap for a derived title.
const maxDerivedTitleLen = 80

// codexTitleSkillRe strips a leading skill invocation token ($name / $vd:name)
// from a derived title's first line. It mirrors codexSkillRe's token shape
// (skillnames.go) but does not require registry membership: a derived title
// should read cleanly even when the invoked skill is not installed locally.
var codexTitleSkillRe = regexp.MustCompile(`^\$[a-z][a-z0-9:_-]{1,40}\s*`)

// deriveCodexTitle builds a fallback Session.Title from a Codex rollout's
// first user message, for rollouts that carry no title field of their own.
// Empty or whitespace-only input yields "" — never a fabricated title.
func deriveCodexTitle(firstUserMsg string) string {
	line := firstNonEmptyLine(firstUserMsg)
	if line == "" {
		return ""
	}
	line = codexTitleSkillRe.ReplaceAllString(line, "")
	line = collapseWhitespace(line)
	if line == "" {
		return ""
	}
	line = redactSecrets(line)
	return truncateWords(line, maxDerivedTitleLen)
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncateWords truncates s to at most max runes, breaking on the last word
// boundary at or before the cut and appending "…". s itself is returned
// unchanged when already within the limit.
func truncateWords(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	cut := max
	for cut > 0 && r[cut-1] != ' ' {
		cut--
	}
	if cut == 0 {
		cut = max
	}
	return strings.TrimRight(string(r[:cut]), " ") + "…"
}

// secretPatterns are secret-shaped token families to strip from a derived
// title before it is ever persisted or rendered. Ordered specific-before-generic:
// once a specific match is replaced with "…" it no longer feeds the generic
// base64/hex catch-all that runs last.
//
// This is the derived-title path ONLY — transcript/prompt storage has its own
// existing handling and is untouched here.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-[A-Za-z0-9]{16,}`),
	regexp.MustCompile(`gh[po]_[A-Za-z0-9]{10,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{10,}`),
	regexp.MustCompile(`AKIA[A-Z0-9]{16}`),
	regexp.MustCompile(`xox[bap]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`(?i)Bearer\s+\S{16,}`),
	regexp.MustCompile(`(?i)(?:password|token|apikey|secret)=\S+`),
	regexp.MustCompile(`[A-Za-z0-9+/=_-]{32,}`),
}

// redactSecrets replaces every secret-shaped token in s with "…".
func redactSecrets(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllString(s, "…")
	}
	return s
}
