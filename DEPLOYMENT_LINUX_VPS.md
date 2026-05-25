# PANDUAN DEPLOYMENT LINUX & CLOUD VPS (INTRANET / ONLINE)
## Aether CBT — Platform Computer-Based Testing Modern

Panduan ini dirancang secara instruksional, langkah-demi-langkah, untuk memandu Anda melakukan deployment platform **Aether CBT** pada server Linux (Ubuntu 20.04/22.04/24.04 LTS atau Debian/Fedora) baik untuk jaringan intranet lokal sekolah maupun online menggunakan Cloud VPS.

---

## 1. LANGKAH MENYIAPKAN STRUKTUR DIREKTORI (SERVER LINUX)

Sebelum berkas dikirimkan, Anda harus membuat direktori aplikasi di server Linux dan mengatur hak akses kepemilikan yang tepat agar SQLite dapat beroperasi dalam mode WAL (Write-Ahead Logging).

1.  Buka terminal server Linux Anda.
2.  Buat direktori aplikasi dan folder data untuk penyimpanan database SQLite:
    ```bash
    sudo mkdir -p /var/www/aether-cbt/data
    ```
3.  Ubah kepemilikan direktori tersebut agar dapat diakses penuh oleh pengguna sistem `www-data` (pengguna standar web server di Linux):
    ```bash
    sudo chown -R www-data:www-data /var/www/aether-cbt
    ```
4.  Atur hak akses direktori agar aman (hanya pemilik yang dapat membaca/menulis):
    ```bash
    sudo chmod -R 755 /var/www/aether-cbt
    ```

Struktur direktori produksi di server Anda harus terlihat seperti ini setelah berkas disalin:
```
/var/www/aether-cbt/
├── 📄 aether-cbt            (File biner aplikasi Linux hasil kompilasi)
├── 📁 web/
│   └── 📁 build/           (Folder aset statis frontend hasil 'npm run build')
└── 📁 data/
    └── 📄 cbt_aether.db    (Database SQLite utama - otomatis dibuat saat berjalan)
```

---

## 2. LANGKAH MENJALANKAN SEBAGAI SERVICE (SYSTEMD)

Agar aplikasi Aether CBT berjalan otomatis di latar belakang (*background service*), tetap aktif setelah terminal ditutup, dan otomatis menyala ketika server Linux di-reboot:

### Langkah A: Membuat Berkas Service Systemd
1.  Buat file unit service baru menggunakan teks editor nano:
    ```bash
    sudo nano /etc/systemd/system/aether-cbt.service
    ```
2.  Salin dan tempel baris konfigurasi berikut ke dalam editor:
    ```ini
    [Unit]
    Description=Aether CBT Application Service
    After=network.target

    [Service]
    Type=simple
    User=www-data
    Group=www-data
    WorkingDirectory=/var/www/aether-cbt
    ExecStart=/var/www/aether-cbt/aether-cbt
    Restart=always
    RestartSec=5
    Environment=PORT=3000 DATABASE_URL=data/cbt_aether.db JWT_SECRET=IsiDenganSecretPanjangAcakMinimal32Karakter CORS_ALLOWED_ORIGINS=https://cbt.sekolah.sch.id

# PENTING: JWT_SECRET HARUS sangat kuat dan acak. Aplikasi akan menolak start jika kosong atau lemah.
# CORS_ALLOWED_ORIGINS wajib diisi di produksi untuk mencegah akses dari origin lain.

    [Install]
    WantedBy=multi-user.target
    ```
3.  Simpan berkas dengan menekan **Ctrl+O**, lalu tekan **Enter**, dan keluar dengan menekan **Ctrl+X**.

### Langkah B: Mengaktifkan dan Menjalankan Service
1.  Muat ulang konfigurasi systemd sistem operasi agar membaca service baru:
    ```bash
    sudo systemctl daemon-reload
    ```
2.  Aktifkan service agar otomatis berjalan saat booting:
    ```bash
    sudo systemctl enable aether-cbt.service
    ```
3.  Nyalakan service Aether CBT sekarang:
    ```bash
    sudo systemctl start aether-cbt.service
    ```
4.  Periksa status jalannya service untuk memastikan tidak ada kesalahan:
    ```bash
    sudo systemctl status aether-cbt.service
    ```
    *(Jika status menampilkan tulisan hijau "active (running)", server Go terintegrasi telah aktif sempurna).*

---

## 3. LANGKAH MENYEDIAKAN AKSES ONLINE (NGINX + HTTPS SSL)

