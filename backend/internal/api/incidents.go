package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kubeboiii/vellum/internal/model"
	"github.com/kubeboiii/vellum/internal/persist/mongo"
	"github.com/kubeboiii/vellum/internal/persist/pg"
	"github.com/kubeboiii/vellum/internal/workflow"
)

type Handlers struct {
	WorkItems   *pg.WorkItemRepository
	RCA         *pg.RCARepository
	Signals     *mongo.SignalRepository
	Transitions *pg.TransitionReader
	Engine      *workflow.Engine
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handlers) {
	rg.GET("/incidents", h.ListIncidents)
	rg.GET("/incidents/closed", h.ListClosedIncidents)
	rg.GET("/incidents/:id", h.GetIncident)
	rg.GET("/incidents/:id/signals", h.ListSignals)
	rg.GET("/incidents/:id/transitions", h.ListTransitions)
	rg.PATCH("/incidents/:id/state", h.PatchState)
	rg.POST("/incidents/:id/rca", h.PostRCA)
}

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

	if wi.Status == model.StatusClosed {
		rca, err := h.RCA.GetByWorkItemID(c.Request.Context(), id)
		if err == nil {
			resp["rca"] = rca
		}

	}
	c.JSON(http.StatusOK, resp)
}

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

type patchStateRequest struct {
	To     model.Status `json:"to"     binding:"required"`
	Reason string       `json:"reason,omitempty"`
	Actor  string       `json:"actor,omitempty"`
}

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

type postRCARequest struct {
	RootCauseCategory model.RootCauseCategory `json:"root_cause_category"`
	FixApplied        string                  `json:"fix_applied"`
	PreventionSteps   string                  `json:"prevention_steps"`
	SubmittedBy       string                  `json:"submitted_by"`
}

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
