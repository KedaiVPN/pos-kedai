package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"kasir-api-go/db"
)

type CategoryHandler struct {
	queries *db.Queries
}

func NewCategoryHandler(queries *db.Queries) *CategoryHandler {
	return &CategoryHandler{queries: queries}
}

// GetAllCategories returns all categories
func (h *CategoryHandler) GetAllCategories(c *gin.Context) {
	categories, err := h.queries.GetAllCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, categories)
}

// CreateCategory adds a new category
func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category name cannot be empty"})
		return
	}

	// Check if already exists
	existing, err := h.queries.GetCategoryByName(c.Request.Context(), name)
	if err == nil {
		c.JSON(http.StatusOK, existing)
		return
	}

	// Create slug
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	category, err := h.queries.CreateCategory(c.Request.Context(), db.CreateCategoryParams{
		Name: name,
		Slug: slug,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, category)
}
