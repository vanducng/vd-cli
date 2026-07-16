package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// obsTestServer wires a server with no obs service: the routes exist, so routing,
// param validation and the reserved-prefix rule are all exercised without a DB.
func obsTestServer(t *testing.T) http.Handler {
	t.Helper()
	srv, err := NewServer(nil, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv.Handler()
}

// Without a reserved /api/ prefix the SPA catch-all answers every unmatched
// /api/* path with index.html and a 200 — a typo, or an entire unwired API,
// looks like a working page.
func TestUnmatchedAPIPathIs404NotSPA(t *testing.T) {
	h := obsTestServer(t)
	for _, p := range []string{"/api/typo", "/api/obs/nope", "/api/obs/"} {
		w := get(t, h, p)
		if w.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 (body starts %.30q)", p, w.Code, w.Body.String())
		}
	}
}

// A client-side route must still reach the SPA.
func TestSPARouteStillServesIndex(t *testing.T) {
	w := get(t, obsTestServer(t), "/obs/sessions/abc")
	if w.Code != http.StatusOK {
		t.Errorf("deep link = %d, want 200 SPA", w.Code)
	}
}

// A nil obs service must not panic a handler.
func TestObsRoutesReport503WhenUnwired(t *testing.T) {
	h := obsTestServer(t)
	for _, p := range []string{"/api/obs/sessions", "/api/obs/usage", "/api/obs/sessions/abc12345"} {
		if w := get(t, h, p); w.Code != http.StatusServiceUnavailable {
			t.Errorf("GET %s = %d, want 503", p, w.Code)
		}
	}
}

// Bad params are a 400: silently ignoring one would return unfiltered data as if
// the filter had applied.
func TestObsBadParamsAre400(t *testing.T) {
	h := obsTestServer(t)
	bad := []string{
		"/api/obs/sessions?agent=bogus",
		"/api/obs/sessions?sort=sideways",
		"/api/obs/sessions?since=yesterday",
		"/api/obs/sessions?limit=-1",
		"/api/obs/sessions?limit=abc",
		"/api/obs/usage?group=hourly",
		"/api/obs/usage?agent=bogus",
	}
	for _, p := range bad {
		w := get(t, h, p)
		if w.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", p, w.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Errorf("GET %s: error body is not JSON: %v", p, err)
		} else if _, ok := body["error"]; !ok {
			t.Errorf("GET %s: error body lacks the {\"error\":...} shape: %v", p, body)
		}
	}
}

func TestObsValidParamsReachTheService(t *testing.T) {
	h := obsTestServer(t)
	// 503 (not 400) proves the params parsed and the handler got as far as the service.
	ok := []string{
		"/api/obs/sessions?agent=claude-code&since=7d&limit=10&offset=5&sort=oldest&q=x&project=vd-cli",
		"/api/obs/sessions?since=2026-07-01T00:00:00Z",
		"/api/obs/usage?group=monthly&agent=codex",
	}
	for _, p := range ok {
		if w := get(t, h, p); w.Code != http.StatusServiceUnavailable {
			t.Errorf("GET %s = %d, want 503 (params should have parsed)", p, w.Code)
		}
	}
}

// Non-GET verbs to /api must 404, not fall through to the SPA and return 200
// index.html — the deny-list is method-agnostic.
func TestNonGetAPIPathsAre404NotSPA(t *testing.T) {
	h := obsTestServer(t)
	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/obs/sessions"},
		{http.MethodDelete, "/api/obs/sessions/abc12345"},
		{http.MethodPost, "/api/typo"},
		{http.MethodPut, "/api/health"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(c.method, c.path, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404 (body %.20q)", c.method, c.path, w.Code, w.Body.String())
		}
	}
}
