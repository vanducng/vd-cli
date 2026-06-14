package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanducng/vd-cli/v2/internal/inventory"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureServer(t *testing.T) http.Handler {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "skills", "alpha", "SKILL.md"),
		"---\nname: alpha\ndescription: alpha skill\n---\nhello\n")
	write(t, filepath.Join(root, "skills.toml"),
		"[meta]\nversion = 1\n\n[skills.alpha]\nsource = \"up\"\nmode = \"tracked\"\n")

	claude := t.TempDir()
	write(t, filepath.Join(claude, "settings.json"),
		`{"hooks":{"SessionStart":[{"matcher":"startup","hooks":[{"type":"command","command":"echo hi"}]}]}}`)

	srv, err := NewServer(inventory.NewService(root, claude))
	if err != nil {
		t.Fatal(err)
	}
	return srv.Handler()
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestInventoryEndpoint(t *testing.T) {
	rec := get(t, fixtureServer(t), "/api/inventory")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var inv inventory.Inventory
	if err := json.Unmarshal(rec.Body.Bytes(), &inv); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(inv.Managed) != 1 || inv.Managed[0].Name != "alpha" {
		t.Errorf("managed = %+v", inv.Managed)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("content-type = %q", rec.Header().Get("Content-Type"))
	}
}

func TestSkillEndpoint(t *testing.T) {
	h := fixtureServer(t)
	rec := get(t, h, "/api/skills/alpha")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var d inventory.SkillDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(d.Body, "hello") {
		t.Errorf("body = %q", d.Body)
	}

	if got := get(t, h, "/api/skills/missing").Code; got != http.StatusNotFound {
		t.Errorf("missing skill status = %d, want 404", got)
	}
}

func TestHooksAndDoctorEndpoints(t *testing.T) {
	h := fixtureServer(t)
	if rec := get(t, h, "/api/hooks"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SessionStart") {
		t.Errorf("hooks: %d %s", rec.Code, rec.Body.String())
	}
	if rec := get(t, h, "/api/doctor"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "entries") {
		t.Errorf("doctor: %d %s", rec.Code, rec.Body.String())
	}
}

func TestHealthEndpoint(t *testing.T) {
	rec := get(t, fixtureServer(t), "/api/health")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "\"ok\":true") {
		t.Errorf("health: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSPAFallback(t *testing.T) {
	h := fixtureServer(t)
	// Unknown client route → embedded index.html.
	rec := get(t, h, "/some/client/route")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="root"`) {
		t.Errorf("spa fallback: %d %s", rec.Code, rec.Body.String())
	}
	// Root path serves index too.
	if rec := get(t, h, "/"); !strings.Contains(rec.Body.String(), `id="root"`) {
		t.Errorf("root index missing: %s", rec.Body.String())
	}
}
