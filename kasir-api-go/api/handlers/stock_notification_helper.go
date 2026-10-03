package handlers

import (
	"context"
	"fmt"
	"kasir-api-go/api"
	"kasir-api-go/db"
	"log"

	"github.com/jackc/pgx/v5/pgtype"
)

func CheckAndSendStockNotification(ctx context.Context, queries *db.Queries, storeProductID pgtype.UUID) {
	// Dapatkan detail produk setelah diupdate stoknya
	storeProduct, err := queries.GetStoreProduct(ctx, storeProductID)
	if err != nil {
		log.Println("Gagal mengambil store product untuk notifikasi stok:", err)
		return
	}

	// Cek apakah notifikasi diaktifkan dan stok kurang dari atau sama dengan minimum
	if !storeProduct.IsStockNotificationEnabled.Bool || storeProduct.Stock > storeProduct.MinStock || storeProduct.Stock == -1 {
		log.Printf("Notifikasi di-skip. Status produk: AktifNotif=%v, Stok=%d, MinStok=%d\n", storeProduct.IsStockNotificationEnabled.Bool, storeProduct.Stock, storeProduct.MinStock)
		return
	}

	// Cari user (owner) dari toko ini
	users, err := queries.ListUsersByStore(ctx, storeProduct.StoreID)
	if err != nil {
		log.Println("Gagal mengambil daftar user toko:", err)
		return
	}

	log.Printf("Ditemukan %d user toko ini\n", len(users))
	for _, user := range users {
		log.Printf("Mengecek user %s, Role: %s, HasToken: %v\n", user.Email.String, user.Role, user.FcmToken.Valid && user.FcmToken.String != "")
		if user.Role == "owner" && user.FcmToken.Valid && user.FcmToken.String != "" {
			log.Printf("Token Ditemukan: %s\n", user.FcmToken.String)
			productName := storeProduct.LocalName.String
			if productName == "" {
				productName = "Produk"
			}
			title := "Peringatan Sisa Stok"
			body := fmt.Sprintf("Stok %s hampir habis. Sisa stok: %d", productName, storeProduct.Stock)

			err = api.SendPushNotification(user.FcmToken.String, title, body)
			if err != nil {
				log.Println("Gagal mengirim notifikasi stok ke owner:", err)
			}
		}
	}
}
