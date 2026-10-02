package handlers

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kasir-api-go/api"
	"kasir-api-go/db"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminHandler struct {
	queries  *db.Queries
	pool     *pgxpool.Pool
	draftRdb *redis.Client
}

func NewAdminHandler(queries *db.Queries, pool *pgxpool.Pool, draftRdb *redis.Client) *AdminHandler {
	return &AdminHandler{queries: queries, pool: pool, draftRdb: draftRdb}
}

type UpdateProRequest struct {
	Days int `json:"days"`
}

func (h *AdminHandler) GetDashboardStats(c *gin.Context) {
	approvedCount, err := h.queries.CountMasterProducts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count approved products"})
		return
	}

	pendingCount, err := h.queries.CountPendingProducts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count pending products"})
		return
	}

	totalStores, err := h.queries.CountTotalStores(c.Request.Context())
	if err != nil {
		totalStores = 0
	}

	proStores, err := h.queries.CountProStores(c.Request.Context())
	if err != nil {
		proStores = 0
	}

	nonProStores, err := h.queries.CountNonProStores(c.Request.Context())
	if err != nil {
		nonProStores = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"approved_count": approvedCount,
		"pending_count":  pendingCount,
		"total_stores":   totalStores,
		"pro_stores":     proStores,
		"non_pro_stores": nonProStores,
	})
}

func (h *AdminHandler) ListStores(c *gin.Context) {
	stores, err := h.queries.ListAdminStores(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch stores"})
		return
	}

	if stores == nil {
		stores = []db.AdminStoreListItem{}
	}

	c.JSON(http.StatusOK, stores)
}

func (h *AdminHandler) GetStoreDetail(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid store ID"})
		return
	}

	detail, err := h.queries.GetAdminStoreDetail(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Store not found"})
		return
	}

	c.JSON(http.StatusOK, detail)
}

func (h *AdminHandler) UpdateStoreProStatus(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid store ID"})
		return
	}

	var req UpdateProRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	storeUUID := pgtype.UUID{Bytes: id, Valid: true}
	storeStatus, err := h.queries.GetStoreProStatus(c.Request.Context(), storeUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Store not found"})
		return
	}

	now := time.Now()
	var newExpiry time.Time

	if req.Days > 0 {
		duration := time.Duration(req.Days) * 24 * time.Hour
		if storeStatus.ProExpiresAt.Valid && storeStatus.ProExpiresAt.Time.After(now) {
			newExpiry = storeStatus.ProExpiresAt.Time.Add(duration)
		} else {
			newExpiry = now.Add(duration)
		}
	} else {
		newExpiry = now
	}

	updatedStore, err := h.queries.UpdateStoreProExpiry(c.Request.Context(), db.UpdateStoreProExpiryParams{
		ProExpiresAt: pgtype.Timestamptz{Time: newExpiry, Valid: true},
		ID:           storeUUID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update store Pro status"})
		return
	}

	if req.Days > 0 {
		NotifyProUpgrade(c.Request.Context(), h.queries, storeUUID)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Pro status updated successfully",
		"store_id":       updatedStore.ID,
		"pro_expires_at": updatedStore.ProExpiresAt,
	})
}

