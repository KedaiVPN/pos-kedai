package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"kasir-api-go/api"
	"kasir-api-go/db"
)

type TransactionHandler struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewTransactionHandler(queries *db.Queries, pool *pgxpool.Pool) *TransactionHandler {
	return &TransactionHandler{queries: queries, pool: pool}
}

type CreateTransactionItemRequest struct {
	StoreProductID  string `json:"store_product_id" binding:"required"`
	MasterProductID string `json:"master_product_id" binding:"required"`
	ProductName     string `json:"product_name" binding:"required"`
	Barcode         string `json:"barcode" binding:"required"`
	Quantity        int32  `json:"quantity" binding:"required"`
	BuyPrice        int64  `json:"buy_price"`
	SellPrice       int64  `json:"sell_price"`
	Subtotal        int64  `json:"subtotal"`
}

type CreateTransactionRequest struct {
	StoreID         string                         `json:"store_id" binding:"required"`
	CashierID       string                         `json:"cashier_id" binding:"required"`
	InvoiceNumber   string                         `json:"invoice_number" binding:"required"`
	TotalAmount     int64                          `json:"total_amount"`
	PaidAmount      int64                          `json:"paid_amount"`
	ChangeAmount    int64                          `json:"change_amount"`
	PaymentMethod   string                         `json:"payment_method" binding:"required"`
	SyncStatus      string                         `json:"sync_status" binding:"required"`
	DeviceID        string                         `json:"device_id"`
	TransactionTime string                         `json:"transaction_time" binding:"required"`
	Items           []CreateTransactionItemRequest `json:"items" binding:"required,min=1"`
}

func (h *TransactionHandler) CreateTransaction(c *gin.Context) {
	var req CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	storeID, err := uuid.Parse(req.StoreID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid store id"})
		return
	}

	cashierID, err := uuid.Parse(req.CashierID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cashier id"})
		return
	}

	transactionTime, err := time.Parse(time.RFC3339, req.TransactionTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction time format, use RFC3339"})
		return
	}

	// Use database transaction
	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin transaction"})
		return
	}
	defer tx.Rollback(ctx)

	qtx := h.queries.WithTx(tx)

	// Create transaction
	arg := db.CreateTransactionParams{
		StoreID:         pgtype.UUID{Bytes: storeID, Valid: true},
		CashierID:       pgtype.UUID{Bytes: cashierID, Valid: true},
		InvoiceNumber:   req.InvoiceNumber,
		TotalAmount:     req.TotalAmount,
		PaidAmount:      req.PaidAmount,
		ChangeAmount:    req.ChangeAmount,
		PaymentMethod:   req.PaymentMethod,
		TransactionTime: pgtype.Timestamp{Time: transactionTime, Valid: true},
		SyncStatus:      req.SyncStatus,
		DeviceID:        pgtype.Text{String: req.DeviceID, Valid: req.DeviceID != ""},
	}

	createdTx, err := qtx.CreateTransaction(ctx, arg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Create transaction items
	for _, item := range req.Items {
		item.ProductName = SanitizeText(item.ProductName)
		item.Barcode = SanitizeText(item.Barcode)
		spID, err := uuid.Parse(item.StoreProductID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid store product id in items"})
			return
		}
		mpID, err := uuid.Parse(item.MasterProductID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid master product id in items"})
			return
		}

		itemArg := db.CreateTransactionItemParams{
			TransactionID:   createdTx.ID,
			StoreProductID:  pgtype.UUID{Bytes: spID, Valid: true},
			MasterProductID: pgtype.UUID{Bytes: mpID, Valid: true},
			ProductName:     item.ProductName,
			Barcode:         item.Barcode,
			Quantity:        item.Quantity,
			BuyPrice:        item.BuyPrice,
			SellPrice:       item.SellPrice,
			Subtotal:        item.Subtotal,
		}

		_, err = qtx.CreateTransactionItem(ctx, itemArg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create transaction item: " + err.Error()})
			return
		}

		// Update stock and create stock movement if stock is not unlimited
		storeProduct, err := qtx.GetStoreProduct(ctx, pgtype.UUID{Bytes: spID, Valid: true})
		if err == nil && storeProduct.Stock != -1 {
			// Deduct stock
			err = qtx.UpdateStoreProductStock(ctx, db.UpdateStoreProductStockParams{
				ID:    pgtype.UUID{Bytes: spID, Valid: true},
				Stock: -item.Quantity,
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update stock: " + err.Error()})
				return
			}

			// Create stock movement
			movementArg := db.CreateStockMovementParams{
				StoreID:        pgtype.UUID{Bytes: storeID, Valid: true},
				StoreProductID: pgtype.UUID{Bytes: spID, Valid: true},
				MovementType:   "SALE",
				Quantity:       item.Quantity,
				ReferenceType:  pgtype.Text{String: "TRANSACTION", Valid: true},
				ReferenceID:    createdTx.ID,
			}
			_, err = qtx.CreateStockMovement(ctx, movementArg)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create stock movement: " + err.Error()})
				return
			}
		}
	}

	err = tx.Commit(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit transaction"})
		return
	}

	// Call notification check AFTER commit to ensure the background task reads the correct updated stock
	for _, item := range req.Items {
		spID, _ := uuid.Parse(item.StoreProductID)
		go CheckAndSendStockNotification(context.Background(), h.queries, pgtype.UUID{Bytes: spID, Valid: true})
	}

	// Send Push Notification to Owners
	go func() {
		bgCtx := context.Background()
		owners, err := h.queries.ListStoreOwners(bgCtx, pgtype.UUID{Bytes: storeID, Valid: true})
		if err == nil {
			title := "Transaksi Baru"
			body := fmt.Sprintf("Transaksi sejumlah Rp %d telah dilakukan.", createdTx.TotalAmount)

			txIDBytes, _ := uuid.FromBytes(createdTx.ID.Bytes[:])
			txIDStr := txIDBytes.String()
			data := map[string]string{
				"route":          "receipt",
				"transaction_id": txIDStr,
			}

			for _, owner := range owners {
				if owner.FcmToken.Valid && owner.FcmToken.String != "" {
					api.SendPushNotificationWithData(owner.FcmToken.String, title, body, data)
				}
			}
		}
	}()

	c.JSON(http.StatusCreated, createdTx)
}

