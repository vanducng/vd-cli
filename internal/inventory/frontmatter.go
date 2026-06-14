package inventory

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

var fence = []byte("---")

// ParseFrontmatter splits a leading `---`-fenced YAML block from the markdown
// body. No frontmatter → empty map + full content. Malformed YAML → error.
func ParseFrontmatter(content []byte) (map[string]any, string, error) {
	fm := map[string]any{}
	trimmed := bytes.TrimLeft(content, " \t\r\n")
	if !bytes.HasPrefix(trimmed, fence) {
		return fm, string(content), nil
	}

	// Step past the opening fence line.
	rest := trimmed[len(fence):]
	nl := bytes.IndexByte(rest, '\n')
	if nl < 0 {
		return fm, string(content), nil
	}
	rest = rest[nl+1:]

	// Closing fence must begin a line.
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return fm, string(content), nil
	}
	block := rest[:end]

	// Body starts after the closing fence's line.
	after := rest[end+len("\n---"):]
	if i := bytes.IndexByte(after, '\n'); i >= 0 {
		after = after[i+1:]
	} else {
		after = nil
	}

	if err := yaml.Unmarshal(block, &fm); err != nil {
		return nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	if fm == nil {
		fm = map[string]any{}
	}
	return fm, string(after), nil
}

// Describe extracts the description field as a trimmed string, tolerant of absence.
func Describe(fm map[string]any) string {
	if v, ok := fm["description"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// enabledFromName strips a trailing `.disabled` marker and reports enablement.
func enabledFromName(name string) (string, bool) {
	if strings.HasSuffix(name, ".disabled") {
		return strings.TrimSuffix(name, ".disabled"), false
	}
	return name, true
}