func (h *AdminHandler) ApproveProduct(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	pendingProduct, err := h.queries.GetPendingProduct(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pending product not found"})
		return
	}

	// Validate & normalize barcode: generate if empty
	barcode := strings.TrimSpace(pendingProduct.Barcode.String)
	isGeneratedBarcode := false
	if barcode == "" {
		barcode = fmt.Sprintf("GEN%d%04d", time.Now().Unix(), rand.Intn(10000))
		isGeneratedBarcode = true
		log.Printf("Generated barcode %v for pending product %v (was empty)\n", barcode, pendingProduct.ID)
	}

	// Get user ID from token
	userIDStr, exists := c.Get("user_id")
	var userIDBytes [16]byte
	if exists {
		if idStr, ok := userIDStr.(string); ok {
			parsedUUID, err := uuid.Parse(idStr)
			if err == nil {
				userIDBytes = parsedUUID
			}
		} else if idBytes, ok := userIDStr.([]interface{}); ok {
			if len(idBytes) == 16 {
				for i, v := range idBytes {
					if floatVal, ok := v.(float64); ok {
						userIDBytes[i] = byte(floatVal)
					}
				}
			}
		}
	}

	// Check if product with this barcode already exists
	existingProduct, errCheck := h.queries.GetMasterProductByBarcode(c.Request.Context(), barcode)
	var masterProduct db.MasterProduct

	if errCheck == nil && existingProduct.ID.Valid {
		// Product barcode already exists - reuse it instead of creating duplicate
		masterProduct = existingProduct
		log.Printf("Barcode %v already exists in master_products (ID: %v). Reusing instead of creating duplicate.\n", barcode, existingProduct.ID)
	} else {
		// Product doesn't exist - create new master product
		
		// Process category
		var categoryID pgtype.UUID
		categoryName := pendingProduct.Category
		if categoryName != "" {
			category, err := h.queries.GetCategoryByName(c.Request.Context(), categoryName)
			if err != nil {
				// Category doesn't exist, create it
				slug := strings.ToLower(strings.ReplaceAll(categoryName, " ", "-"))
				newCategory, err := h.queries.CreateCategory(c.Request.Context(), db.CreateCategoryParams{
					Name: categoryName,
					Slug: slug,
				})
				if err == nil {
					categoryID = newCategory.ID
				} else {
					log.Printf("Failed to create category '%v': %v. Continuing without category.\n", categoryName, err)
					categoryID = pgtype.UUID{Valid: false}
				}
			} else {
				categoryID = category.ID
			}
		} else {
			categoryID = pgtype.UUID{Valid: false}
		}

		// Ensure photo_url is set
		photoUrl := pendingProduct.ImageUrl
		if !photoUrl.Valid || photoUrl.String == "" {
			log.Printf("Warning: Pending product %v has no photo_url. This should not happen with client-side validation.\n", pendingProduct.ID)
			photoUrl = pgtype.Text{String: "", Valid: false}
		}

		// Create master product
		arg := db.CreateMasterProductParams{
			Barcode:            barcode,
			Name:               pendingProduct.Name,
			PhotoUrl:           photoUrl,
			PhotoPath:          pgtype.Text{Valid: false},
			CategoryID:         categoryID,
			BrandID:            pgtype.UUID{Valid: false},
			Unit:               pgtype.Text{String: "pcs", Valid: true},
			Source:             pgtype.Text{String: "store_request", Valid: true},
			IsGeneratedBarcode: pgtype.Bool{Bool: isGeneratedBarcode, Valid: true},
			CreatedBy:          pgtype.UUID{Bytes: userIDBytes, Valid: true},
		}

		masterProduct, err = h.queries.CreateMasterProduct(c.Request.Context(), arg)
		if err != nil {
			log.Printf("Error creating master product for barcode %v, name %v: %v\n", barcode, pendingProduct.Name, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create master product: %v", err)})
			return
		}
	}

	// Add product to store_products for the store that requested it
	if pendingProduct.StoreID.Valid {
		storeProductArg := db.CreateStoreProductParams{
			StoreID:         pendingProduct.StoreID,
			MasterProductID: masterProduct.ID,
			BuyPrice:        pendingProduct.BuyPrice,
			SellPrice:       pendingProduct.SellPrice,
			Stock:           pendingProduct.Stock,
			MinStock:        0,
			LocalName:       pgtype.Text{String: pendingProduct.Name, Valid: true},
			LocalCategory:   pgtype.Text{String: pendingProduct.Category, Valid: true},
		}

		_, err = h.queries.CreateStoreProduct(c.Request.Context(), storeProductArg)
		if err != nil {
			log.Printf("Failed to add product %v to store %v: %v\n", masterProduct.ID, pendingProduct.StoreID, err)
			// Continue - master product is already created
		}
	}

	// Delete from pending
	err = h.queries.DeletePendingProduct(c.Request.Context(), pendingProduct.ID)
	if err != nil {
		log.Printf("Failed to delete pending product %v after approval: %v\n", pendingProduct.ID, err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Product approved successfully",
		"product": masterProduct,
		"barcode_generated": isGeneratedBarcode,
		"barcode": barcode,
	})
}

func (h *AdminHandler) RejectProduct(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	pendingProduct, err := h.queries.GetPendingProduct(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pending product not found"})
		return
	}

	// Delete image file if exists and is local
	if pendingProduct.ImageUrl.Valid {
		url := pendingProduct.ImageUrl.String
		if strings.HasPrefix(url, "/uploads/") {
			filePath := "." + url
			err := os.Remove(filePath)
			if err != nil {
				log.Printf("Failed to delete image file %s: %v\n", filePath, err)
			}
		}
	}

	err = h.queries.DeletePendingProduct(c.Request.Context(), pendingProduct.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete pending product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product rejected and deleted successfully"})
}

// DeleteStore handles the admin action to permanently delete a store and all its cascade dependencies
// DeleteStore handles the admin action to permanently delete a store and all its cascade dependencies
func (h *AdminHandler) DeleteStore(c *gin.Context) {
	idParam := c.Param("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid store ID: " + err.Error()})
		return
	}
	storeUUID := pgtype.UUID{Bytes: id, Valid: true}

	ctx := c.Request.Context()

	if h.pool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection pool is nil"})
		return
	}

	// Begin transaction
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
		return
	}
	defer tx.Rollback(ctx)

	q := h.queries.WithTx(tx)

	// 1) Fetch store + collect logo file
	store, err := q.GetStore(ctx, storeUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Store not found: " + err.Error()})
		return
	}

	var filesToDelete []string
	if store.LogoUrl.Valid && strings.HasPrefix(store.LogoUrl.String, "/uploads/") {
		filesToDelete = append(filesToDelete, "."+store.LogoUrl.String)
	}

	// 2) Collect user/owner files BEFORE deleting users
	var userIDsToDelete []pgtype.UUID
	var fcmTokensToSend []string
	if store.OwnerID.Valid {
		owner, err := q.GetUser(ctx, store.OwnerID)
		if err == nil {
			if owner.PhotoUrl.Valid && strings.HasPrefix(owner.PhotoUrl.String, "/uploads/") {
				filesToDelete = append(filesToDelete, "."+owner.PhotoUrl.String)
			}
			userIDsToDelete = append(userIDsToDelete, store.OwnerID)
			if owner.FcmToken.Valid && owner.FcmToken.String != "" {
				fcmTokensToSend = append(fcmTokensToSend, owner.FcmToken.String)
			}
		} else {
		}
	}

	cashiers, err := q.ListUsersByStore(ctx, storeUUID)
	if err != nil {
	} else {
		for _, u := range cashiers {
			if u.PhotoUrl.Valid && strings.HasPrefix(u.PhotoUrl.String, "/uploads/") {
				filesToDelete = append(filesToDelete, "."+u.PhotoUrl.String)
			}
			userIDsToDelete = append(userIDsToDelete, u.ID)
			if u.FcmToken.Valid && u.FcmToken.String != "" {
				fcmTokensToSend = append(fcmTokensToSend, u.FcmToken.String)
			}
		}
	}

	// 3) Collect pending product images for this store
	pending, err := q.ListPendingProductsByStore(ctx, storeUUID)
	if err != nil {
	} else {
		for _, p := range pending {
			if p.ImageUrl.Valid && strings.HasPrefix(p.ImageUrl.String, "/uploads/") {
				filesToDelete = append(filesToDelete, "."+p.ImageUrl.String)
			}
		}
	}

	// 4) CASCADE DELETE inside transaction — child rows first, store last
	execStep := func(stepName string, query string, args ...interface{}) error {
		_, execErr := tx.Exec(ctx, query, args...)
		return execErr
	}

	cascadeQueries := []struct {
		name  string
		query string
		args  []interface{}
	}{
		{"subscription_transactions", "DELETE FROM subscription_transactions WHERE store_id = $1", []interface{}{storeUUID}},
		{"cashier_reports", "DELETE FROM cashier_reports WHERE store_id = $1", []interface{}{storeUUID}},
		{"transaction_items", "DELETE FROM transaction_items WHERE transaction_id IN (SELECT id FROM transactions WHERE store_id = $1)", []interface{}{storeUUID}},
		{"transactions", "DELETE FROM transactions WHERE store_id = $1", []interface{}{storeUUID}},
		{"stock_movements", "DELETE FROM stock_movements WHERE store_id = $1", []interface{}{storeUUID}},
		{"pending_products_by_store_id", "DELETE FROM pending_products WHERE store_id = $1", []interface{}{storeUUID}},
		{"pending_products_by_image_path", "DELETE FROM pending_products WHERE image_url LIKE '/uploads/' || $1 || '/%'", []interface{}{idParam}},
		{"store_products", "DELETE FROM store_products WHERE store_id = $1", []interface{}{storeUUID}},
	}

	for _, cq := range cascadeQueries {
		if err := execStep(cq.name, cq.query, cq.args...); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed during cascade delete step '" + cq.name + "': " + err.Error(),
			})
			return
		}
	}

	// 5) Delete the store itself
	if err := q.DeleteStore(ctx, storeUUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete store row: " + err.Error()})
		return
	}

	// 6) NOW delete users (owner + cashiers)
	for _, uid := range userIDsToDelete {
		if _, uErr := tx.Exec(ctx, "DELETE FROM users WHERE id = $1", uid); uErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user: " + uErr.Error()})
			return
		}
	}

	// 7) Commit transaction
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed: " + err.Error()})
		return
	}

	// 8) Kirim FCM force-logout push ke owner & kasir BEFORE deleting user files
	go func(tokens []string, storeName string) {
		// FCM data-only payload: include title/body inside data map so Android onMessageReceived always fires
		data := map[string]string{
			"type":       "force_logout",
			"store_name": storeName,
			"title":      "Toko Dihapus",
			"body":       fmt.Sprintf("Toko %s telah dihapus oleh Admin. Sesi login Anda berakhir.", storeName),
		}
		for _, token := range tokens {
			if token != "" {
				_ = api.SendPushDataOnly(token, data)
			}
		}
	}(fcmTokensToSend, store.StoreName)

	// 9) Delete physical files/folders
	for _, f := range filesToDelete {
		if err := os.Remove(f); err != nil {
		} else {
		}
	}
	uploadsDir := filepath.Join(".", "uploads", idParam)
	if err := os.RemoveAll(uploadsDir); err != nil {
	} else {
	}

	log.Printf("[DELETE_STORE] === SUCCESS: Store %s and all associated data deleted ===", idParam)
	c.JSON(http.StatusOK, gin.H{"message": "Store and all related data deleted successfully"})
}

