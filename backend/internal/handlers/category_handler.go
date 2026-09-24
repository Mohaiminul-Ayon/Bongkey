package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"bongkey/backend/internal/database"
	"bongkey/backend/internal/models"

	"github.com/gin-gonic/gin"
)

type categoryRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

func ListCategories(c *gin.Context) {
	if !database.Ready() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not connected"})
		return
	}
	rows, err := database.DB.Query(`SELECT id, name, slug, description FROM categories ORDER BY id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load categories"})
		return
	}
	defer rows.Close()
	var categories []models.Category
	for rows.Next() {
		var category models.Category
		if err := rows.Scan(&category.ID, &category.Name, &category.Slug, &category.Description); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read categories"})
			return
		}
		categories = append(categories, category)
	}
	c.JSON(http.StatusOK, categories)
}

func CreateCategory(c *gin.Context) {
	saveCategory(c, 0)
}

func UpdateCategory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	saveCategory(c, id)
}

func saveCategory(c *gin.Context, id int) {
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category name is required"})
		return
	}
	if req.Slug == "" {
		req.Slug = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	}
	if id == 0 {
		_, err := database.DB.Exec(`INSERT INTO categories (name, slug, description) VALUES ($1,$2,$3)`, req.Name, req.Slug, req.Description)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create category"})
			return
		}
	} else {
		_, err := database.DB.Exec(`UPDATE categories SET name=$1, slug=$2, description=$3, updated_at=NOW() WHERE id=$4`, req.Name, req.Slug, req.Description, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update category"})
			return
		}
	}
	ListCategories(c)
}

func DeleteCategory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := database.DB.Exec(`DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete category"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