Jika server dideploy di Cloud VPS agar dapat diakses dari mana saja menggunakan domain kustom, kita harus menggunakan **Nginx** sebagai *reverse proxy* dan memasang sertifikat **SSL Let's Encrypt**.

### Langkah A: Instalasi Dependensi Server
Jalankan perintah berikut untuk memasang Nginx dan Certbot di server Ubuntu:
```bash
sudo apt update
sudo apt install nginx certbot python3-certbot-nginx -y
```

### Langkah B: Konfigurasi Nginx Server Block (Wildcard Subdomain)
Guna mendukung pembacaan tenant dinamis berbasis subdomain dinamis (misal: `sekolaha.ujiancbt.id`), kita konfigurasikan Nginx agar menerima subdomain secara wildcard.

1.  Hapus konfigurasi default Nginx yang tidak digunakan:
    ```bash
    sudo rm /etc/nginx/sites-enabled/default
    ```
2.  Buat berkas konfigurasi Nginx baru untuk Aether CBT:
    ```bash
    sudo nano /etc/nginx/sites-available/aether-cbt
    ```
3.  Salin dan tempel blok server Nginx berikut (Ganti `ujiancbt.id` dengan nama domain Anda):
    ```nginx
    server {
        listen 80;
        server_name *.ujiancbt.id ujiancbt.id;

        location / {
            proxy_pass http://127.0.0.1:3000;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection 'upgrade';
            proxy_set_header Host $host;
            proxy_cache_bypass $http_upgrade;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }
    }
    ```
4.  Simpan dan keluar (**Ctrl+O**, **Enter**, **Ctrl+X**).
5.  Aktifkan konfigurasi dengan membuat symbolic link:
    ```bash
    sudo ln -s /etc/nginx/sites-available/aether-cbt /etc/nginx/sites-enabled/
    ```
6.  Uji kebenaran sintaks konfigurasi Nginx:
    ```bash
    sudo nginx -t
    ```
7.  Muat ulang Nginx jika tidak ada pesan error:
    ```bash
    sudo systemctl restart nginx
    ```

### Langkah C: Mengamankan Lalu Lintas Data dengan HTTPS (SSL Gratis)
Jalankan perintah Certbot untuk mengonfigurasi sertifikat enkripsi otomatis pada domain utama dan wildcard subdomain Anda:
```bash
sudo certbot --nginx -d ujiancbt.id -d *.ujiancbt.id
```
*Ikuti petunjuk di layar (masukkan email dan setujui syarat layanan). Certbot akan secara otomatis mengamankan rute domain dan memperbarui sertifikat SSL secara terjadwal di latar belakang.*

---

## 4. SISTEM PENCADANGAN DATABASE OTOMATIS (BACKUP CRON JOB)

SQLite berkinerja tinggi dalam mode WAL, namun berkas database tidak boleh langsung disalin secara paksa saat server aktif menulis karena berisiko merusak database. Kita harus menggunakan utilitas `.backup` bawaan SQLite.

1.  Buat berkas skrip pencadangan otomatis:
    ```bash
    sudo nano /var/www/aether-cbt/backup.sh
    ```
2.  Masukkan kode skrip shell berikut:
    ```bash
    #!/bin/bash
    BACKUP_DIR="/var/www/aether-cbt/backups"
    DB_PATH="/var/www/aether-cbt/data/cbt_aether.db"
    DATE=$(date +%Y-%m-%d_%H%M%S)

    # Buat direktori backup jika belum ada
    mkdir -p $BACKUP_DIR
    
    # Lakukan pencadangan database secara aman tanpa mematikan server
    sqlite3 $DB_PATH ".backup '$BACKUP_DIR/backup_$DATE.db'"
    
    # Ubah kepemilikan agar aman
    chown -R www-data:www-data $BACKUP_DIR
    
    # Hapus file cadangan lama yang berusia lebih dari 30 hari untuk menghemat disk VPS
    find $BACKUP_DIR -type f -name "*.db" -mtime +30 -delete
    ```
3.  Simpan dan keluar (**Ctrl+O**, **Enter**, **Ctrl+X**).
4.  Berikan izin eksekusi pada berkas skrip tersebut:
    ```bash
    sudo chmod +x /var/www/aether-cbt/backup.sh
    ```
5.  Daftarkan skrip ke sistem penjadwalan Linux (**crontab**) agar berjalan otomatis setiap tengah malam (pukul 00:00):
    ```bash
    (sudo crontab -l 2>/dev/null; echo "0 0 * * * /var/www/aether-cbt/backup.sh") | sudo crontab -
    ```
