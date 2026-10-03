package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type StockReportResponse struct {
	ID                         string `json:"id"`
	Name                       string `json:"name"`
	Category                   string `json:"category"`
	Stock                      int32  `json:"stock"`
	MinStock                   int32  `json:"min_stock"`
	Status                     string `json:"status"` // "Aman", "Hampir Habis", "Habis"
	IsStockNotificationEnabled bool   `json:"is_stock_notification_enabled"`
}

func (h *ProductHandler) GetStockReport(c *gin.Context) {
	storeIDParam, exists := c.Get("store_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var parsedStoreID pgtype.UUID
	err := parsedStoreID.Scan(storeIDParam.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid store ID format"})
		return
	}

	products, err := h.queries.ListStoreProductsByStore(context.Background(), parsedStoreID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch stock report"})
		return
	}

	var response []StockReportResponse
	for _, p := range products {
		// Hanya tampilkan produk yang notifikasi stok-nya aktif DAN stoknya sudah mencapai batas minimal (atau habis)
		if !p.IsStockNotificationEnabled.Bool || p.Stock > p.MinStock || p.Stock == -1 {
			continue
		}

		status := "Aman"
		if p.Stock == 0 {
			status = "Habis"
		} else if p.Stock <= p.MinStock {
			status = "Hampir Habis"
		}

		name := p.LocalName.String
		if name == "" {
			name = "Produk" // fallback
		}

		category := p.LocalCategory.String
		if category == "" {
			category = p.CategoryName.String
		}

		idStr := ""
		if p.ID.Valid {
			idStr = fmt.Sprintf("%x-%x-%x-%x-%x", p.ID.Bytes[0:4], p.ID.Bytes[4:6], p.ID.Bytes[6:8], p.ID.Bytes[8:10], p.ID.Bytes[10:16])
		}

		response = append(response, StockReportResponse{
			ID:                         idStr,
			Name:                       name,
			Category:                   category,
			Stock:                      p.Stock,
			MinStock:                   p.MinStock,
			Status:                     status,
			IsStockNotificationEnabled: p.IsStockNotificationEnabled.Bool,
		})
	}

	if response == nil {
		response = []StockReportResponse{}
	}

	c.JSON(http.StatusOK, response)
}
