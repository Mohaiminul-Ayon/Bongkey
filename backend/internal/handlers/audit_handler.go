package handlers

import (
	"context"
	"net/http"
	"strconv"

	"bongkey/backend/internal/audit"
	"github.com/gin-gonic/gin"
)

// auditReader is the audit log read used by AuditHandler.
type auditReader interface {
	Recent(ctx context.Context, beforeID int64) ([]audit.Entry, error)
}

// AuditHandler shows the owner the audit log; routes put it behind AdminAuth.
type AuditHandler struct{ Reader auditReader }

// List returns one page, newest first; ?before=<id> pages back.
func (h AuditHandler) List(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var before int64
	if raw := c.Query("before"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page"})
			return
		}
		before = parsed
	}
	entries, err := h.Reader.Recent(c.Request.Context(), before)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load the audit log"})
		return
	}
	c.JSON(http.StatusOK, entries)
}