type ToggleStoreBlockRequest struct {
	IsBlocked bool `json:"is_blocked"`
}

func (h *AdminHandler) ToggleStoreBlock(c *gin.Context) {
	idParam := c.Param("id")
	storeUUID, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid store ID format"})
		return
	}

	var req ToggleStoreBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	ctx := c.Request.Context()
	q := h.queries

	// Get store details
	store, err := q.GetStore(ctx, pgtype.UUID{Bytes: storeUUID, Valid: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Store not found: " + err.Error()})
		return
	}

	// Update block status
	err = q.UpdateStoreBlockStatus(ctx, db.UpdateStoreBlockStatusParams{
		ID:        pgtype.UUID{Bytes: storeUUID, Valid: true},
		IsBlocked: req.IsBlocked,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update block status: " + err.Error()})
		return
	}

	// Fetch users to send FCM
	users, err := q.ListUsersByStore(ctx, pgtype.UUID{Bytes: storeUUID, Valid: true})
	var fcmTokens []string
	if err == nil {
		for _, u := range users {
			if u.FcmToken.Valid && u.FcmToken.String != "" {
				fcmTokens = append(fcmTokens, u.FcmToken.String)
			}
		}
	}

	// Send FCM
	go func(tokens []string, isBlocked bool, storeName string) {
		var fcmType, title, body string
		if isBlocked {
			fcmType = "store_blocked"
			title = "Toko Diblokir"
			body = fmt.Sprintf("Toko %s telah dikunci oleh Admin. Anda akan dikeluarkan.", storeName)
		} else {
			fcmType = "store_unblocked"
			title = "Toko Dibuka"
			body = fmt.Sprintf("Kunci toko %s telah dibuka. Anda dapat masuk kembali.", storeName)
		}

		data := map[string]string{
			"type":       fcmType,
			"store_name": storeName,
			"title":      title,
			"body":       body,
		}
		for _, token := range tokens {
			if token != "" {
				_ = api.SendPushDataOnly(token, data)
			}
		}
	}(fcmTokens, req.IsBlocked, store.StoreName)

	statusStr := "unblocked"
	if req.IsBlocked {
		statusStr = "blocked"
	}
	log.Printf("[STORE_BLOCK] === Store %s %s successfully ===", idParam, statusStr)
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Store %s successfully", statusStr)})
}

type ToggleMasterProductRequest struct {
	IsEnabled bool `json:"is_enabled"`
}

// ToggleMasterProduct mengaktifkan / menonaktifkan halaman Master Produk secara global.
func (h *AdminHandler) ToggleMasterProduct(c *gin.Context) {
	var req ToggleMasterProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	valStr := "false"
	if req.IsEnabled {
		valStr = "true"
	}

	err := h.queries.UpdateGlobalSetting(c.Request.Context(), db.UpdateGlobalSettingParams{
		Key:   "is_master_product_enabled",
		Value: valStr,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update master product setting: " + err.Error()})
		return
	}

	log.Printf("[MASTER_PRODUCT] === Master product page enabled=%v ===", req.IsEnabled)
	c.JSON(http.StatusOK, gin.H{
		"message":    "Master product setting updated successfully",
		"is_enabled": req.IsEnabled,
	})
}

// GetMasterProductStatus mengembalikan status global halaman Master Produk.
func (h *AdminHandler) GetMasterProductStatus(c *gin.Context) {
	val, err := h.queries.GetGlobalSetting(c.Request.Context(), "is_master_product_enabled")
	if err != nil {
		// Default aktif jika belum ada setting.
		c.JSON(http.StatusOK, gin.H{"is_enabled": true})
		return
	}

	isEnabled := val != "false"
	c.JSON(http.StatusOK, gin.H{"is_enabled": isEnabled})
}

// ListRecentUploads returns a list of recently uploaded image filenames/URLs in uploads directory
func (h *AdminHandler) ListRecentUploads(c *gin.Context) {
	// Candidate directories to search
	candidates := []string{
		"./uploads",
		"uploads",
		"/kasir-api-go/uploads",
		"/www/wwwroot/kasir/uploads",
	}

	var uploadsDir string
	for _, dir := range candidates {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			uploadsDir = dir
			break
		}
	}

	if uploadsDir == "" {
		log.Printf("[DEBUG-UPLOADS] ERROR: Tidak ada direktori uploads yang ditemukan dari candidates: %v", candidates)
		c.JSON(http.StatusOK, gin.H{
			"files": []map[string]interface{}{},
			"debug": gin.H{
				"error":      "uploads directory not found",
				"candidates": candidates,
			},
		})
		return
	}

	log.Printf("[DEBUG-UPLOADS] ListRecentUploads menggunakan uploadsDir: %s", uploadsDir)

	type UploadFileInfo struct {
		Name    string    `json:"name"`
		URL     string    `json:"url"`
		ModTime time.Time `json:"mod_time"`
	}

	var imageFiles []UploadFileInfo
	var totalScanned int
	var walkDir func(currentDir string, relPrefix string)
	walkDir = func(currentDir string, relPrefix string) {
		entries, err := os.ReadDir(currentDir)
		if err != nil {
			log.Printf("[DEBUG-UPLOADS] ERROR: os.ReadDir gagal pada %s: %v", currentDir, err)
			return
		}
		for _, entry := range entries {
			totalScanned++
			entryPath := filepath.Join(currentDir, entry.Name())
			relPath := filepath.Join(relPrefix, entry.Name())

			if entry.IsDir() {
				walkDir(entryPath, relPath)
				continue
			}

			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
				info, err := entry.Info()
				if err == nil {
					cleanURL := filepath.ToSlash(relPath)
					if !strings.HasPrefix(cleanURL, "/") {
						cleanURL = "/" + cleanURL
					}
					imageFiles = append(imageFiles, UploadFileInfo{
						Name:    entry.Name(),
						URL:     cleanURL,
						ModTime: info.ModTime(),
					})
				}
			} else {
				log.Printf("[DEBUG-UPLOADS] Skip non-image: %s (ext: %s)", entry.Name(), ext)
			}
		}
	}

	walkDir(uploadsDir, "uploads")
	log.Printf("[DEBUG-UPLOADS] Scan selesai. Total file/folder di-scan: %d, Total gambar valid: %d", totalScanned, len(imageFiles))

	// Sort newest first
	sort.Slice(imageFiles, func(i, j int) bool {
		return imageFiles[i].ModTime.After(imageFiles[j].ModTime)
	})

	// Limit to top 50 files
	if len(imageFiles) > 50 {
		imageFiles = imageFiles[:50]
	}

	c.JSON(http.StatusOK, gin.H{
		"files": imageFiles,
		"debug": gin.H{
			"target_dir":    uploadsDir,
			"total_scanned": totalScanned,
			"total_images":  len(imageFiles),
		},
	})
}
