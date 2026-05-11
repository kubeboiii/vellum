package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kubeboiii/vellum/internal/persist/pg"
	"github.com/kubeboiii/vellum/internal/workflow"
)

func parseID(c *gin.Context) (uuid.UUID, bool) {
	raw := c.Param("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a UUID"})
		return uuid.Nil, false
	}
	return id, true
}

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
	case errors.Is(err, pg.ErrSerializationFailure):
		c.JSON(http.StatusConflict, gin.H{
			"error": "concurrent update detected; please retry",
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
