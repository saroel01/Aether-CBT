# Panduan Resmi Setup Server & Infrastruktur Jaringan Aether CBT
**Pedoman Teknis untuk Proktor, Teknisi IT Jaringan, dan Administrator Sekolah**

---

## 1. KEUNGGULAN OPERASIONAL AETHER CBT
Aether CBT dirancang untuk kemandirian dan keandalan penuh dalam pelaksanaan ujian di sekolah:
1. **Instalasi Mandiri (Zero-Dependency):** Tidak membutuhkan instalasi web server eksternal (Apache/XAMPP/Nginx) ataupun database terpisah (MySQL/PostgreSQL). Seluruh komponen backend, antarmuka web, dan database telah terkompilasi utuh di dalam satu paket eksekusi.
2. **Kinerja Tinggi Berbasis Disk SSD:** Menggunakan penyimpanan lokal berkecepatan tinggi yang sanggup melayani ratusan pengumpulan jawaban siswa secara serentak tanpa latensi antrean.
3. **Penyelamatkan Jawaban Persisten (Crash-Resilient):** Jika terjadi pemadaman listrik secara mendadak di tengah ujian, seluruh progres dan jawaban siswa tersimpan aman di disk lokal dan otomatis dipulihkan saat server menyala kembali.
4. **100% Bekerja Tanpa Internet:** Sistem bekerja secara mandiri di jaringan lokal sekolah (*intranet*) sehingga sekolah tidak memerlukan kuota internet berbayar saat ujian berlangsung.

---

## 2. STANDAR SPESIFIKASI PERANGKAT KERAS (HARDWARE REQUIREMENTS)

| Skala Pengujian | Kapasitas Peserta | Rekomendasi CPU Server | RAM Server | Jenis Media Penyimpanan |
| :--- | :--- | :--- | :--- | :--- |
| **Uji Coba Mandiri** | 1 – 10 Siswa | Laptop Intel Core i3 / Celeron | 4 GB | HDD / SSD apa pun |
| **1 Lab Komputer** | 30 – 50 Siswa | PC/Laptop Core i5 / Ryzen 5 | 8 GB | **Wajib SSD** (SATA III) |
| **Multi-Lab / 1 Gedung** | 60 – 120 Siswa | PC Desktop Core i5/i7 gen 8+ | 16 GB | **Wajib SSD NVMe** |
| **Skala Sekolah Penuh** | 150 – 250 Siswa | PC Server Core i7/Xeon | 16 – 32 GB | **Wajib SSD NVMe High Endurance** |

> [!NOTE]
> **Mengapa SSD Diwajibkan untuk Ujian >30 Siswa?**  
> Saat puluhan siswa serentak mengirim jawaban dan deteksi anti-cheat, sistem melakukan pencatatan transaksi data ke berkas database secara intensif. Hard disk mekanik (HDD) memiliki piringan berputar yang lambat dan dapat memicu antrean (*lag*). SSD memiliki kecepatan akses instan (<0.1 ms) sehingga seluruh aksi siswa terproses mulus tanpa jeda.

---

## 3. DESAIN TOPOLOGI JARINGAN UJIAN SEKOLAH

### A. Diagram Fisik Pemasangan Kabel & WiFi:
```text
                         ┌────────────────────────────────────┐
                         │   ROUTER UTAMA (MIKROTIK / TPLINK) │
                         │   - IP Gateway  : 192.168.1.1      │
                         │   - DHCP Server : 192.168.1.50-250 │
                         │   - KABEL WAN / INTERNET: DICABUT  │
                         └─────────────────┬──────────────────┘
                                           │ (Kabel LAN UTP Gigabit)
                                           ▼
                         ┌────────────────────────────────────┐
                         │     GIGABIT SWITCH (8/16/24 PORT)  │
                         └──────┬──────────────────────┬──────┘
                                │                      │
        ┌───────────────────────┘                      └───────────────────────┐
        ▼                                                                      ▼
┌───────────────────────────────┐                              ┌───────────────────────────────┐
│     KOMPUTER SERVER CBT       │                              │      ACCESS POINT WIFI 5 GHz  │
│  - IP Statis: 192.168.1.10    │                              │  - SSID: UJIAN_SEKOLAH_5G     │
│  - Port Server: 3000          │                              │  - AP Isolation: NONAKTIF     │
│  - Kabel LAN UTP Gigabit      │                              └───────────────┬───────────────┘
└───────────────────────────────┘                                              │ (WiFi 5 GHz)
                                                        ┌──────────────────────┼──────────────────────┐
                                                        ▼                      ▼                      ▼
                                                ┌───────────────┐      ┌───────────────┐      ┌───────────────┐
                                                │   HP SISWA 1  │      │  TABLET SISWA │      │  LAPTOP SISWA │
                                                │ 192.168.1.51  │      │ 192.168.1.52  │      │ 192.168.1.53  │
                                                └───────────────┘      └───────────────┘      └───────────────┘
```