type DashboardStatsResponse struct {
	TotalRevenue       int64            `json:"total_revenue"`
	TotalTransactions  int32            `json:"total_transactions"`
	TotalProducts      int32            `json:"total_products"`
	NetProfit          int64            `json:"net_profit"`
	RecentTransactions []db.Transaction `json:"recent_transactions"`
}

func (h *TransactionHandler) GetDashboardStats(c *gin.Context) {
	storeIDRaw, exists := c.Get("store_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "store_id not found in token"})
		return
	}

	storeID, err := uuid.Parse(storeIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid store id"})
		return
	}

	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found in token"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	role := c.GetString("role")

	ctx := c.Request.Context()

	var statsTotalRevenue int64
	var statsTotalTransactions int32
	var statsTotalProducts int32
	var statsNetProfit int64
	var recentTx []db.Transaction

	if role == "owner" {
		stats, err := h.queries.GetStoreDashboardStats(ctx, pgtype.UUID{Bytes: storeID, Valid: true})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch stats"})
			return
		}
		statsTotalRevenue = stats.TotalRevenue
		statsTotalTransactions = stats.TotalTransactions
		statsTotalProducts = stats.TotalProducts
		statsNetProfit = stats.NetProfit

		recentTx, err = h.queries.GetRecentStoreTransactions(ctx, pgtype.UUID{Bytes: storeID, Valid: true})
		if err != nil {
			recentTx = []db.Transaction{}
		}
	} else {
		stats, err := h.queries.GetStoreDashboardStatsByCashier(ctx, db.GetStoreDashboardStatsByCashierParams{
			StoreID:   pgtype.UUID{Bytes: storeID, Valid: true},
			CashierID: pgtype.UUID{Bytes: userID, Valid: true},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch stats"})
			return
		}
		statsTotalRevenue = stats.TotalRevenue
		statsTotalTransactions = stats.TotalTransactions
		statsTotalProducts = stats.TotalProducts
		statsNetProfit = stats.NetProfit

		recentTx, err = h.queries.GetRecentStoreTransactionsByCashier(ctx, db.GetRecentStoreTransactionsByCashierParams{
			StoreID:   pgtype.UUID{Bytes: storeID, Valid: true},
			CashierID: pgtype.UUID{Bytes: userID, Valid: true},
		})
		if err != nil {
			recentTx = []db.Transaction{}
		}
	}

	c.JSON(http.StatusOK, DashboardStatsResponse{
		TotalRevenue:       statsTotalRevenue,
		TotalTransactions:  statsTotalTransactions,
		TotalProducts:      statsTotalProducts,
		NetProfit:          statsNetProfit,
		RecentTransactions: recentTx,
	})
}

