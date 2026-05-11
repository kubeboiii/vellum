package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kubeboiii/ims/internal/persist/pg"
	"github.com/kubeboiii/ims/internal/workflow"
)

// parseID extracts the `:id` path parameter as a UUID. Writes the
// 400 response itself and returns ok=false on failure so the handler
// can early-return.
func parseID(c *gin.Context) (uuid.UUID, bool) {
	raw := c.Param("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a UUID"})
		return uuid.Nil, false
	}
	return id, true
}

// parseLimit accepts the ?limit= query string. Caps at 500 to keep
// list responses bounded; rejects negatives.
func parseLimit(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if n < 1 {
		return 0, errors.New("limit must be >= 1")
	}
	if n > 500 {
		n = 500
	}
	return n, nil
}

// writeWorkflowError maps the workflow / pg sentinels to the right
// HTTP status codes and response body shapes.
//
// The mapping (matches PRD G3 and 03-api-contract sketch):
//
//	pg.ErrNotFound              → 404 {"error": "incident not found"}
//	workflow.ErrInvalidTransition→ 409 {"error": "transition not allowed: ..."}
//	workflow.ErrMissingRCA      → 422 {"error": "rca is required"}
//	workflow.ErrIncompleteRCA   → 422 {"errors": [{field, error}, ...]}
//	anything else               → 500 {"error": err.Error()}
//
// Putting this in one place means the four handlers all surface the
// same shape — a frontend dev relying on `response.errors[]` doesn't
// have to special-case each endpoint.
func writeWorkflowError(c *gin.Context, err error) {
	var ire *workflow.IncompleteRCAError
	switch {
	case errors.Is(err, pg.ErrNotFound), errors.Is(err, workflow.ErrWorkItemNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "incident not found"})
	case errors.As(err, &ire):
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":  "rca is incomplete",
			"fields": ire.Fields,
		})
	case errors.Is(err, workflow.ErrMissingRCA):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, workflow.ErrInvalidTransition):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
