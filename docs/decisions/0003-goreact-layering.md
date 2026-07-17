# 0003 — goreact layering: one Go core, many frontends

Status: accepted (2026-07-17)
Context: the `vd obs` feature (local observability over Claude Code + Codex transcripts) and its web portal.

## Decision

Structure a Go-backend + React-frontend feature so one core serves every surface — CLI, HTTP API, and SPA — with no logic duplicated across them. The backend follows the `fastreact` layer roles, translated to Go; the frontend follows `fastreact` verbatim.

### The layer map (fastreact → Go)

| fastreact role | here | what lives there |
|---|---|---|
| `schemas/` (DTOs, contracts) | `internal/obs/model/` | wire types + json tags — the frozen contract |
| `models/` (tables) | `internal/obs/store/` | sqlite schema, queries, transactions |
| `clients/` (I/O seams) | `internal/obs/ingest/` | transcript parsers |
| `services/` (stateless logic) | `internal/obs/service.go` | the seam — cost, clamping, filtering |
| `apis/v1/` (thin routers) | `internal/ui/web/obs_handlers.go` | serialize the service, no logic |

Note the vocabulary flip that trips people: fastreact's `models/` are database tables → our `store/`; fastreact's `schemas/` are DTOs → our `model/`. Pin it here so nobody files a DTO under `store/` or a table under `model/`.

### Rules that make it hold

- **Service-as-seam.** Cost, cache-hit math, default-clamping, and filtering live in `obs.Service`, once. The CLI calls it in-process; the HTTP handlers serialize it. They agree by construction, not by discipline — proven: `vd obs usage` and `GET /api/obs/usage` return byte-identical totals off the same synced cache.
- **Stateful vs stateless facade.** `obs.Service` owns a `*sql.DB`, so it has `Close()` and its constructor returns an error. `inventory.Service` (the earlier one) is a stateless struct of paths. `obs.Service` is the canonical shape going forward; `inventory.Service` predates it.
- **Contract-first DTO freeze.** `model/model.go` freezes DTOs + filters + the envelope + json casing (flat lowercase) *before* any consumer is written. A contract that names types but not their wire form isn't a contract.
- **Named-collection envelope.** List endpoints return `{resource, total, limit, offset}`, matching the existing `{hooks: …}`; single GETs are bare. Never `{data: …}`.
- **Never-guess pricing, one package.** `internal/obs/pricing` — unknown model → `(0, false)` rendered as `?`, never a fake `$0.00`. Exactly one cost path; the frontend re-derives nothing.
- **`{"error": …}` envelope**, not fastreact's `{"detail": …}` — matches vd-cli's four pre-existing endpoints and its `errors.Is`→status mapper. A deliberate deviation from fastreact, recorded so goreact states it rather than inheriting the wording.
- **Deny-list before the SPA catch-all.** `mux.Handle("/api/", NotFoundHandler)` method-agnostic, so an unmatched or wrong-verb `/api/*` 404s instead of returning 200 index.html.

### Frontend

`fastreact` verbatim: `app/routes` (TanStack file-based), `features/<slice>/{schemas,queries,components}`, `components/{ui,layout}`, `lib/`, `config/env.ts`. Deviations: no auth/RBAC (localhost read-only), no Docker/Postgres, `fetch` in `lib/api-client.ts` instead of axios (no auth headers or FormData to justify it).

## Reference implementation

`internal/obs/` (core → service → CLI + API) and `web/src/features/obs/` (the SPA slice). This is the pattern a future `goreact` skill would extract — but only once it has shipped more than once. Extract what ran, not what was planned.
