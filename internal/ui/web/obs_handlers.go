package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs"
	"github.com/vanducng/vd-cli/v2/internal/obs/ingest"
	"github.com/vanducng/vd-cli/v2/internal/obs/model"
	"github.com/vanducng/vd-cli/v2/internal/obs/store"
)

// obsHandler serializes obs.Service. It holds no logic of its own: cost, clamping
// and filtering all belong to the service, so the CLI and the web agree by
// construction rather than by discipline.
type obsHandler struct{ svc *obs.Service }

func (h *obsHandler) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/obs/sessions", h.sessions)
	mux.HandleFunc("GET /api/obs/sessions/{id}", h.session)
	mux.HandleFunc("GET /api/obs/usage", h.usage)
}

// ready reports whether obs is wired. A nil service reaching a handler would
// panic, so the routes answer 503 instead. Handlers validate params first: a
// malformed request is the client's bug whether or not the backend is up.
func (h *obsHandler) ready(w http.ResponseWriter) bool {
	if h == nil || h.svc == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("obs is not available in this build"))
		return false
	}
	return true
}

func (h *obsHandler) sync(r *http.Request) {
	_, _ = h.svc.Sync(r.Context(), ingest.SyncOptions{})
}

func (h *obsHandler) sessions(w http.ResponseWriter, r *http.Request) {
	f, err := parseSessionFilter(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !h.ready(w) {
		return
	}
	h.sync(r)
	list, err := h.svc.Sessions(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *obsHandler) session(w http.ResponseWriter, r *http.Request) {
	turns, err := intParam(r, "turns")
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	offset, err := intParam(r, "offset")
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := checkAgent(r.URL.Query().Get("agent")); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !h.ready(w) {
		return
	}
	h.sync(r)
	d, err := h.svc.Session(r.Context(), r.PathValue("id"), r.URL.Query().Get("agent"), turns, offset)
	switch {
	case errors.Is(err, obs.ErrNotFound):
		writeErr(w, http.StatusNotFound, err)
	case errors.Is(err, obs.ErrAmbiguous), errors.Is(err, obs.ErrTooShort):
		writeErr(w, http.StatusConflict, err)
	case err != nil:
		writeErr(w, http.StatusInternalServerError, err)
	default:
		writeJSON(w, http.StatusOK, d)
	}
}

func (h *obsHandler) usage(w http.ResponseWriter, r *http.Request) {
	f, err := parseUsageFilter(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !h.ready(w) {
		return
	}
	h.sync(r)
	rep, err := h.svc.Usage(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func parseSessionFilter(r *http.Request) (model.SessionFilter, error) {
	q := r.URL.Query()
	f := model.SessionFilter{
		Agent:   q.Get("agent"),
		Project: q.Get("project"),
		Q:       q.Get("q"),
		Sort:    q.Get("sort"),
	}
	if err := checkAgent(f.Agent); err != nil {
		return f, err
	}
	if f.Sort != "" && f.Sort != store.SortNewest && f.Sort != store.SortOldest {
		return f, errors.New("sort must be newest or oldest")
	}
	since, err := parseSince(q.Get("since"))
	if err != nil {
		return f, err
	}
	f.Since = since
	if f.Limit, err = intParam(r, "limit"); err != nil {
		return f, err
	}
	if f.Offset, err = intParam(r, "offset"); err != nil {
		return f, err
	}
	return f, nil
}

func parseUsageFilter(r *http.Request) (model.UsageFilter, error) {
	q := r.URL.Query()
	f := model.UsageFilter{Group: q.Get("group"), Agent: q.Get("agent")}
	if err := checkAgent(f.Agent); err != nil {
		return f, err
	}
	switch f.Group {
	case "", store.UsageGroupDaily, store.UsageGroupMonthly:
	default:
		return f, errors.New("group must be daily or monthly")
	}
	since, err := parseSince(q.Get("since"))
	if err != nil {
		return f, err
	}
	f.Since = since
	return f, nil
}

func checkAgent(a string) error {
	switch a {
	case "", model.AgentClaude, model.AgentCodex:
		return nil
	}
	return errors.New("agent must be claude-code or codex")
}

// parseSince accepts RFC3339 or a Nd/Nh shorthand. An unparseable value is a 400:
// silently ignoring it would return unfiltered data as if the filter applied.
func parseSince(v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	unit := v[len(v)-1]
	n, err := strconv.Atoi(strings.TrimSuffix(v, string(unit)))
	if err != nil || n < 0 {
		return time.Time{}, errors.New("since must be RFC3339 or a duration like 7d")
	}
	switch unit {
	case 'd':
		return time.Now().AddDate(0, 0, -n), nil
	case 'h':
		return time.Now().Add(-time.Duration(n) * time.Hour), nil
	}
	return time.Time{}, errors.New("since must be RFC3339 or a duration like 7d")
}

func intParam(r *http.Request, name string) (int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, errors.New(name + " must be a non-negative integer")
	}
	return n, nil
}
