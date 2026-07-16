package web

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/vanducng/vd-cli/v2/internal/inventory"
	"github.com/vanducng/vd-cli/v2/internal/obs"
	"github.com/vanducng/vd-cli/v2/internal/version"
)

// Server wraps the inventory service as a read-only JSON API plus the SPA.
type Server struct {
	svc    *inventory.Service
	obs    *obsHandler
	static fs.FS
}

// NewServer builds the HTTP server around the inventory and obs services. A nil
// obs service is allowed: its routes then answer 503 rather than panicking.
func NewServer(svc *inventory.Service, obsSvc *obs.Service) (*Server, error) {
	static, err := staticFS()
	if err != nil {
		return nil, err
	}
	return &Server{svc: svc, obs: &obsHandler{svc: obsSvc}, static: static}, nil
}

// Handler returns the routed http.Handler (Go 1.22+ method+path patterns).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/inventory", s.handleInventory)
	mux.HandleFunc("GET /api/skills/{name}", s.handleSkill)
	mux.HandleFunc("GET /api/hooks", s.handleHooks)
	mux.HandleFunc("GET /api/doctor", s.handleDoctor)
	s.obs.registerRoutes(mux)
	// Reserved prefix: without this the SPA catch-all answers every unmatched
	// /api/* path with index.html and a 200, so a typo — or a whole unwired API —
	// looks like a working page.
	mux.Handle("GET /api/", http.NotFoundHandler())
	mux.Handle("/", s.spaHandler())
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": version.Version})
}

func (s *Server) handleInventory(w http.ResponseWriter, _ *http.Request) {
	inv, err := s.svc.Inventory()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, inv)
}

func (s *Server) handleSkill(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.SkillDetail(r.PathValue("name"))
	switch {
	case errors.Is(err, inventory.ErrNotFound):
		writeErr(w, http.StatusNotFound, err)
	case err != nil:
		writeErr(w, http.StatusInternalServerError, err)
	default:
		writeJSON(w, http.StatusOK, d)
	}
}

func (s *Server) handleHooks(w http.ResponseWriter, _ *http.Request) {
	hooks, err := s.svc.Hooks()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hooks": hooks})
}

func (s *Server) handleDoctor(w http.ResponseWriter, _ *http.Request) {
	rep, err := s.svc.Doctor()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// spaHandler serves embedded assets, falling back to index.html for client routes.
func (s *Server) spaHandler() http.Handler {
	fileServer := http.FileServer(http.FS(s.static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(s.static, p); err != nil {
			s.serveIndex(w) // unknown path → SPA entry for client-side routing
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) serveIndex(w http.ResponseWriter) {
	data, err := fs.ReadFile(s.static, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
