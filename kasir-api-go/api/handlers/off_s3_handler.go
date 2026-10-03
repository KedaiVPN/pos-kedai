package handlers

import (
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	S3DumpURL = "https://openfoodfacts-ds.s3.eu-west-3.amazonaws.com/en.openfoodfacts.org.products.csv.gz"
)

// FetchFromS3Dump mengambil produk Indonesia dari S3 dump OFF (fallback saat API OFF down)
func (h *OFFHandler) FetchFromS3Dump(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "25")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 50 {
		pageSize = 25
	}

	// Stream dari S3 dump
	req, err := http.NewRequest("GET", S3DumpURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create S3 request"})
		return
	}
	req.Header.Set("User-Agent", OFFUserAgent)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to fetch S3 dump: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("S3 returned status %d", resp.StatusCode)})
		return
	}

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress S3 dump"})
		return
	}
	defer gzReader.Close()

	csvReader := csv.NewReader(bufio.NewReader(gzReader))
	csvReader.Comma = '\t'
	csvReader.LazyQuotes = true
	csvReader.FieldsPerRecord = -1 // Allow variable number of fields

	// Baca header
	header, err := csvReader.Read()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read CSV header"})
		return
	}

	// Map kolom
	colMap := make(map[string]int)
	for i, h := range header {
		colMap[h] = i
	}

	// Cari kolom yang dibutuhkan
	codeIdx, codeOk := colMap["code"]
	nameIdx, nameOk := colMap["product_name"]
	imageIdx, imageOk := colMap["image_url"]
	countriesIdx, countriesOk := colMap["countries_tags"]
	brandsIdx := colMap["brands"]
	categoriesIdx := colMap["categories"]

	if !codeOk || !nameOk || !imageOk || !countriesOk {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "CSV schema mismatch (missing required columns)"})
		return
	}

	// Skip sampai ke halaman yang diminta
	skipCount := (page - 1) * pageSize
	currentCount := 0
	collectedCount := 0
	var products []DraftProduct

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip baris rusak
		}

		// Ambil nilai kolom
		code := safeGet(record, codeIdx)
		countries := safeGet(record, countriesIdx)
		name := safeGet(record, nameIdx)
		img := safeGet(record, imageIdx)
		brands := safeGet(record, brandsIdx)
		categories := safeGet(record, categoriesIdx)

		// Filter: produk Indonesia (barcode 899... ATAU countries_tags contains indonesia)
		isIndonesian := strings.HasPrefix(code, "899") || strings.Contains(strings.ToLower(countries), "indonesia")
		if !isIndonesian || name == "" {
			continue
		}

		// Kalau belum sampai halaman yang diminta, skip
		if currentCount < skipCount {
			currentCount++
			continue
		}

		// Kalau sudah dapat cukup, stop
		if collectedCount >= pageSize {
			break
		}

		// Download foto (tanpa blocking lama — timeout singkat)
		localImagePath := ""
		if img != "" {
			// Langsung download & simpan — error diabaikan saja
			localImagePath, _ = downloadAndSaveImage(img, code)
		}

		// Buat draft product
		draft := DraftProduct{
			Barcode:     code,
			RawName:     name,
			Brand:       brands,
			RawCategory: categories,
			ImageURL:    localImagePath,
			FetchedAt:   time.Now().Unix(),
		}

		products = append(products, draft)
		collectedCount++
		currentCount++
	}

	// Simpan ke Redis draft
	ctx := c.Request.Context()
	for _, draft := range products {
		draftJSON, err := json.Marshal(draft)
		if err != nil {
			continue
		}
		h.draftRdb.HSet(ctx, DraftHashKey, draft.Barcode, draftJSON)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Batch fetch completed",
		"total":        len(products),
		"inserted":     len(products),
		"skipped":      0,
		"current_page": page,
		"source":       "S3 Dump (Fallback)",
	})
}

// safeGet helper untuk mengambil elemen array dengan bounds check
func safeGet(record []string, idx int) string {
	if idx < 0 || idx >= len(record) {
		return ""
	}
	return record[idx]
}
