package handlers

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ValidateImageFile memeriksa ukuran, ekstensi (whitelist), dan sniff MIME
// dari 512 byte pertama. Mengembalikan pesan error berbahasa Indonesia bila
// file tidak lolos validasi, atau nil bila file aman.
//
// Catatan: sniff MIME ini bukan pertahanan absolut, tapi efektif mencegah
// upload file non-gambar (skrip, executable, dll) lewat form image.
func ValidateImageFile(file *multipart.FileHeader) error {
	if file == nil {
		return fmt.Errorf("File tidak ditemukan")
	}

	if file.Size <= 0 {
		return fmt.Errorf("File kosong")
	}

	if file.Size > MaxUploadSize {
		return fmt.Errorf("File terlalu besar, maksimal %d MB", MaxUploadSize/(1024*1024))
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !AllowedImageExt[ext] {
		return fmt.Errorf("Tipe file tidak diizinkan. Gunakan: png, jpg, jpeg, webp")
	}

	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("Gagal membaca file")
	}
	defer src.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(src, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return fmt.Errorf("Gagal membaca file")
	}

	mime := http.DetectContentType(head[:n])
	if !strings.HasPrefix(mime, "image/") {
		return fmt.Errorf("File bukan gambar yang valid")
	}

	return nil
}

func UploadImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to upload image: " + err.Error()})
		return
	}

	// Validasi ukuran, ekstensi, dan MIME sebelum menyimpan apa pun ke disk.
	if err := ValidateImageFile(file); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Normalisasi ekstensi ke huruf kecil supaya tidak ada file .PNG / .JPG
	// yang lolos dengan casing aneh lalu dieksekusi sebagai skrip.
	extension := strings.ToLower(filepath.Ext(file.Filename))
	
	// Cek apakah ada barcode yang disertakan dalam form-data atau query param
	barcode := strings.TrimSpace(c.PostForm("barcode"))
	if barcode == "" {
		barcode = strings.TrimSpace(c.Query("barcode"))
	}

	var filename string
	if barcode != "" {
		// Bersihkan karakter non-alphanumeric pada barcode agar aman digunakan sebagai nama file
		reg := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
		cleanBarcode := reg.ReplaceAllString(barcode, "")
		if cleanBarcode != "" {
			filename = fmt.Sprintf("%s_%d%s", cleanBarcode, time.Now().UnixNano(), extension)
		} else {
			filename = fmt.Sprintf("%d%s", time.Now().UnixNano(), extension)
		}
	} else {
		filename = fmt.Sprintf("%d%s", time.Now().UnixNano(), extension)
	}

	// Ensure uploads directory exists
	if err := os.MkdirAll("uploads", 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	uploadPath := filepath.Join("uploads", filename)

	if err := c.SaveUploadedFile(file, uploadPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
		return
	}

	// Return the relative URL to access the image
	imageURL := fmt.Sprintf("/uploads/%s", filename)
	c.JSON(http.StatusOK, gin.H{"image_url": imageURL})
}
