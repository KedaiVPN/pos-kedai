package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	OFFUserAgent = "PosKedaiAdmin - Web - Version 1.0 - admin@poskedai.my.id"
)

// IndonesianPopularBrands - Daftar brand FMCG terlaris di Indonesia (Kantar 2024-2025)
var IndonesianPopularBrands = []string{
	// Makanan Instan & Mie
	"indomie", "mie sedaap", "sarimi", "supermi", "pop mie", "bihunku", "sukses", "gaga",
	// Biskuit & Snack
	"roma", "nabati", "gery", "chitato", "lays", "qtela", "tango", "oreo", "khong guan",
	"monde", "slai olai", "malkist", "beng-beng", "choki choki", "momogi", "taro",
	// Bumbu & Penyedap
	"royco", "masako", "bango", "sasa", "ajinomoto", "sedaap", "indofood", "abc", "ladaku", "desaku",
	// Kopi & Minuman
	"kapal api", "torabika", "luwak", "good day", "nescafe", "teh botol", "teh pucuk",
	"le minerale", "aqua", "cleo", "ultra milk", "indomilk", "dancow", "frisian flag",
	"bear brand", "marjan", "nutrisari", "floridina", "pocari", "mizone",
	// Sembako & Minyak
	"sunco", "bimoli", "filma", "tropical", "sania", "fortune", "rose brand", "segitiga biru",
	"gulaku", "cakra kembar", "kunci biru",
	// Sabun & Perawatan Rumah
	"soklin", "daia", "rinso", "sunlight", "mama lemon", "ekonomi", "downy", "molto",
	"wipol", "super pell", "bayfresh", "hit", "vape", "baygon",
	// Perawatan Pribadi
	"lifebuoy", "lux", "giv", "nuvo", "pepsodent", "ciptadent", "close up", "pantene",
	"sunsilk", "clear", "head & shoulders", "rejoice", "shinzui", "wardah", "kahf",
	// Frozen Food & Olahan
	"kanzler", "so good", "fiesta", "champ", "belfoods", "bernardi",
}

// isIndonesianBrand mengecek apakah brand produk termasuk brand populer Indonesia
func isIndonesianBrand(brand, productName string) bool {
	combined := strings.ToLower(brand + " " + productName)
	for _, b := range IndonesianPopularBrands {
		if strings.Contains(combined, b) {
			return true
		}
	}
	return false
}

// downloadAndSaveImage mengunduh gambar dari OFF dan menyimpannya di folder uploads lokal
func downloadAndSaveImage(imageURL, barcode string) (string, error) {
	if imageURL == "" {
		return "", nil
	}

	// Anti-SSRF: tolak URL ke localhost / private network / scheme non-HTTP.
	if !IsSafeExternalURL(imageURL) {
		return "", fmt.Errorf("unsafe image URL rejected")
	}

	// Cegah path traversal lewat barcode saat dipakai di nama file.
	safeBarcode := SanitizeBarcode(barcode)
	if safeBarcode == "" {
		safeBarcode = "unknown"
	}

	// Jangan ikuti redirect ke host internal (SSRF via 302).
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			if !IsSafeExternalURL(r.URL.String()) {
				return fmt.Errorf("redirect to unsafe host rejected")
			}
			return nil
		},
	}
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", OFFUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch image: status %d", resp.StatusCode)
	}

	// Batasi ukuran unduhan (disk exhaustion guard).
	limited := io.LimitReader(resp.Body, MaxUploadSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if int64(len(data)) > MaxUploadSize {
		return "", fmt.Errorf("image exceeds max size %d bytes", MaxUploadSize)
	}

	// Validasi konten benar-benar gambar (MIME sniffing), bukan percaya ekstensi URL.
	ext := ""
	switch http.DetectContentType(data) {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	default:
		return "", fmt.Errorf("downloaded content is not a supported image")
	}

	uploadDir := "uploads"
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("off_%s_%d%s", safeBarcode, time.Now().Unix(), ext)
	filePath := filepath.Join(uploadDir, filename)

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return "", err
	}

	return fmt.Sprintf("/uploads/%s", filename), nil
}

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
	Barcode     string `json:"barcode"`
	RawName     string `json:"raw_name"`
	Brand       string `json:"brand"`
	RawCategory string `json:"raw_category"`
	ImageURL    string `json:"image_url"`
	FetchedAt   int64  `json:"fetched_at"`
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

		// Filter: Hanya produk dengan brand Indonesia populer
		nameToUse := prod.ProductNameID
		if nameToUse == "" {
			nameToUse = prod.ProductName
		}
		if !isIndonesianBrand(prod.Brands, nameToUse) {
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

		// Download gambar ke folder uploads lokal
		localImageURL := ""
		if prod.ImageURL != "" {
			downloadedURL, err := downloadAndSaveImage(prod.ImageURL, prod.Barcode)
			if err == nil {
				localImageURL = downloadedURL
			}
		}

		// Simpan ke Redis DB 1
		draft := DraftProduct{
			Barcode:     prod.Barcode,
			RawName:     nameToUse,
			Brand:       prod.Brands,
			RawCategory: prod.Categories,
			ImageURL:    localImageURL,
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

	// Normalize photo URL to /uploads/ format (downloads external URLs)
	normalizedPhotoUrl := normalizePhotoURL(draft.ImageURL, req.Barcode)
	if normalizedPhotoUrl == "" {
		log.Printf("Warning: Failed to normalize photo URL %s for OFF draft %s\n", draft.ImageURL, req.Barcode)
	}

	arg := db.CreateMasterProductParams{
		Barcode:            req.Barcode,
		Name:               req.Name,
		PhotoUrl:           pgtype.Text{String: normalizedPhotoUrl, Valid: normalizedPhotoUrl != ""},
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

// RejectDraft - Admin skip/reject draft, hapus dari Redis dan delete foto
func (h *OFFHandler) RejectDraft(c *gin.Context) {
	barcode := c.Param("barcode")
	if barcode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Barcode is required"})
		return
	}

	ctx := c.Request.Context()

	// Ambil draft dulu untuk mendapatkan image URL sebelum dihapus
	draftJSON, err := h.draftRdb.HGet(ctx, DraftHashKey, barcode).Result()
	var draft DraftProduct
	if err == nil {
		json.Unmarshal([]byte(draftJSON), &draft)
	}

	// Hapus foto dari folder uploads jika ada
	if draft.ImageURL != "" && strings.HasPrefix(draft.ImageURL, "/uploads/") {
		// Konversi URL ke file path lokal
		filename := strings.TrimPrefix(draft.ImageURL, "/uploads/")
		filePath := filepath.Join("uploads", filename)
		if err := os.Remove(filePath); err != nil {
			// Log tapi jangan fail jika delete foto gagal
			fmt.Printf("Warning: Failed to delete image file %s: %v\n", filePath, err)
		}
	}

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
		"message": "Draft rejected, removed from Redis, and image deleted",
		"barcode": barcode,
	})
}
