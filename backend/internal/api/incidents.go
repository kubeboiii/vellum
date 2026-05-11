// Package api holds the HTTP handlers for the workflow + dashboard
// endpoints. Split from `internal/ingest` because these handlers are
// human-driven (low frequency, transactional reads/writes) whereas
// ingest is the hot path (rate-limited, non-blocking enqueue).
//
// Endpoints:
//
//	GET    /v1/incidents                  — live feed (non-CLOSED, sorted)
//	GET    /v1/incidents/:id              — detail (work_item + rca if any)
//	PATCH  /v1/incidents/:id/state        — advance state machine one step
//	POST   /v1/incidents/:id/rca          — submit RCA and close in one tx
//	GET    /v1/incidents/:id/signals      — paginated raw signals (Phase 5)
package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kubeboiii/ims/internal/model"
	"github.com/kubeboiii/ims/internal/persist/mongo"
	"github.com/kubeboiii/ims/internal/persist/pg"
	"github.com/kubeboiii/ims/internal/workflow"
)

// Handlers bundles the dependencies the API package needs. Constructed
// once in main.go. We use concrete repo pointers (not interfaces)
// because v1 has exactly one Postgres + Mongo implementation each;
// introducing interfaces for a single implementation is YAGNI.
type Handlers struct {
	WorkItems   *pg.WorkItemRepository
	RCA         *pg.RCARepository
	Signals     *mongo.SignalRepository
	Transitions *pg.TransitionReader
	Engine      *workflow.Engine
}

// RegisterRoutes mounts the endpoints onto a Gin router group.
// Call from main.go: `api.RegisterRoutes(v1, &api.Handlers{...})`.
func RegisterRoutes(rg *gin.RouterGroup, h *Handlers) {
	rg.GET("/incidents", h.ListIncidents)
	rg.GET("/incidents/closed", h.ListClosedIncidents)
	rg.GET("/incidents/:id", h.GetIncident)
	rg.GET("/incidents/:id/signals", h.ListSignals)
	rg.GET("/incidents/:id/transitions", h.ListTransitions)
	rg.PATCH("/incidents/:id/state", h.PatchState)
	rg.POST("/incidents/:id/rca", h.PostRCA)
}

// ---- GET /v1/incidents ----