type TransactionResponse struct {
	ID              string                         `json:"id"`
	StoreID         string                         `json:"store_id"`
	CashierID       string                         `json:"cashier_id"`
	CashierName     string                         `json:"cashier_name"`
	InvoiceNumber   string                         `json:"invoice_number"`
	TotalAmount     int64                          `json:"total_amount"`
	PaidAmount      int64                          `json:"paid_amount"`
	ChangeAmount    int64                          `json:"change_amount"`
	PaymentMethod   string                         `json:"payment_method"`
	TransactionTime string                         `json:"transaction_time"`
	SyncStatus      string                         `json:"sync_status"`
	DeviceID        string                         `json:"device_id"`
	Items           []CreateTransactionItemRequest `json:"items"`
}

func (h *TransactionHandler) GetAllTransactions(c *gin.Context) {
	ctx := c.Request.Context()
	storeIDStr := c.GetString("store_id")

	if storeIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	storeIDBytes, err := uuid.Parse(storeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid store id"})
		return
	}

	userIDStr := c.GetString("user_id")
	userIDBytes, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	role := c.GetString("role")

	var transactions []db.Transaction
	if role == "owner" {
		transactions, err = h.queries.GetAllStoreTransactions(ctx, pgtype.UUID{Bytes: storeIDBytes, Valid: true})
	} else {
		transactions, err = h.queries.GetAllStoreTransactionsByCashier(ctx, db.GetAllStoreTransactionsByCashierParams{
			StoreID:   pgtype.UUID{Bytes: storeIDBytes, Valid: true},
			CashierID: pgtype.UUID{Bytes: userIDBytes, Valid: true},
		})
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch transactions"})
		return
	}

	response := []TransactionResponse{}
	for _, tx := range transactions {
		items, err := h.queries.GetTransactionItemsByTransactionId(ctx, tx.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch transaction items"})
			return
		}

		itemResponses := []CreateTransactionItemRequest{}
		for _, item := range items {
			storeProductID, _ := uuid.FromBytes(item.StoreProductID.Bytes[:])
			masterProductID, _ := uuid.FromBytes(item.MasterProductID.Bytes[:])

			itemResponses = append(itemResponses, CreateTransactionItemRequest{
				StoreProductID:  storeProductID.String(),
				MasterProductID: masterProductID.String(),
				ProductName:     item.ProductName,
				Barcode:         item.Barcode,
				Quantity:        item.Quantity,
				BuyPrice:        item.BuyPrice,
				SellPrice:       item.SellPrice,
				Subtotal:        item.Subtotal,
			})
		}

		txID, _ := uuid.FromBytes(tx.ID.Bytes[:])
		storeID, _ := uuid.FromBytes(tx.StoreID.Bytes[:])
		cashierID, _ := uuid.FromBytes(tx.CashierID.Bytes[:])

		cashierName := ""
		cashierUser, err := h.queries.GetUser(ctx, tx.CashierID)
		if err == nil {
			cashierName = cashierUser.FullName
		}

		response = append(response, TransactionResponse{
			ID:              txID.String(),
			StoreID:         storeID.String(),
			CashierID:       cashierID.String(),
			CashierName:     cashierName,
			InvoiceNumber:   tx.InvoiceNumber,
			TotalAmount:     tx.TotalAmount,
			PaidAmount:      tx.PaidAmount,
			ChangeAmount:    tx.ChangeAmount,
			PaymentMethod:   tx.PaymentMethod,
			TransactionTime: tx.TransactionTime.Time.Format(time.RFC3339),
			SyncStatus:      tx.SyncStatus,
			DeviceID:        tx.DeviceID.String,
			Items:           itemResponses,
		})
	}

	c.JSON(http.StatusOK, response)
}
