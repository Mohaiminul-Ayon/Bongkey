package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"bongkey/backend/internal/catalog"
	"bongkey/backend/internal/models"

	"github.com/gin-gonic/gin"
)

type categoryRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

// CategoryHandler maps category HTTP input to catalog operations.
type CategoryHandler struct{ Service catalog.CategoryService }

// ListCategories returns categories used by both public and owner pages.
func (h CategoryHandler) ListCategories(c *gin.Context) {
	categories, err := h.Service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load categories"})
		return
	}
	c.JSON(http.StatusOK, categories)
}

// CreateCategory validates HTTP input and creates one category.
func (h CategoryHandler) CreateCategory(c *gin.Context) { h.saveCategory(c, 0) }

// UpdateCategory validates the path ID before editing one category.
func (h CategoryHandler) UpdateCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid category ID is required"})
		return
	}
	h.saveCategory(c, id)
}

// saveCategory parses input, delegates rules, and returns the refreshed list.
func (h CategoryHandler) saveCategory(c *gin.Context, id int) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category data"})
		return
	}
	err := h.Service.Save(c.Request.Context(), id, models.Category{Name: req.Name, Slug: req.Slug, Description: req.Description})
	if errors.Is(err, catalog.ErrCategoryNameRequired) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category name is required"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save category"})
		return
	}
	h.ListCategories(c)
}

// DeleteCategory validates the path ID and removes one category.
func (h CategoryHandler) DeleteCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid category ID is required"})
		return
	}
	if err := h.Service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete category"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