### B. Aturan Penting Konfigurasi Jaringan:
1. **Server Wajib Memakai Kabel LAN (Wired Ethernet):**  
   Hubungkan komputer server langsung ke switch/router menggunakan kabel LAN Cat5e/Cat6. Jangan menghubungkan komputer server melalui WiFi karena sinyal server akan berebut frekuensi nirkabel dengan puluhan HP siswa.
2. **Nonaktifkan "AP Isolation" / "Client Isolation":**  
   Di menu pengaturan Access Point / Router WiFi, pastikan fitur bernama `AP Isolation`, `Client Isolation`, atau `Station Isolation` dalam keadaan **NONAKTIF (DISABLED)**. Jika fitur ini aktif, perangkat siswa akan diblokir untuk mengakses IP komputer server.
3. **Gunakan Frekuensi 5 GHz (Dual Band):**  
   Untuk ruangan dengan 30–40 siswa berbasis smartphone, gunakan Access Point frekuensi 5 GHz (WiFi 5 / WiFi 6) agar pita jaringan luas dan tidak terganggu gelombang sinyal lainnya.
4. **Alokasikan Jumlah IP DHCP yang Cukup:**  
   Atur DHCP Pool router agar mampu menampung seluruh peserta ujian (misalnya `192.168.1.50` s/d `192.168.1.250`). Atur DHCP Lease Time ke `120 Menit` atau `240 Menit`.
5. **Mode Ujian Terisolasi (Cabut Kabel Internet):**  
   Untuk memastikan ujian bebas kecurangan, cukup cabut kabel internet (WAN) dari router saat ujian dimulai. Aether CBT beroperasi 100% tanpa internet.

---

## 4. KONFIGURASI SISTEM OPERASI KOMPUTER SERVER (WINDOWS)

Lakukan 3 langkah konfigurasi ini pada komputer yang dijadikan server ujian:

### Langkah 1: Pasang IP Statis pada Komputer Server
Agar alamat IP server tidak berganti-ganti saat router menyala ulang:
1. Buka **Control Panel** $\rightarrow$ **Network and Sharing Center** $\rightarrow$ **Change adapter settings**.
2. Klik kanan pada **Ethernet** $\rightarrow$ Pilih **Properties** $\rightarrow$ Klik ganda **Internet Protocol Version 4 (TCP/IPv4)**.
3. Pilih opsi *"Use the following IP address"*:
   - **IP address**: `192.168.1.10`
   - **Subnet mask**: `255.255.255.0`
   - **Default gateway**: `192.168.1.1` (IP router Anda)
4. Klik **OK**.

### Langkah 2: Buka Port 3000 pada Windows Firewall
1. Klik kanan tombol Start Windows $\rightarrow$ Pilih **Terminal (Admin)** atau **PowerShell (Admin)**.
2. Tempelkan perintah berikut lalu tekan Enter:
   ```powershell
   New-NetFirewallRule -DisplayName "Aether CBT Server Port 3000" -Direction Inbound -LocalPort 3000 -Protocol TCP -Action Allow -Profile Any
   ```

