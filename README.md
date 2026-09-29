# POS KEDAI POS System

POS KEDAI adalah sistem aplikasi kasir/POS (Point of Sales) modern untuk toko kecil dan menengah yang dirancang dengan konsep **Offline-first** pada aplikasi kasir Android, dipadukan dengan **Backend API berbasis Go** berkinerja tinggi.

Proyek ini terdiri dari dua komponen utama:
1. **Aplikasi Toko & Admin (Android)**: Aplikasi kasir offline-first dengan Room SQLite + aplikasi admin pengelola katalog terpusat.
2. **Backend API (Go)**: Service sentralisasi data, otentikasi, sinkronisasi transaksi, manajemen langganan (TriPay), dan push notification (FCM).

---

## 🛠 Tech Stack

### Backend API (`kasir-api-go`)
- **Language & Runtime:** Go (golang) 1.25+
- **HTTP Framework:** Gin (`github.com/gin-gonic/gin`)
- **Database & Query:** PostgreSQL + sqlc (`github.com/jackc/pgx/v5`)
- **Caching & Rate Limiting:** Redis (`github.com/go-redis/redis/v8`)
- **Authentication:** JWT (`golang-jwt/jwt/v5`) + bcrypt
- **Realtime / Push:** Gorilla WebSockets + Firebase Cloud Messaging (FCM)
- **Payment Gateway:** TriPay Integrasi Sandbox & Production
- **Email Service:** SMTP (Gomail)

### Android Frontend (`kasir-android`)
- **Language:** Kotlin
- **UI Toolkit:** Jetpack Compose (Material 3)
- **Architecture:** Clean Architecture + MVVM (Multi-module)
- **Local Database:** Room SQLite (Offline Transaction & Sync Queue)
- **Networking:** Retrofit2 + OkHttp3 + WebSockets
- **Background Worker:** WorkManager (Sinkronisasi Otomatis Offline-to-Online)
- **Barcode Scanner:** CameraX + Google ML Kit Barcode Scanning

---

## 📂 Struktur Proyek

Proyek ini menggunakan struktur *monorepo* yang menampung API server Go dan aplikasi Android multi-module.

```text
.
├── kasir-api-go/               # Backend API Server (Go)
│   ├── api/                    # Route handlers, middleware, WebSocket, FCM
│   ├── db/                     # Migrasi SQL & query hasil generate sqlc
│   ├── utils/                  # Helper Email, Redis, TriPay
│   ├── main.go                 # Entry point server
│   └── .env.example            # Template konfigurasi environment
│
├── kasir-android/              # Aplikasi Android (Multi-module)
│   ├── core/                   # Shared module (Retrofit Client, TokenManager, API Models)
│   ├── app-store/              # Aplikasi Kasir Toko (Offline-first, POS, Sync)
│   └── app-admin/              # Aplikasi Admin (Manajemen Katalog Pusat & User)
│
├── README.md                   # Penjelasan Proyek
├── DOCUMENTATION.md            # Panduan Build & Deployment VPS
└── nginx.conf.example          # Template Nginx Reverse Proxy
```

---

## 🚀 Fitur Utama

- **Offline-First Cashier:** Kasir tetap bisa melakukan transaksi tanpa koneksi internet. Data tersimpan di Room SQLite lokal dan otomatis di-sync ke server Go saat koneksi kembali online.
- **Katalog Terpusat & Custom Product:** Toko dapat mengunduh katalog produk master dari admin atau membuat produk khusus toko sendiri.
- **Manajemen Stok & Laporan:** Pemantauan stok real-time, laporan penjualan harian/bulanan, ekspor laporan.
- **Integrasi Pembayaran (TriPay):** Mendukung pembayaran digital/QRIS via payment gateway TriPay.
- **Push Notification & FCM:** Notifikasi stok menipis, konfirmasi pembayaran, dan pengumuman sistem via Firebase Cloud Messaging.
