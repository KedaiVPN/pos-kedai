package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kasir-api-go/db"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	DraftHashKey = "off:drafts"
	OFFAPI       = "https://id.openfoodfacts.org/api/v2"
)

type OFFHandler struct {
	queries  *db.Queries
	draftRdb *redis.Client
}

func NewOFFHandler(queries *db.Queries, draftRdb *redis.Client) *OFFHandler {
	return &OFFHandler{queries: queries, draftRdb: draftRdb}
}

type OFFProduct struct {
	Barcode       string `json:"code"`
	ProductName   string `json:"product_name"`
	ProductNameID string `json:"product_name_id"`
	Brands        string `json:"brands"`
	Categories    string `json:"categories"`
	ImageURL      string `json:"image_front_url"`
}

type OFFSearchResponse struct {
	Count    int          `json:"count"`
	Products []OFFProduct `json:"products"`
}

type DraftProduct struct {
	Barcode       string `json:"barcode"`
	RawName       string `json:"raw_name"`
	Brand         string `json:"brand"`
	RawCategory   string `json:"raw_category"`
	ImageURL      string `json:"image_url"`
	FetchedAt     int64  `json:"fetched_at"`
}

// FetchFromOFF - Admin menarik batch produk dari Open Food Facts
func (h *OFFHandler) FetchFromOFF(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "25")

	// Ambil data dari Open Food Facts Indonesia
	url := fmt.Sprintf("%s/search?countries_tags_en=indonesia&page=%s&page_size=%s&fields=code,product_name,product_name_id,brands,categories,image_front_url",
		OFFAPI, page, pageSize)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	req.Header.Set("User-Agent", "PosKedaiAdmin - WebApp - Version 1.0 - admin@poskedai.my.id")

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch from OFF: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, gin.H{
			"error":  "Open Food Facts API returned non-200 status",
			"status": resp.StatusCode,
			"body":   string(body),
		})
		return
	}

	var offResp OFFSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&offResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode OFF response"})
		return
	}

	ctx := c.Request.Context()
	fetchedAt := time.Now().Unix()
	inserted := 0
	skipped := 0

	for _, prod := range offResp.Products {
		// Skip jika barcode kosong
		if prod.Barcode == "" {
			skipped++
			continue
		}

		// Cek apakah barcode sudah ada di master_products PostgreSQL
		exists, err := h.queries.CheckMasterProductBarcodeExists(ctx, prod.Barcode)
		if err == nil && exists {
			skipped++
			continue
		}

		// Cek apakah sudah ada di Redis draft
		existsInDraft, err := h.draftRdb.HExists(ctx, DraftHashKey, prod.Barcode).Result()
		if err == nil && existsInDraft {
			skipped++
			continue
		}

		// Simpan ke Redis DB 1
		draft := DraftProduct{
			Barcode:     prod.Barcode,
			RawName:     prod.ProductName,
			Brand:       prod.Brands,
			RawCategory: prod.Categories,
			ImageURL:    prod.ImageURL,
			FetchedAt:   fetchedAt,
		}

		draftJSON, err := json.Marshal(draft)
		if err != nil {
			continue
		}

		err = h.draftRdb.HSet(ctx, DraftHashKey, prod.Barcode, string(draftJSON)).Err()
		if err != nil {
			continue
		}

		inserted++
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Batch fetch completed",
		"total":        len(offResp.Products),
		"inserted":     inserted,
		"skipped":      skipped,
		"off_count":    offResp.Count,
		"current_page": page,
	})
}

// ListDrafts - Admin melihat daftar draft produk yang perlu dikurasi
func (h *OFFHandler) ListDrafts(c *gin.Context) {
	ctx := c.Request.Context()

	// Ambil semua draft dari Redis Hash
	draftsMap, err := h.draftRdb.HGetAll(ctx, DraftHashKey).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch drafts from Redis"})
		return
	}

	drafts := make([]DraftProduct, 0, len(draftsMap))
	for _, draftJSON := range draftsMap {
		var draft DraftProduct
		if err := json.Unmarshal([]byte(draftJSON), &draft); err != nil {
			continue
		}
		drafts = append(drafts, draft)
	}

	c.JSON(http.StatusOK, gin.H{
		"count":  len(drafts),
		"drafts": drafts,
	})
}

