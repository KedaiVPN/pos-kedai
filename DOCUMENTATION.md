# Panduan Build & Deploy (POS KEDAI)

Dokumen ini berisi panduan lengkap untuk melakukan kompilasi (*build*) aplikasi Android serta proses *deployment* Backend API Go di VPS Ubuntu/Debian.

---

## 1. Build Aplikasi Android via GitHub Actions

Aplikasi Android (`.apk`) dapat di-build otomatis di cloud menggunakan GitHub Actions tanpa membutuhkan komputer berspesifikasi tinggi.

### Langkah-langkah:
1. Pastikan URL API sudah benar pada file:
   `kasir-android/core/src/main/java/com/poskedai/core/network/RetrofitClient.kt`
   - Sesuaikan `BASE_URL` ke domain VPS Anda (contoh: `https://api.poskedai.domain.com/`).
2. Masuk ke tab **Actions** di repositori GitHub.
3. Pilih workflow **Build Android APKs**.
4. Klik tombol **Run workflow** pada branch aktif.
5. Tunggu proses build selesai (centang hijau), lalu buka detail run workflow.
6. Pada bagian **Artifacts**, unduh:
   - `app-store-debug.apk` (Aplikasi kasir untuk toko)
   - `app-admin-debug.apk` (Aplikasi admin katalog pusat)

---

## 2. Deployment Backend Go (`kasir-api-go`) di VPS

Backend Go dikompilasi menjadi satu file binary executable mandiri. Sangat ringan, hemat RAM (< 50MB saat idle), dan tidak membutuhkan runtime Java atau node_modules di VPS.

### Persyaratan Server:
- Ubuntu 22.04 / 24.04 LTS
- PostgreSQL 15+
- Redis Server
- Nginx (sebagai reverse proxy & SSL termination)
- Go 1.23+ (jika build langsung di VPS)

---

### Langkah 1: Install PostgreSQL & Redis

```bash
sudo apt update && sudo apt install -y postgresql postgresql-contrib redis-server

# Pastikan service aktif
sudo systemctl enable --now postgresql
sudo systemctl enable --now redis-server
```

Buat database dan user PostgreSQL:
```bash
sudo -i -u postgres psql
```

Di dalam psql prompt:
```sql
CREATE DATABASE kasir;
CREATE USER poskedai WITH ENCRYPTED PASSWORD 'password_rahasia_anda';
GRANT ALL PRIVILEGES ON DATABASE kasir TO poskedai;
\c kasir
GRANT ALL ON SCHEMA public TO poskedai;
\q
```

---

### Langkah 2: Build Binary Go

Ada 2 cara:

#### Opsi A: Build langsung di VPS (jika Go terpasang)
```bash
cd /root/pos-kedai/kasir-api-go
go build -o bin/server main.go
```

#### Opsi B: Cross-compile dari lokal (hemat resource VPS)
```bash
# Dari komputer lokal untuk server Linux x86_64:
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/server main.go

# Kirim binary ke VPS via SCP:
scp bin/server root@IP_VPS:/var/www/kasir-api/server
```

---

### Langkah 3: Setup Direktori & Konfigurasi `.env`

1. Buat folder aplikasi di VPS:
```bash
sudo mkdir -p /var/www/kasir-api/uploads
sudo chown -R www-data:www-data /var/www/kasir-api
```

2. Letakkan binary `server` di `/var/www/kasir-api/server` dan beri izin eksekusi:
```bash
sudo chmod +x /var/www/kasir-api/server
```

3. Buat file `/var/www/kasir-api/.env` dengan menyalin contoh dari `kasir-api-go/.env.example`:
```env
DATABASE_URL="postgres://poskedai:password_rahasia_anda@localhost:5432/kasir?sslmode=disable"
PORT=8080
JWT_SECRET="ganti_dengan_string_acak_panjang"
ADMIN_EMAIL="admin@poskedai.com"
ADMIN_PASSWORD="password_admin_pertama"

# Redis
REDIS_URL="localhost:6379"
REDIS_PASSWORD=""

# SMTP (opsional untuk verifikasi email)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=emailanda@gmail.com
SMTP_PASS=app_password_anda
SMTP_FROM="POS Kedai <emailanda@gmail.com>"

# TriPay Payment Gateway (opsional untuk langganan)
TRIPAY_MERCHANT_CODE=""
TRIPAY_API_KEY=""
TRIPAY_PRIVATE_KEY=""
TRIPAY_MODE="sandbox"
```

---

### Langkah 4: Buat Service Systemd

Buat file systemd unit `/etc/systemd/system/kasir-api.service`:

```ini
[Unit]
Description=POS Kedai Go Backend API
After=network.target postgresql.service redis-server.service

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/var/www/kasir-api
ExecStart=/var/www/kasir-api/server
Restart=always
RestartSec=5s
EnvironmentFile=/var/www/kasir-api/.env

# Proteksi resource
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

Aktifkan dan jalankan service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now kasir-api.service

# Cek status dan log
sudo systemctl status kasir-api.service
sudo journalctl -u kasir-api.service -f
```

---

### Langkah 5: Konfigurasi Nginx Reverse Proxy & SSL

1. Buat file konfigurasi `/etc/nginx/sites-available/poskedai`:
```nginx
server {
    listen 80;
    server_name api.poskedai.domainkamu.com;

    client_max_body_size 20M;

    # Folder file upload (foto produk, logo toko)
    location /uploads/ {
        alias /var/www/kasir-api/uploads/;
        expires 30d;
        add_header Cache-Control "public, no-transform";
    }

    # Reverse proxy ke API Go (port 8080)
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        
        # Dukungan WebSocket
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

2. Aktifkan konfigurasi dan test:
```bash
sudo ln -s /etc/nginx/sites-available/poskedai /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

3. Pasang SSL gratis dengan Certbot:
```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d api.poskedai.domainkamu.com
```

Selesai. Backend Go kini aktif, aman dengan HTTPS, hemat RAM, dan otomatis restart saat VPS reboot.
