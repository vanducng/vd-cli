package web

import (
	"errors"
	"log"
	"net/http"
	"strconv"
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
	mux.HandleFunc("GET /api/obs/health", h.health)
	mux.HandleFunc("GET /api/obs/skills", h.skills)
	mux.HandleFunc("GET /api/obs/hooks", h.hooks)
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

// refresh syncs the cache before a read. It runs inline in the request: the
// first cold request pays the sync latency; concurrent ones return immediately
// via the service's debounce.
func (h *obsHandler) refresh(r *http.Request) {
	// A failed sync serves whatever the cache already has rather than erroring the
	// read — but log it, so silent staleness is at least observable server-side.
	if _, err := h.svc.Sync(r.Context(), ingest.SyncOptions{}); err != nil {
		log.Printf("obs: background sync failed, serving cached data: %v", err)
	}
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
	h.refresh(r)
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
	h.refresh(r)
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
	h.refresh(r)
	rep, err := h.svc.Usage(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (h *obsHandler) health(w http.ResponseWriter, r *http.Request) {
	f, err := parseHealthFilter(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !h.ready(w) {
		return
	}
	h.refresh(r)
	rep, err := h.svc.Health(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (h *obsHandler) skills(w http.ResponseWriter, r *http.Request) {
	f, err := parseSkillFilter(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !h.ready(w) {
		return
	}
	h.refresh(r)
	rep, err := h.svc.Skills(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (h *obsHandler) hooks(w http.ResponseWriter, r *http.Request) {
	f, err := parseHookFilter(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !h.ready(w) {
		return
	}
	h.refresh(r)
	rep, err := h.svc.Hooks(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// parseCommonFilter reads the agent/project/since triple that health, skills
// and hooks all share, so a validation rule added here can never apply to one
// and miss the others.
func parseCommonFilter(r *http.Request) (agent, project string, since time.Time, err error) {
	q := r.URL.Query()
	agent, project = q.Get("agent"), q.Get("project")
	if err = checkAgent(agent); err != nil {
		return "", "", time.Time{}, err
	}
	if since, err = store.ParseSince(q.Get("since")); err != nil {
		return "", "", time.Time{}, err
	}
	return agent, project, since, nil
}

func parseHealthFilter(r *http.Request) (model.HealthFilter, error) {
	agent, project, since, err := parseCommonFilter(r)
	return model.HealthFilter{Agent: agent, Project: project, Since: since}, err
}

func parseSkillFilter(r *http.Request) (model.SkillFilter, error) {
	agent, project, since, err := parseCommonFilter(r)
	return model.SkillFilter{Agent: agent, Project: project, Since: since}, err
}

func parseHookFilter(r *http.Request) (model.HookFilter, error) {
	agent, project, since, err := parseCommonFilter(r)
	return model.HookFilter{Agent: agent, Project: project, Since: since}, err
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
	since, err := store.ParseSince(q.Get("since"))
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
	since, err := store.ParseSince(q.Get("since"))
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