type ApproveDraftRequest struct {
	Barcode    string `json:"barcode" binding:"required"`
	Name       string `json:"name" binding:"required"`
	CategoryID string `json:"category_id" binding:"required"`
	Unit       string `json:"unit"`
}

// ApproveDraft - Admin approve draft dan masukkan ke master_products PostgreSQL
func (h *OFFHandler) ApproveDraft(c *gin.Context) {
	var req ApproveDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Ambil draft dari Redis
	draftJSON, err := h.draftRdb.HGet(ctx, DraftHashKey, req.Barcode).Result()
	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Draft not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch draft from Redis"})
		return
	}

	var draft DraftProduct
	if err := json.Unmarshal([]byte(draftJSON), &draft); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse draft"})
		return
	}

	// Parse category_id (bisa UUID atau nama kategori)
	var categoryID pgtype.UUID
	uid, err := uuid.Parse(req.CategoryID)
	if err == nil {
		categoryID = pgtype.UUID{Bytes: uid, Valid: true}
	} else {
		// Nama kategori, cari atau buat
		categoryName := req.CategoryID
		category, err := h.queries.GetCategoryByName(ctx, categoryName)
		if err != nil {
			slug := strings.ToLower(strings.ReplaceAll(categoryName, " ", "-"))
			newCategory, err := h.queries.CreateCategory(ctx, db.CreateCategoryParams{
				Name: categoryName,
				Slug: slug,
			})
			if err == nil {
				categoryID = newCategory.ID
			} else {
				categoryID = pgtype.UUID{Valid: false}
			}
		} else {
			categoryID = category.ID
		}
	}

	// Ambil user ID dari context (admin yang login)
	userID, exists := c.Get("user_id")
	var createdBy pgtype.UUID
	if exists {
		if uid, err := uuid.Parse(fmt.Sprintf("%v", userID)); err == nil {
			createdBy = pgtype.UUID{Bytes: uid, Valid: true}
		}
	}

	// Create master product
	unit := req.Unit
	if unit == "" {
		unit = "pcs"
	}

	arg := db.CreateMasterProductParams{
		Barcode:            req.Barcode,
		Name:               req.Name,
		PhotoUrl:           pgtype.Text{String: draft.ImageURL, Valid: draft.ImageURL != ""},
		PhotoPath:          pgtype.Text{Valid: false},
		CategoryID:         categoryID,
		BrandID:            pgtype.UUID{Valid: false},
		Unit:               pgtype.Text{String: unit, Valid: true},
		Source:             pgtype.Text{String: "open_food_facts", Valid: true},
		IsGeneratedBarcode: pgtype.Bool{Bool: false, Valid: true},
		CreatedBy:          createdBy,
	}

	masterProduct, err := h.queries.CreateMasterProduct(ctx, arg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create master product: %v", err)})
		return
	}

	// Hapus dari Redis draft
	h.draftRdb.HDel(ctx, DraftHashKey, req.Barcode)

	c.JSON(http.StatusOK, gin.H{
		"message": "Draft approved and saved to master products",
		"product": masterProduct,
	})
}

// RejectDraft - Admin skip/reject draft, hapus dari Redis
func (h *OFFHandler) RejectDraft(c *gin.Context) {
	barcode := c.Param("barcode")
	if barcode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Barcode is required"})
		return
	}

	ctx := c.Request.Context()

	// Hapus dari Redis
	deleted, err := h.draftRdb.HDel(ctx, DraftHashKey, barcode).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete draft"})
		return
	}

	if deleted == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Draft not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Draft rejected and removed",
		"barcode": barcode,
	})
}
