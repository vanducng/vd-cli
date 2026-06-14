package inventory

import "testing"

func TestParseFrontmatter_Present(t *testing.T) {
	src := []byte("---\nname: foo\ndescription: a tool\n---\n# Body\ntext\n")
	fm, body, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm["name"] != "foo" {
		t.Errorf("name = %v, want foo", fm["name"])
	}
	if Describe(fm) != "a tool" {
		t.Errorf("description = %q, want %q", Describe(fm), "a tool")
	}
	if body != "# Body\ntext\n" {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontmatter_None(t *testing.T) {
	src := []byte("# Just markdown\nno frontmatter\n")
	fm, body, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}
	if body != string(src) {
		t.Errorf("body should equal full content")
	}
}

func TestParseFrontmatter_Malformed(t *testing.T) {
	src := []byte("---\nkey: [unterminated\n---\nbody\n")
	if _, _, err := ParseFrontmatter(src); err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestParseFrontmatter_EmptyBody(t *testing.T) {
	src := []byte("---\nname: x\n---\n")
	fm, body, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm["name"] != "x" || body != "" {
		t.Errorf("fm=%v body=%q", fm, body)
	}
}

func TestDescribe_MissingOrNonString(t *testing.T) {
	if Describe(map[string]any{}) != "" {
		t.Error("missing description should be empty")
	}
	if Describe(map[string]any{"description": 42}) != "" {
		t.Error("non-string description should be empty")
	}
}

func TestEnabledFromName(t *testing.T) {
	if n, ok := enabledFromName("SKILL.md"); !ok || n != "SKILL.md" {
		t.Errorf("got %q,%v", n, ok)
	}
	if n, ok := enabledFromName("SKILL.md.disabled"); ok || n != "SKILL.md" {
		t.Errorf("got %q,%v", n, ok)
	}
}
