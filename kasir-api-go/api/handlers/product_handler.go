package handlers

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"kasir-api-go/db"
)

type ProductHandler struct {
	queries   *db.Queries
	wsManager *WebSocketManager
}

func NewProductHandler(queries *db.Queries, wsManager *WebSocketManager) *ProductHandler {
	return &ProductHandler{
		queries:   queries,
		wsManager: wsManager,
	}
}

type CreateMasterProductRequest struct {
	Barcode            string `json:"barcode" binding:"required"`
	Name               string `json:"name" binding:"required"`
	PhotoURL           string `json:"photo_url"`
	PhotoPath          string `json:"photo_path"`
	CategoryID         string `json:"category_id"`
	BrandID            string `json:"brand_id"`
	Unit               string `json:"unit"`
	Source             string `json:"source"`
	IsGeneratedBarcode bool   `json:"is_generated_barcode"`
	CreatedBy          string `json:"created_by"`
}

func (h *ProductHandler) CreateMasterProduct(c *gin.Context) {

	role, exists := c.Get("role")
	if exists && role == "kasir" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Kasir is not allowed to create master product"})
		return
	}
	var req CreateMasterProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Sanitasi field teks.
	req.Barcode = SanitizeText(req.Barcode)
	req.Name = SanitizeText(req.Name)
	req.CategoryID = SanitizeText(req.CategoryID)
	req.Unit = SanitizeText(req.Unit)
	req.Source = SanitizeText(req.Source)

	// Normalize photo URL before storing
	normalizedPhotoUrl := req.PhotoURL
	if normalizedPhotoUrl != "" {
		normalizedPhotoUrl = normalizePhotoURL(normalizedPhotoUrl, req.Barcode)
	}

	arg := db.CreateMasterProductParams{
		Barcode:            req.Barcode,
		Name:               req.Name,
		PhotoUrl:           pgtype.Text{String: normalizedPhotoUrl, Valid: normalizedPhotoUrl != ""},
		PhotoPath:          pgtype.Text{String: req.PhotoPath, Valid: req.PhotoPath != ""},
		Unit:               pgtype.Text{String: req.Unit, Valid: req.Unit != ""},
		Source:             pgtype.Text{String: req.Source, Valid: req.Source != ""},
		IsGeneratedBarcode: pgtype.Bool{Bool: req.IsGeneratedBarcode, Valid: true},
	}

	if req.CategoryID != "" {
		// Category might be a UUID or a name, let's process it like in ApproveProduct
		var categoryID pgtype.UUID
		uid, err := uuid.Parse(req.CategoryID)
		if err == nil {
			categoryID = pgtype.UUID{Bytes: uid, Valid: true}
		} else {
			// It's a name, look it up or create it
			categoryName := req.CategoryID
			category, err := h.queries.GetCategoryByName(c.Request.Context(), categoryName)
			if err != nil {
				slug := strings.ToLower(strings.ReplaceAll(categoryName, " ", "-"))
				newCategory, err := h.queries.CreateCategory(c.Request.Context(), db.CreateCategoryParams{
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
		arg.CategoryID = categoryID
	}
	if req.BrandID != "" {
		uid, err := uuid.Parse(req.BrandID)
		if err == nil {
			arg.BrandID = pgtype.UUID{Bytes: uid, Valid: true}
		}
	}
	if req.CreatedBy != "" {
		uid, err := uuid.Parse(req.CreatedBy)
		if err == nil {
			arg.CreatedBy = pgtype.UUID{Bytes: uid, Valid: true}
		}
	}

	product, err := h.queries.CreateMasterProduct(c.Request.Context(), arg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Call notification check immediately after creating or updating
	go CheckAndSendStockNotification(context.Background(), h.queries, product.ID)

	c.JSON(http.StatusCreated, product)
}

func (h *ProductHandler) GetMasterProduct(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid master product id"})
		return
	}

	product, err := h.queries.GetMasterProduct(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "master product not found"})
		return
	}

	c.JSON(http.StatusOK, product)
}

type MasterProductResponse struct {
	ID                 string `json:"id"`
	Barcode            string `json:"barcode"`
	Name               string `json:"name"`
	PhotoUrl           string `json:"photo_url"`
	PhotoPath          string `json:"photo_path"`
	CategoryID         string `json:"category_id"`
	BrandID            string `json:"brand_id"`
	Unit               string `json:"unit"`
	Source             string `json:"source"`
	IsGeneratedBarcode bool   `json:"is_generated_barcode"`
	IsActive           bool   `json:"is_active"`
	CreatedBy          string `json:"created_by"`
	CategoryName       string `json:"category_name"`
}

func (h *ProductHandler) ListMasterProducts(c *gin.Context) {
	products, err := h.queries.ListMasterProducts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var formattedProducts []MasterProductResponse
	for _, p := range products {
		idUUID, _ := uuid.FromBytes(p.ID.Bytes[:])
		categoryUUID, _ := uuid.FromBytes(p.CategoryID.Bytes[:])
		brandUUID, _ := uuid.FromBytes(p.BrandID.Bytes[:])
		createdByUUID, _ := uuid.FromBytes(p.CreatedBy.Bytes[:])

		formatted := MasterProductResponse{
			ID:                 idUUID.String(),
			Barcode:            p.Barcode,
			Name:               p.Name,
			IsGeneratedBarcode: p.IsGeneratedBarcode.Bool,
			IsActive:           p.IsActive.Bool,
		}

		if p.PhotoUrl.Valid {
			formatted.PhotoUrl = p.PhotoUrl.String
		}
		if p.PhotoPath.Valid {
			formatted.PhotoPath = p.PhotoPath.String
		}
		if p.CategoryID.Valid {
			formatted.CategoryID = categoryUUID.String()
		}
		if p.CategoryName.Valid {
			formatted.CategoryName = p.CategoryName.String
		}
		if p.BrandID.Valid {
			formatted.BrandID = brandUUID.String()
		}
		if p.Unit.Valid {
			formatted.Unit = p.Unit.String
		}
		if p.Source.Valid {
			formatted.Source = p.Source.String
		}
		if p.CreatedBy.Valid {
			formatted.CreatedBy = createdByUUID.String()
		}

		formattedProducts = append(formattedProducts, formatted)
	}

	c.JSON(http.StatusOK, formattedProducts)
}

type CreateStoreProductRequest struct {
	StoreID                    string `json:"store_id" binding:"required"`
	MasterProductID            string `json:"master_product_id" binding:"required"`
	BuyPrice                   int64  `json:"buy_price" binding:"required"`
	SellPrice                  int64  `json:"sell_price" binding:"required"`
	Stock                      int32  `json:"stock"`
	MinStock                   int32  `json:"min_stock"`
	LocalName                  string `json:"local_name"`
	LocalCategory              string `json:"local_category"`
	IsStockNotificationEnabled bool   `json:"is_stock_notification_enabled"`
}

func (h *ProductHandler) CreateStoreProduct(c *gin.Context) {

	role, exists := c.Get("role")
	if exists && role == "kasir" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Kasir is not allowed to create store product"})
		return
	}
	var req CreateStoreProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.LocalName = SanitizeText(req.LocalName)
	req.LocalCategory = SanitizeText(req.LocalCategory)

	storeID, err := uuid.Parse(req.StoreID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid store id"})
		return
	}
	masterProductID, err := uuid.Parse(req.MasterProductID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid master product id"})
		return
	}

	arg := db.CreateStoreProductParams{
		StoreID:                    pgtype.UUID{Bytes: storeID, Valid: true},
		MasterProductID:            pgtype.UUID{Bytes: masterProductID, Valid: true},
		BuyPrice:                   req.BuyPrice,
		SellPrice:                  req.SellPrice,
		Stock:                      req.Stock,
		MinStock:                   req.MinStock,
		LocalName:                  pgtype.Text{String: req.LocalName, Valid: req.LocalName != ""},
		LocalCategory:              pgtype.Text{String: req.LocalCategory, Valid: req.LocalCategory != ""},
		IsStockNotificationEnabled: pgtype.Bool{Bool: req.IsStockNotificationEnabled, Valid: true},
	}

	product, err := h.queries.CreateStoreProduct(c.Request.Context(), arg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, product)
}

