package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"bongkey/backend/internal/lens"
	"github.com/gin-gonic/gin"
)

type lensPackageRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       int     `json:"price"`
	SortOrder   int     `json:"sort_order"`
	Active      bool    `json:"active"`
	CategoryIDs []int64 `json:"category_ids"`
}

// LensHandler maps lens package HTTP input to lens operations.
type LensHandler struct{ Service lens.Service }

// ListForProduct returns the enabled packages a shopper may pick for one frame.
func (h LensHandler) ListForProduct(c *gin.Context) {
	packages, err := h.Service.ForProduct(c.Request.Context(), c.Param("slug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load lens options"})
		return
	}
	c.JSON(http.StatusOK, packages)
}

// AdminListPackages returns every package, including disabled ones, after AdminAuth.
func (h LensHandler) AdminListPackages(c *gin.Context) {
	packages, err := h.Service.ListAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load lens packages"})
		return
	}
	c.JSON(http.StatusOK, packages)
}

// CreatePackage creates one lens package.
func (h LensHandler) CreatePackage(c *gin.Context) { h.savePackage(c, 0) }

// UpdatePackage validates the path ID before editing one lens package.
func (h LensHandler) UpdatePackage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid lens package ID is required"})
		return
	}
	h.savePackage(c, id)
}

// savePackage parses bounded input, delegates rules, and returns the refreshed list.
func (h LensHandler) savePackage(c *gin.Context, id int) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req lensPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lens package data"})
		return
	}
	err := h.Service.Save(c.Request.Context(), id, lens.Package{
		Name: req.Name, Description: req.Description, Price: req.Price, SortOrder: req.SortOrder, Active: req.Active, CategoryIDs: req.CategoryIDs,
	})
	switch {
	case errors.Is(err, lens.ErrInvalidPackage):
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, a price above 0 and valid categories are required"})
	case errors.Is(err, lens.ErrDuplicateName):
		c.JSON(http.StatusConflict, gin.H{"error": "another lens package already has this name"})
	case errors.Is(err, lens.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "lens package not found"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save lens package"})
	default:
		h.AdminListPackages(c)
	}
}
