package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"kasir-api-go/db"
)

type StoreHandler struct {
	queries *db.Queries
}

func NewStoreHandler(queries *db.Queries) *StoreHandler {
	return &StoreHandler{queries: queries}
}

type UpdateStoreRequest struct {
	StoreName string `json:"store_name"`
	Address   string `json:"address"`
	Phone     string `json:"phone"`
}

func (h *StoreHandler) UpdateStore(c *gin.Context) {
	// Must be owner
	role, exists := c.Get("role")
	if !exists || role.(string) != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only store owner can update store details"})
		return
	}

	storeIDStr, exists := c.Get("store_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Store ID missing in token"})
		return
	}

	uid, err := uuid.Parse(storeIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid store ID format"})
		return
	}
	storeIDBytes := uid[:]

	var req UpdateStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.StoreName = SanitizeText(req.StoreName)
	req.Address = SanitizeText(req.Address)
	req.Phone = SanitizeText(req.Phone)

	arg := db.UpdateStoreParams{
		ID:        pgtype.UUID{Bytes: [16]byte(storeIDBytes), Valid: true},
		StoreName: req.StoreName,
		Address:   pgtype.Text{String: req.Address, Valid: true},
		Phone:     pgtype.Text{Valid: false}, // Don't wipe phone number
	}

	store, err := h.queries.UpdateStore(c.Request.Context(), arg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update store"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Store updated successfully",
		"store":   store,
	})
}

func (h *StoreHandler) UploadStoreLogo(c *gin.Context) {
	// Must be owner
	role, exists := c.Get("role")
	if !exists || role.(string) != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only store owner can upload logo"})
		return
	}

	storeIDStr, exists := c.Get("store_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Store ID missing in token"})
		return
	}

	uid, err := uuid.Parse(storeIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid store ID format"})
		return
	}
	storeIDBytes := uid[:]

	store, err := h.queries.GetStore(c.Request.Context(), pgtype.UUID{Bytes: [16]byte(storeIDBytes), Valid: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Store not found"})
		return
	}

	file, err := c.FormFile("logo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to upload logo: " + err.Error()})
		return
	}

	// Validasi ukuran, ekstensi (whitelist), dan MIME dari isi file.
	if err := ValidateImageFile(file); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Delete old logo if exists
	if store.LogoUrl.Valid && store.LogoUrl.String != "" {
		// logo url format is usually /uploads/id/filename
		// we strip the leading slash to make it a relative path
		oldPath := strings.TrimPrefix(store.LogoUrl.String, "/")
		// Strip query parameters from old path if they exist
		if idx := strings.Index(oldPath, "?"); idx != -1 {
			oldPath = oldPath[:idx]
		}
		if oldPath != "" {
			_ = os.Remove(oldPath) // Ignore error if file doesn't exist
		}
	}

	// Use store ID for folder creation to ensure consistency even if store name changes
	storeIdString := uid.String()

	// Normalisasi ekstensi ke huruf kecil (cegah .PNG / .JPG casing aneh).
	extension := strings.ToLower(filepath.Ext(file.Filename))
	filename := "logo" + extension

	dirPath := filepath.Join("uploads", storeIdString)
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	uploadPath := filepath.Join(dirPath, filename)

	if err := c.SaveUploadedFile(file, uploadPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save logo"})
		return
	}

	imageURL := fmt.Sprintf("/uploads/%s/%s?t=%d", storeIdString, filename, time.Now().Unix())

	// Update store with logo URL
	arg := db.UpdateStoreParams{
		ID:      pgtype.UUID{Bytes: [16]byte(storeIDBytes), Valid: true},
		LogoUrl: pgtype.Text{String: imageURL, Valid: true},
	}

	arg.StoreName = store.StoreName
	arg.Address = pgtype.Text{Valid: false}
	arg.Phone = pgtype.Text{Valid: false}

	_, err = h.queries.UpdateStore(c.Request.Context(), arg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update store logo in database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Logo uploaded successfully",
		"logo_url": imageURL,
	})
}