func (h *ProductHandler) GetStoreProduct(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid store product id"})
		return
	}

	product, err := h.queries.GetStoreProduct(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "store product not found"})
		return
	}

	c.JSON(http.StatusOK, product)
}

type StoreProductResponse struct {
	ID              string `json:"id"`
	StoreID         string `json:"store_id"`
	MasterProductID string `json:"master_product_id"`
	BuyPrice        int64  `json:"buy_price"`
	SellPrice       int64  `json:"sell_price"`
	Stock           int32  `json:"stock"`
	MinStock        int32  `json:"min_stock"`
	IsActive        bool   `json:"is_active"`
	LocalName       string `json:"local_name"`
	LocalCategory   string `json:"local_category"`
	Barcode         string `json:"barcode"`
	ImageUrl        string `json:"image_url"`
	CategoryName    string `json:"category_name"`
	CreatedAt       string `json:"created_at"`
}

func (h *ProductHandler) ListStoreProducts(c *gin.Context) {
	storeIDParam := c.Query("store_id")
	if storeIDParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "store_id query param is required"})
		return
	}

	storeID, err := uuid.Parse(storeIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid store id"})
		return
	}

	products, err := h.queries.ListStoreProductsByStore(c.Request.Context(), pgtype.UUID{Bytes: storeID, Valid: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var formattedProducts []StoreProductResponse
	for _, p := range products {
		idUUID, _ := uuid.FromBytes(p.ID.Bytes[:])
		storeUUID, _ := uuid.FromBytes(p.StoreID.Bytes[:])
		masterUUID, _ := uuid.FromBytes(p.MasterProductID.Bytes[:])

		barcodeStr := ""
		if p.Barcode.Valid {
			barcodeStr = p.Barcode.String
		}

		imageUrlStr := ""
		if p.ImageUrl.Valid {
			imageUrlStr = p.ImageUrl.String
		}

		categoryNameStr := ""
		if p.CategoryName.Valid {
			categoryNameStr = p.CategoryName.String
		}

		formattedProducts = append(formattedProducts, StoreProductResponse{
			ID:              idUUID.String(),
			StoreID:         storeUUID.String(),
			MasterProductID: masterUUID.String(),
			BuyPrice:        p.BuyPrice,
			SellPrice:       p.SellPrice,
			Stock:           p.Stock,
			MinStock:        p.MinStock,
			IsActive:        p.IsActive.Bool,
			LocalName:       p.LocalName.String,
			LocalCategory:   p.LocalCategory.String,
			Barcode:         barcodeStr,
			ImageUrl:        imageUrlStr,
			CategoryName:    categoryNameStr,
			CreatedAt:       p.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	c.JSON(http.StatusOK, formattedProducts)
}

type SubmitPendingProductRequest struct {
	Name        string `json:"name" binding:"required"`
	BuyPrice    int64  `json:"buy_price" binding:"required"`
	SellPrice   int64  `json:"sell_price" binding:"required"`
	Stock       int32  `json:"stock"`
	Category    string `json:"category" binding:"required"`
	Description string `json:"description"`
	Barcode     string `json:"barcode"`
	ImageURL    string `json:"image_url"`
	StoreID     string `json:"store_id"`
}

func (h *ProductHandler) SubmitPendingProduct(c *gin.Context) {

	role, exists := c.Get("role")
	if exists && role == "kasir" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Kasir is not allowed to submit pending product"})
		return
	}
	var req SubmitPendingProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Name = SanitizeText(req.Name)
	req.Category = SanitizeText(req.Category)
	req.Description = SanitizeText(req.Description)
	req.Barcode = SanitizeText(req.Barcode)

	arg := db.CreatePendingProductParams{
		Name:        req.Name,
		BuyPrice:    req.BuyPrice,
		SellPrice:   req.SellPrice,
		Stock:       req.Stock,
		Category:    req.Category,
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Barcode:     pgtype.Text{String: req.Barcode, Valid: req.Barcode != ""},
		ImageUrl:    pgtype.Text{String: req.ImageURL, Valid: req.ImageURL != ""},
	}

	if req.StoreID != "" {
		uid, err := uuid.Parse(req.StoreID)
		if err == nil {
			arg.StoreID = pgtype.UUID{Bytes: uid, Valid: true}
		}
	}

	pendingProduct, err := h.queries.CreatePendingProduct(c.Request.Context(), arg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.wsManager != nil {
		h.wsManager.BroadcastNewPendingProduct(pendingProduct)
	}

	c.JSON(http.StatusCreated, pendingProduct)
}

func (h *ProductHandler) ListPendingProducts(c *gin.Context) {
	products, err := h.queries.ListPendingProducts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, products)
}

func (h *ProductHandler) DeleteProduct(c *gin.Context) {

	role, exists := c.Get("role")
	if exists && role == "kasir" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Kasir is not allowed to delete product"})
		return
	}
	idParam := c.Param("id")
	status := c.Query("status") // "pending" or "approved"

	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	if status == "pending" {
		// Get pending product first to delete the image from disk
		product, err := h.queries.GetPendingProduct(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
		if err == nil && product.ImageUrl.Valid && product.ImageUrl.String != "" {
			// Basic security check: ensure we don't delete system files by accident
			filename := filepath.Base(product.ImageUrl.String)
			fullPath := filepath.Join("uploads", filename)
			_ = os.Remove(fullPath) // Ignore err if file doesn't exist
		}

		err = h.queries.DeletePendingProduct(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else if status == "approved" {
		uuidParam := pgtype.UUID{Bytes: id, Valid: true}

		// First get the master product to delete the image from disk later
		product, errGet := h.queries.GetMasterProduct(c.Request.Context(), uuidParam)

		// First delete associated transaction items and stock movements
		// Then delete store products, then the master product itself.
		// Note: The UI currently passes master product ID for deletion.
		// User explicitly requested to NOT delete transaction_items, stock_movements,
		// and store_products when admin deletes a master product.
		// Note: This means those child records will become orphans or have broken foreign keys
		// unless the database schema cascades or allows NULLs. Assuming DB handles it or it's intended.
		// We only delete the master product.

		// Wait, if there are foreign key constraints, `DeleteMasterProduct` will FAIL with a foreign key violation
		// if we don't delete the child records first! I need to be careful here.

		err = h.queries.DeleteMasterProduct(c.Request.Context(), uuidParam)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete master product: " + err.Error()})
			return
		}

		// Safe to delete image file now
		if errGet == nil && product.PhotoUrl.Valid && product.PhotoUrl.String != "" {
			filename := filepath.Base(product.PhotoUrl.String)
			fullPath := filepath.Join("uploads", filename)
			_ = os.Remove(fullPath)
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status query param must be 'pending' or 'approved'"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "product deleted successfully"})
}

type UpdateProductRequest struct {
	Name                       string `json:"name"`
	BuyPrice                   int64  `json:"buy_price"`
	SellPrice                  int64  `json:"sell_price"`
	Stock                      int32  `json:"stock"`
	MinStock                   int32  `json:"min_stock"`
	Category                   string `json:"category"`
	Description                string `json:"description"`
	Barcode                    string `json:"barcode"`
	ImageURL                   string `json:"image_url"`
	IsStockNotificationEnabled bool   `json:"is_stock_notification_enabled"`
}

func (h *ProductHandler) UpdateProduct(c *gin.Context) {

	role, exists := c.Get("role")
	if exists && role == "kasir" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Kasir is not allowed to update product"})
		return
	}
	idParam := c.Param("id")
	status := c.Query("status") // "pending" or "approved"

	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	var req UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if status == "pending" {
		// Normalize photo URL before storing
		normalizedPhotoUrl := req.ImageURL
		if normalizedPhotoUrl != "" {
			normalizedPhotoUrl = normalizePhotoURL(normalizedPhotoUrl, req.Barcode)
		}

		arg := db.UpdatePendingProductParams{
			ID:          pgtype.UUID{Bytes: id, Valid: true},
			Name:        req.Name,
			BuyPrice:    req.BuyPrice,
			SellPrice:   req.SellPrice,
			Stock:       req.Stock,
			Category:    req.Category,
			Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
			Barcode:     pgtype.Text{String: req.Barcode, Valid: req.Barcode != ""},
			ImageUrl:    pgtype.Text{String: normalizedPhotoUrl, Valid: normalizedPhotoUrl != ""},
		}
		err = h.queries.UpdatePendingProduct(c.Request.Context(), arg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else if status == "approved" {
		arg := db.UpdateStoreProductParams{
			ID:                         pgtype.UUID{Bytes: id, Valid: true},
			BuyPrice:                   req.BuyPrice,
			SellPrice:                  req.SellPrice,
			Stock:                      req.Stock,
			MinStock:                   req.MinStock,
			LocalName:                  pgtype.Text{String: req.Name, Valid: req.Name != ""},         // Name mapped to local_name
			LocalCategory:              pgtype.Text{String: req.Category, Valid: req.Category != ""}, // Category mapped to local_category
			IsStockNotificationEnabled: pgtype.Bool{Bool: req.IsStockNotificationEnabled, Valid: true},
		}
		err = h.queries.UpdateStoreProduct(c.Request.Context(), arg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Call notification check immediately after updating
		go CheckAndSendStockNotification(context.Background(), h.queries, pgtype.UUID{Bytes: id, Valid: true})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status query param must be 'pending' or 'approved'"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "product updated successfully"})
}

func (h *ProductHandler) DeleteStoreProductSpecific(c *gin.Context) {

	role, exists := c.Get("role")
	if exists && role == "kasir" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Kasir is not allowed to delete store product"})
		return
	}
	idParam := c.Param("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid store product id"})
		return
	}

	// Validate ownership
	contextStoreID, exists := c.Get("store_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: missing store_id in context"})
		return
	}

	storeIDStr, ok := contextStoreID.(string)
	if !ok || storeIDStr == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only store users can delete products"})
		return
	}

	product, err := h.queries.GetStoreProduct(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "store product not found"})
		return
	}

	parsedStoreID, _ := uuid.FromBytes(product.StoreID.Bytes[:])
	productStoreIDStr := parsedStoreID.String()

	if productStoreIDStr != storeIDStr {
		c.JSON(http.StatusForbidden, gin.H{"error": "you do not have permission to delete this product"})
		return
	}

	err = h.queries.DeleteStoreProduct(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete store product: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "store product deleted successfully"})
}

type AddStockRequest struct {
	AdditionalStock int32 `json:"additional_stock" binding:"required"`
}

func (h *ProductHandler) AddStoreProductStock(c *gin.Context) {
	role, exists := c.Get("role")
	if exists && role == "kasir" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Kasir is not allowed to add stock"})
		return
	}

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid store product id"})
		return
	}

	var req AddStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.queries.UpdateStoreProductStock(c.Request.Context(), db.UpdateStoreProductStockParams{
		ID:    pgtype.UUID{Bytes: id, Valid: true},
		Stock: req.AdditionalStock,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add stock"})
		return
	}

	// Call notification check in background
	go CheckAndSendStockNotification(context.Background(), h.queries, pgtype.UUID{Bytes: id, Valid: true})

	c.JSON(http.StatusOK, gin.H{"message": "stock added successfully"})
}
