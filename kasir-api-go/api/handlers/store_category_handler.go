package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"kasir-api-go/db"
)

type StoreCategoryHandler struct {
	queries *db.Queries
}

func NewStoreCategoryHandler(queries *db.Queries) *StoreCategoryHandler {
	return &StoreCategoryHandler{queries: queries}
}

// GetStoreCategories returns all categories for a store
func (h *StoreCategoryHandler) GetStoreCategories(c *gin.Context) {
	storeID, exists := c.Get("store_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Store ID not found"})
		return
	}

	storeIDStr, ok := storeID.(string)
	if !ok || storeIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid store ID"})
		return
	}

	// Parse UUID
	var uuid pgtype.UUID
	err := uuid.Scan(storeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid store ID format"})
		return
	}

	categories, err := h.queries.GetAllStoreCategories(c.Request.Context(), uuid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	if categories == nil {
		categories = []db.StoreCategory{}
	}

	c.JSON(http.StatusOK, categories)
}

// CreateStoreCategory adds a new category for a store
func (h *StoreCategoryHandler) CreateStoreCategory(c *gin.Context) {
	storeID, exists := c.Get("store_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Store ID not found"})
		return
	}

	storeIDStr, ok := storeID.(string)
	if !ok || storeIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid store ID"})
		return
	}

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

	// Parse UUID
	var uuid pgtype.UUID
	err := uuid.Scan(storeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid store ID format"})
		return
	}

	// Check if category already exists for this store
	existing, err := h.queries.GetStoreCategoryByName(c.Request.Context(), db.GetStoreCategoryByNameParams{
		StoreID: uuid,
		Name:    name,
	})
	if err == nil {
		// Category already exists
		c.JSON(http.StatusOK, existing)
		return
	}

	// Create slug
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	category, err := h.queries.CreateStoreCategory(c.Request.Context(), db.CreateStoreCategoryParams{
		StoreID: uuid,
		Name:    name,
		Slug:    slug,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, category)
}