### Langkah 3: Matikan Fitur Tidur (Sleep) & Kunci Layar
1. Buka **Windows Settings** $\rightarrow$ **System** $\rightarrow$ **Power & sleep**.
2. Ubah *"When plugged in, PC goes to sleep after"* menjadi **Never**.
3. Pada laptop, buka **Control Panel** $\rightarrow$ **Power Options** $\rightarrow$ **Choose what closing the lid does** $\rightarrow$ Ubah ke **Do nothing** agar server tetap aktif saat layar laptop ditutup.

---

## 5. STANDAR OPERASIONAL PROSEDUR UJIAN (SOP PELAKSANAAN)

### FASE 1: H-1 Persiapan (Pra-Ujian)
- [ ] **Upload Paket Soal:** Login ke `http://localhost:3000/admin`, masuk menu sidebar **Paket Soal**, buat mata pelajaran dan upload paket ZIP soal kuis iSpring (HTML5).
- [ ] **Buat Jadwal Sesi Ujian:** Masuk menu sidebar **Sesi Ujian**, tentukan Nama Ujian, Rentang Tanggal, Durasi Pengerjaan, dan Token Sesi (contoh: `AETHER1`).
- [ ] **Import Data Siswa:** Unggah daftar peserta melalui menu **Peserta Ujian** menggunakan format CSV (`no_id, nama_peserta, kelas_id, ruang_id, jenis_kelamin, password`).
- [ ] **Cetak Kartu Peserta:** Klik tombol **"Cetak Kartu Ujian"** di menu Peserta Ujian.
- [ ] **Cetak Dokumen Ruangan:** Masuk menu **Ruang Ujian**, klik **"Cetak Daftar Hadir"** dan **"Cetak Berita Acara"** untuk masing-masing ruangan.
- [ ] **Uji Sampel Klien:** Hubungkan 1 HP ke WiFi ujian dan lakukan login percobaan.

### FASE 2: Hari H Pelaksanaan Ujian
- [ ] **Nyalakan Server:** Jalankan file `run-test.bat` (atau `aether-cbt.exe`). Pastikan konsol terbuka dan menampilkan status server aktif di port 3000.
- [ ] **Pengawas Masuk Ruangan:** Pengawas membuka laptop pengawas di alamat `http://192.168.1.10:3000/supervisor/login` dan login menggunakan akun ruangan masing-masing (misal `lab1`).
- [ ] **Rilis Token Ujian:** Tulis Token Ujian (contoh: `AETHER1`) di papan tulis ruang ujian.
- [ ] **Siswa Masuk:** Siswa membuka `http://192.168.1.10:3000` di HP/laptop masing-masing, mengklik **"Portal Login Siswa"**, dan memasukkan nomor peserta, password, serta token ujian.
- [ ] **Monitoring Live (Pemantauan Ruangan Real-Time):**
  - Pengawas memantau perkembangan ujian siswa secara langsung di layar pemantauan proktor (`http://192.168.1.10:3000/supervisor/login`).
  - Data di tabel pemantauan melakukan penyegaran (*refresh*) otomatis secara berkala (setiap 3 detik) tanpa pengawas perlu menekan tombol reload browser.
  - **Status Awal Siswa:** Siswa yang belum masuk atau belum mulai mengerjakan berstatus netral abu-abu: **`Idle`**.
  - **Status Saat Siswa Mulai:** Begitu siswa login dan mengklik tombol "Mulai Ujian", status siswa otomatis berubah menjadi biru: **`Mengerjakan`**, kolom mulai terisi waktu login, dan bilah progres (*progress bar*) menampilkan perbandingan soal yang sudah dijawab terhadap total soal.
  - **Deteksi Anti-Cheat (Pindah Layar / Tab):** Jika siswa terdeteksi berpindah tab atau membuka aplikasi lain di perangkatnya, lencana merah berkedip bertuliskan **`⚠️ 1x Tab`** (dan seterusnya) akan langsung muncul di samping nama siswa tersebut pada monitor pengawas.
  - **Penanganan Siswa Terkunci (Auto-Lock):** Jika siswa mencapai batas pelanggaran ($\ge 3$ kali), status siswa berubah menjadi merah berkedip: **`⚠️ TERKUNCI`** dan layar siswa dibekukan. Tombol kuning **"Buka Kunci"** akan muncul di kolom Aksi siswa tersebut pada monitor pengawas. Pengawas cukup mengklik **"Buka Kunci"** untuk memulihkan akses siswa dalam 3 detik tanpa menghapus progres atau jawaban sebelumnya.
  - **Tombol Reset Sesi:** Tombol merah **"Reset Sesi"** hanya digunakan apabila perangkat siswa mengalami masalah teknis parah (seperti HP mati total atau layar membeku permanen) dan perlu login ulang dari perangkat baru.

