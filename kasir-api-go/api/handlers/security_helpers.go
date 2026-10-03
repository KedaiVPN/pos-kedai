package handlers

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// GetJWTSecret mengembalikan secret yang dipakai untuk signing JWT.
// Jika env JWT_SECRET tidak diset atau kosong, fungsi ini akan panic supaya
// tidak pernah jatuh ke default yang mudah ditebak ("secret") di produksi.
func GetJWTSecret() string {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		panic("JWT_SECRET environment variable wajib di-set di produksi. Tidak boleh kosong.")
	}
	if s == "secret" || s == "change-me" || len(s) < 16 {
		panic("JWT_SECRET terlalu lemah atau memakai default. Gunakan minimal 32 karakter random.")
	}
	return s
}

// ValidateJWTSecret memeriksa kekuatan JWT_SECRET tanpa memicu panic.
// Dipakai di main() (fail-fast) agar server tidak jalan dengan secret lemah.
func ValidateJWTSecret() error {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		return fmt.Errorf("JWT_SECRET kosong")
	}
	if s == "secret" || s == "change-me" || len(s) < 16 {
		return fmt.Errorf("JWT_SECRET terlalu lemah (pakai default/minimal 16 karakter)")
	}
	return nil
}

// htmlCleaner adalah UGC policy bluemonday yang mengizinkan tag HTML aman
// sambil membuang script / event handler berbahaya.
var htmlCleaner = bluemonday.UGCPolicy()

// SanitizeText menghapus tag HTML/JS berbahaya dari input teks user
// sebelum disimpan ke database. Cukup untuk mencegah XSS tersimpan.
func SanitizeText(input string) string {
	return strings.TrimSpace(htmlCleaner.Sanitize(input))
}

// AllowedImageExt daftar ekstensi gambar yang boleh di-upload.
var AllowedImageExt = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

// MaxUploadSize default 3 MB – sama dengan nilai di store_handler.go.
const MaxUploadSize int64 = 3 * 1024 * 1024

// IsSafeExternalURL memeriksa apakah URL aman untuk di-request dari server (anti-SSRF).
// Menolak host localhost, private network (RFC 1918, link-local, loopback).
func IsSafeExternalURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return false
	}

	// Blokir langsung localhost dan domain internal
	lowerHost := strings.ToLower(hostname)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".local") || strings.HasSuffix(lowerHost, ".internal") {
		return false
	}

	// Resolve IP address
	ips, err := net.LookupIP(hostname)
	if err != nil || len(ips) == 0 {
		return false
	}

	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return false
		}
	}

	return true
}

// SanitizeBarcode membersihkan karakter yang tidak valid dari barcode agar aman dipakai di nama file.
func SanitizeBarcode(barcode string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
	return reg.ReplaceAllString(barcode, "")
}