# Aether CBT

**Modern Computer-Based Testing Platform with Multi-Tenant Architecture**

Aether CBT adalah platform Computer-Based Testing (CBT) multi-tenant berkinerja tinggi, aman, dan siap pakai yang dirancang untuk sekolah. Platform ini mendukung kapasitas tinggi dengan true offline capability menggunakan Go (Fiber) di sisi backend, SQLite (WAL mode) untuk penyimpanan terisolasi yang andal, dan SvelteKit di sisi frontend.

---

## 🚀 Fitur Utama (Core Features)

*   **Multi-Tenant yang Ketat**: Isolasi data penuh antara sekolah (tenant) menggunakan pemfilteran dinamis `tenant_id` pada tingkat database dan middleware.
*   **Integrasi iSpring QuizMaker Dinamis**: 
    *   Mendukung penuh unmarshaling berkas detail XML (`dr`) yang dikirimkan oleh iSpring QuizMaker.
    *   Menggunakan parser polymorphic dinamis (`xml:",any"`) yang mematuhi skema XSD iSpring (Substitution Groups).
    *   Pemetaan otomatis jawaban pilihan ganda berbasis indeks (`userAnswerIndex` & `correctAnswerIndex`) ke teks jawaban riil siswa.
*   **Proteksi Keamanan Anti-Cheat**: Validasi sesi ujian secara langsung di monitor ruang pengawas (`cek_login`). Pengiriman hasil kuis di luar sesi ujian aktif akan otomatis ditolak dengan kode **`403 Forbidden`**.
*   **Ekspor Lembar Jawaban Esai Multi-Format (CSV, XLSX, PDF)**:
    *   *CSV*: Rekapan cepat grid data.
    *   *Excel (XLSX)*: Desain visual premium (Steel Blue header, auto-fit, grid borders, dan wrap text otomatis pada kolom esai siswa).
    *   *PDF Cetak Premium*: Dilengkapi Kop Surat Tenant Sekolah formal, pemisah visual soal (kotak abu-abu lembut `#F5F5F5`), jawaban esai siswa berwarna biru tua, kolom input nilai fisik korektor guru (`Skor: ____ / ____`), dan penomoran halaman dinamis.
*   **Batas Ruang Pengawas**: Supervisor dapat memantau aktivitas ruang ujian secara real-time dan melakukan reset sesi siswa jika terdeteksi kecurangan atau kendala teknis.

---

## 🛠️ Tech Stack

*   **Backend**: Go (Fiber v2)
*   **Database**: SQLite 3 (WAL Mode + Enforced Foreign Keys)
*   **Frontend**: SvelteKit + TypeScript + Tailwind CSS (Functional MVP)
*   **Ekspor & Cetak**: `excelize/v2` (Spreadsheet XLSX), `gofpdf` (Dokumen PDF)
*   **QR Code**: Skip2 QR Code Generator

---

## 📁 Struktur Proyek (Project Structure)

```
aether-cbt/
├── cmd/
│   ├── server/main.go         # Entry point server utama
│   ├── createadmin/main.go    # Script pembuat admin default
│   └── seed/main.go           # Script seeder data simulasi
├── internal/
│   ├── api/
│   │   ├── handlers/          # Controller HTTP (ispring, csv_utility, auth, dll.)
│   │   └── middleware/        # Middleware Auth & Tenant dinamis
│   ├── config/                # Konfigurasi aplikasi dari env
│   ├── db/
│   │   ├── sqlite.go          # Pengaturan koneksi SQLite WAL
│   │   ├── migrate.go         # Runner migrasi database otomatis
│   │   └── migrations/        # Berkas migrasi database terurut (.sql)
│   ├── models/                # Struktur database GORM/SQL
│   └── utils/                 # Helper enkripsi, token JWT, QR Code, dan respons
├── web/                       # Aplikasi SvelteKit frontend (Early MVP)
├── data/                      # Folder database SQLite & aset kuis iSpring per tenant
└── docs/                      # Dokumentasi lengkap arsitektur dan spesifikasi UI
```

---

## 🏁 Memulai (Getting Started)

Aether CBT dirancang untuk dijalankan dengan sangat mudah, baik oleh operator sekolah (pengguna awam) maupun oleh pengembang perangkat lunak.

### A. Cara Mudah (Untuk Sekolah & Pengguna Awam)
Sangat praktis! Anda tidak membutuhkan instalasi Go atau Node.js. Cukup gunakan berkas rilis terkompilasi yang menyajikan frontend dan backend sekaligus pada port `3000`:
1.  Unduh paket rilis ZIP Aether CBT dan ekstrak di server lokal.
2.  Klik dua kali berkas biner `aether-cbt.exe` (Windows) atau jalankan `./aether-cbt` (Linux/macOS).
3.  Akses platform di alamat: `http://localhost:3000` (atau IP server Anda, misal `http://192.168.1.15:3000`).

### B. Mode Pengembangan (Untuk Developer)
1.  **Prerequisites**: Pastikan Anda memiliki **Go 1.22+** dan **Node.js 18+** terinstal di sistem Anda.
2.  **Jalankan Mode Dev** (Hot-reload frontend & backend):
    ```bash
    npm run dev
    ```
3.  **Mengisi Data Simulasi Awal** (Seeding):
    ```bash
    npm run seed
    ```

---

## 📚 Dokumentasi Lebih Lanjut

Seluruh spesifikasi teknis dan panduan operasional tersedia secara lengkap di folder `docs/` dan root direktori:

*   **[Panduan Penggunaan Lengkap](USAGE_GUIDE.md)** — Berisi petunjuk operasional Admin, Guru, dan Integrasi Kuis iSpring QuizMaker.
*   **[PRD.md](docs/PRD.md)** — Product Requirements Document.
*   **[Technical_Architecture.md](docs/Technical_Architecture.md)** — Desain arsitektur teknis dan detail API endpoints.
*   **[Database_Schema.md](docs/Database_Schema.md)** — Skema database lengkap SQLite 3.
*   **[UI_Component_Library.md](docs/UI_Component_Library.md)** — Desain spesifikasi visual dan komponen PWA UI.