### FASE 3: Pasca Ujian (Penutupan & Rekapitulasi)
- [ ] **Konfirmasi Pengumpulan:** Pastikan seluruh siswa di tabel monitor pengawas telah berstatus hijau: **`Selesai`**.
- [ ] **Ekspor Rekap Skor:** Masuk ke Admin Panel (`http://192.168.1.10:3000/admin`), lalu di pojok kanan atas halaman Dashboard klik tombol biru **"Ekspor Skor (CSV)"**. Berkas CSV yang diunduh telah disisipi UTF-8 BOM sehingga langsung terbaca rapi di Microsoft Excel Windows tanpa masalah karakter rusak.
- [ ] **Analisis Butir Soal (Opsional):** Buka menu **Analisis Soal** di sidebar admin untuk meninjau tingkat kesukaran dan daya pembeda butir soal kuis.
- [ ] **Matikan Server dengan Aman:** Buka jendela konsol hitam server, lalu tekan **`Ctrl + C`**. Ketik huruf **`Y`** dan tekan **Enter** untuk mematikan server secara aman (*graceful shutdown*).
- [ ] **Arsipkan Database:** Salin berkas `data/cbt_aether.db` ke flashdisk atau penyimpanan arsip sekolah.

---

## 6. PANDUAN PEMECAHAN MASALAH (TROUBLESHOOTING)

| Gejala Kendala | Akar Penyebab | Solusi Penanganan |
| :--- | :--- | :--- |
| **HP siswa loading lama atau gagal terhubung ke server (Timeout)** | 1. HP terhubung ke WiFi lain.<br>2. Windows Firewall server aktif.<br>3. Fitur *AP Isolation* di router menyala. | 1. Pastikan nama WiFi di HP sama persis dengan router server.<br>2. Jalankan perintah firewall PowerShell di Bagian 4 Langkah 2.<br>3. Buka menu router WiFi dan matikan (*Disable*) fitur AP Isolation. |
| **Siswa login muncul error: `"session has ended"`** | Jam atau tanggal di laptop server tidak akurat (di luar jadwal ujian). | Sinkronkan tanggal dan jam di laptop server. Periksa rentang jadwal sesi ujian di menu Sesi Ujian Admin Panel. |
| **Siswa login muncul error: `"Invalid credentials"`** | Salah memasukkan Nomor Peserta atau Password. | Pastikan nomor peserta terdaftar di menu Peserta Ujian dan kata sandi diketik dengan huruf kecil semua. |
| **Siswa login muncul error: `"Invalid exam token"`** | Token yang dimasukkan tidak cocok dengan token aktif. | Periksa token yang berlaku di papan tulis atau menu Sesi Ujian. Pastikan diketik dengan huruf kapital. |
| **Layar siswa terkunci (`UJIAN ANDA DIKUNCI!`)** | Siswa berpindah aplikasi atau membuka tab browser lain $\ge 3$ kali. | Pengawas mengecek monitor ruangan, lalu mengklik tombol kuning **"Buka Kunci"** di baris nama siswa tersebut. |
| **Tombol "Reset Sesi" tidak muncul di samping nama siswa** | Siswa belum login (*Idle*) atau sudah menyelesaikan ujian (*Selesai*). | Tombol Reset Sesi hanya aktif untuk siswa yang sedang dalam sesi ujian berjalan (*Mengerjakan* atau *Terkunci*). Untuk siswa yang sudah selesai, data nilai sudah tersimpan permanen. |

---

*Dokumentasi resmi sistem Aether CBT. Disusun berdasarkan arsitektur kode sumber aktual dan prosedur operasional standar sekolah.*
