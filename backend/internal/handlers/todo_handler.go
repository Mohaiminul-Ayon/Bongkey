package handlers

import (
	"context"
	"net/http"

	"bongkey/backend/internal/dashboard"
	"github.com/gin-gonic/gin"
)

// todoReader is the count query used by TodoHandler.
type todoReader interface {
	Todo(ctx context.Context) (dashboard.Todo, error)
}

// TodoHandler gives staff the waiting-work counts for the dashboard and menu badges; routes put it behind AdminAuth.
type TodoHandler struct{ Reader todoReader }

// Counts returns how many orders, payments, call-backs and returns are waiting.
func (h TodoHandler) Counts(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	todo, err := h.Reader.Todo(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load the to-do counts"})
		return
	}
	c.JSON(http.StatusOK, todo)
}
