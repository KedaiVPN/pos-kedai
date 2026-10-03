package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"kasir-api-go/db"
)

type ReportHandler struct {
	q *db.Queries
}

func NewReportHandler(q *db.Queries) *ReportHandler {
	return &ReportHandler{q: q}
}

func (h *ReportHandler) SubmitReport(c *gin.Context) {
	storeIDStr := c.GetString("store_id")
	userIDStr := c.GetString("user_id")

	storeID, err := uuid.Parse(storeIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid store context"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
		return
	}

	pgStoreID := pgtype.UUID{Bytes: storeID, Valid: true}
	pgUserID := pgtype.UUID{Bytes: userID, Valid: true}

	user, err := h.q.GetUser(c.Request.Context(), pgUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// 1. Fetch unreported transactions from database directly
	unreported, err := h.q.GetUnreportedTransactionsByCashier(c.Request.Context(), db.GetUnreportedTransactionsByCashierParams{
		StoreID:   pgStoreID,
		CashierID: pgUserID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check transactions"})
		return
	}

	// 2. Check if there are any unreported transactions
	if len(unreported) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "NO_UNREPORTED_TRANSACTIONS"})
		return
	}

	// 3. Calculate totals from server data
	var totalRevenue int64 = 0
	var totalProfit int64 = 0

	startTime := unreported[0].TransactionTime
	endTime := unreported[len(unreported)-1].TransactionTime

	for _, tx := range unreported {
		totalRevenue += tx.TotalAmount

		items, err := h.q.GetTransactionItemsByTransactionId(c.Request.Context(), tx.ID)
		if err == nil {
			for _, item := range items {
				totalProfit += (item.Subtotal - (item.BuyPrice * int64(item.Quantity)))
			}
		}
	}

	// 4. Create the report
	report, err := h.q.CreateCashierReport(c.Request.Context(), db.CreateCashierReportParams{
		StoreID:           pgStoreID,
		CashierID:         pgUserID,
		CashierName:       user.FullName,
		StartTime:         pgtype.Timestamptz{Time: startTime.Time, Valid: startTime.Valid},
		EndTime:           pgtype.Timestamptz{Time: endTime.Time, Valid: endTime.Valid},
		TotalTransactions: int32(len(unreported)),
		TotalRevenue:      totalRevenue,
		TotalProfit:       totalProfit,
	})
	if err != nil {
		log.Printf("Error creating report: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create report"})
		return
	}

	// 5. Mark transactions as reported
	err = h.q.MarkTransactionsAsReported(c.Request.Context(), db.MarkTransactionsAsReportedParams{
		StoreID:   pgStoreID,
		CashierID: pgUserID,
	})
	if err != nil {
		log.Printf("Error marking transactions: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Report submitted successfully",
		"report":  report,
	})
}

func (h *ReportHandler) GetStoreReports(c *gin.Context) {
	storeIDStr := c.GetString("store_id")
	storeID, err := uuid.Parse(storeIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid store context"})
		return
	}

	pgStoreID := pgtype.UUID{Bytes: storeID, Valid: true}

	reports, err := h.q.GetCashierReportsByStore(c.Request.Context(), pgStoreID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reports"})
		return
	}

	if reports == nil {
		reports = []db.CashierReport{}
	}

	c.JSON(http.StatusOK, gin.H{
		"reports": reports,
	})
}

func (h *ReportHandler) DeleteReport(c *gin.Context) {
	storeIDStr := c.GetString("store_id")
	userRole := c.GetString("role")
	reportIDStr := c.Param("id")

	if userRole != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only owners can delete reports"})
		return
	}

	storeID, err := uuid.Parse(storeIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid store context"})
		return
	}

	reportID, err := uuid.Parse(reportIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid report ID"})
		return
	}

	pgStoreID := pgtype.UUID{Bytes: storeID, Valid: true}
	pgReportID := pgtype.UUID{Bytes: reportID, Valid: true}

	_, err = h.q.GetCashierReportById(c.Request.Context(), db.GetCashierReportByIdParams{
		ID:      pgReportID,
		StoreID: pgStoreID,
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
		return
	}

	err = h.q.DeleteCashierReport(c.Request.Context(), db.DeleteCashierReportParams{
		ID:      pgReportID,
		StoreID: pgStoreID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete report"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Report deleted successfully",
	})
}
