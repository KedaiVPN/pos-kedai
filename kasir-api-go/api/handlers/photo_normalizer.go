package handlers

import (
	"log"
	"strings"
)

// normalizePhotoURL normalizes photo URLs to relative /uploads/ paths.
//
// Rules:
// 1. If URL starts with https://api-go-v1.free-account.my.id/uploads/ -> strip domain, return /uploads/...
// 2. If URL starts with /uploads/ -> return as-is (already normalized)
// 3. If URL is external (http:// or https://) -> download to /uploads/, return /uploads/{barcode}_{timestamp}.ext
// 4. If URL is empty or invalid -> return empty string
//
// Returns: normalized path (e.g., "/uploads/123_456.jpg") or empty string on error
func normalizePhotoURL(urlOrPath, barcode string) string {
	if urlOrPath == "" {
		return ""
	}

	// Case 1: Strip our own domain prefix
	if strings.HasPrefix(urlOrPath, "https://api-go-v1.free-account.my.id/uploads/") {
		return strings.TrimPrefix(urlOrPath, "https://api-go-v1.free-account.my.id")
	}
	if strings.HasPrefix(urlOrPath, "http://api-go-v1.free-account.my.id/uploads/") {
		return strings.TrimPrefix(urlOrPath, "http://api-go-v1.free-account.my.id")
	}

	// Case 2: Already normalized
	if strings.HasPrefix(urlOrPath, "/uploads/") {
		return urlOrPath
	}

	// Case 3: External URL - download it using existing downloadAndSaveImage function
	if strings.HasPrefix(urlOrPath, "http://") || strings.HasPrefix(urlOrPath, "https://") {
		log.Printf("[PHOTO-NORMALIZE] Downloading external URL: %s", urlOrPath)
		// downloadAndSaveImage is in the same package (off_handler.go)
		localPath, err := downloadAndSaveImage(urlOrPath, barcode)
		if err != nil {
			log.Printf("[PHOTO-NORMALIZE] Failed to download %s: %v", urlOrPath, err)
			return ""
		}
		log.Printf("[PHOTO-NORMALIZE] Downloaded %s -> %s", urlOrPath, localPath)
		return localPath
	}

	// Case 4: Invalid or unsupported format
	log.Printf("[PHOTO-NORMALIZE] Unsupported photo URL format: %s", urlOrPath)
	return ""
}