// ListIncidents returns the non-CLOSED Work Items sorted by severity
// then last_signal_ts. Default cap 100; ?limit= overrides up to 500
// to keep response sizes bounded.
func (h *Handlers) ListIncidents(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := parseLimit(v); err == nil {
			limit = n
		}
	}
	items, err := h.WorkItems.ListActive(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

// ---- GET /v1/incidents/closed ----

// ListClosedIncidents returns the CLOSED Work Items sorted by
// closed_at DESC. Powers the post-mortem history page
// (`/incidents/closed` in the dashboard).
//
// Routed BEFORE the `:id` parameter route so the literal "closed"
// segment doesn't get parsed as a UUID. Gin's tree router handles
// this correctly because static segments outrank wildcards, but
// listing this explicitly above the wildcard registration in
// RegisterRoutes also documents the precedence.
func (h *Handlers) ListClosedIncidents(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := parseLimit(v); err == nil {
			limit = n
		}
	}
	items, err := h.WorkItems.ListClosed(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

// ---- GET /v1/incidents/:id/transitions ----

// ListTransitions returns the audit trail (state_transitions table
// rows) for one Work Item, chronological ascending. Powers the
// detail page's Timeline panel (PRD FR-7.2). Returns an empty list
// for unknown IDs rather than 404 — same trade-off as ListSignals.
func (h *Handlers) ListTransitions(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	rows, err := h.Transitions.ListByWorkItem(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows, "count": len(rows)})
}

// ---- GET /v1/incidents/:id ----

// GetIncident returns one WI plus its RCA (if any). 404 if no WI.
func (h *Handlers) GetIncident(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	wi, err := h.WorkItems.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pg.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "incident not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := gin.H{"work_item": wi}
	// RCA is only present for CLOSED incidents (since Phase 4's
	// POST /rca is the only thing that writes one). For other
	// statuses, skip the read — saves a Postgres round-trip.
	if wi.Status == model.StatusClosed {
		rca, err := h.RCA.GetByWorkItemID(c.Request.Context(), id)
		if err == nil {
			resp["rca"] = rca
		}
		// Silently ignore ErrNotFound — a closed WI without an RCA
		// shouldn't happen in normal flow but is harmless to display
		// as "no RCA".
	}
	c.JSON(http.StatusOK, resp)
}

// ---- GET /v1/incidents/:id/signals ----

// ListSignals returns the raw signals attached to a Work Item, newest
// first, paginated (FR-7.2). Default 50/page, max 200/page. Powers
// the detail page's "Raw signals" panel.
//
// We don't 404 on an unknown work_item_id here — the response is
// simply `{items: [], total: 0}`. That keeps the handler stateless
// (no Postgres lookup just to confirm existence), and the dashboard
// already 404s on the parent GET /v1/incidents/:id when needed.
func (h *Handlers) ListSignals(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	page := 1
	perPage := 50
	if v := c.Query("page"); v != "" {
		if n, err := parseLimit(v); err == nil {
			page = n
		}
	}
	if v := c.Query("per_page"); v != "" {
		if n, err := parseLimit(v); err == nil {
			perPage = n
		}
	}
	pageOut, err := h.Signals.ListByWorkItem(c.Request.Context(), id, page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pageOut)
}

// ---- PATCH /v1/incidents/:id/state ----

type patchStateRequest struct {
	To     model.Status `json:"to"     binding:"required"`
	Reason string       `json:"reason,omitempty"`
	Actor  string       `json:"actor,omitempty"`
}

// PatchState advances a Work Item's state one step. The State pattern's
// CanTransitionTo rules decide what's legal; this handler only does
// translation between HTTP and the workflow engine.
//
// Note: CLOSED is intentionally NOT reachable via this endpoint.
// Closing requires an RCA, which goes through POST /rca. Attempting
// PATCH state to:"CLOSED" returns 422 — the rule lives in
// ResolvedState.CanTransitionTo (and we surface it here as the State
// pattern would).
func (h *Handlers) PatchState(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req patchStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
		return
	}
	next, err := workflow.FromStatus(req.To)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tctx := workflow.TransitionContext{
		Reason: req.Reason,
		Actor:  req.Actor,
	}
	wi, err := h.Engine.Transition(c.Request.Context(), id, next, tctx)
	if err != nil {
		writeWorkflowError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"work_item": wi})
}

// ---- POST /v1/incidents/:id/rca ----

type postRCARequest struct {
	RootCauseCategory model.RootCauseCategory `json:"root_cause_category"`
	FixApplied        string                  `json:"fix_applied"`
	PreventionSteps   string                  `json:"prevention_steps"`
	SubmittedBy       string                  `json:"submitted_by"`
}

// PostRCA inserts an RCA AND closes the work_item in one transaction
// (workflow.Engine.CloseWithRCA). The State pattern still gates the
// close — ResolvedState.CanTransitionTo returns ErrMissingRCA /
// ErrIncompleteRCA which we map to 422.
func (h *Handlers) PostRCA(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req postRCARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
		return
	}
	rca := &model.RCA{
		RootCauseCategory: req.RootCauseCategory,
		FixApplied:        req.FixApplied,
		PreventionSteps:   req.PreventionSteps,
		SubmittedBy:       req.SubmittedBy,
	}
	wi, savedRCA, err := h.Engine.CloseWithRCA(c.Request.Context(), id, rca, req.SubmittedBy)
	if err != nil {
		writeWorkflowError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"work_item": wi,
		"rca":       savedRCA,
	})
}
