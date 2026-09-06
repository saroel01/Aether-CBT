# PANDUAN RESMI SETUP SERVER & INFRASTRUKTUR JARINGAN AETHER CBT

Panduan ini ditujukan bagi **Proktor, Teknisi IT Sekolah, dan Administrator Sistem** untuk mengonfigurasi PC Server, Router, Access Point, dan infrastruktur jaringan lokal (LAN/WiFi) sebelum pelaksanaan Computer-Based Testing (CBT).

---

## 1. ARSITEKTUR & PRINSIP DESAIN AETHER CBT

Aether CBT dirancang dengan filosofi **Sovereign High-Performance Offline CBT**:
1. **Zero External Dependency**: Backend Go yang dikompilasi secara statis (*native Windows binary*) tidak memerlukan instalasi Node.js, Python, runtime Go, web server eksternal (Apache/Nginx), ataupun server database terpisah (MySQL/PostgreSQL).
2. **SQLite Embedded dengan Mode WAL (Write-Ahead Logging)**:
   - Database terintegrasi langsung di dalam binary (`cbt_aether.db`).
   - Mode WAL memungkinkan operasi pembacaan konkuren (*read*) dilakukan secara paralel tanpa mengunci operasi penulisan (*write*).
   - Pengaturan connection pooling internal: `MaxOpenConns = 25`, `MaxIdleConns = 10`, `busy_timeout = 5000ms`, `foreign_keys = ON`.
3. **Filesystem Submission Queue**:
   - Lembar jawaban kuis iSpring diproses secara asynchronous melalui antrean berbasis berkas lokal (`data/queue/`).
   - Jika terjadi pemadaman listrik mendadak, file antrean tersimpan persisten di disk dan akan otomatis diproses ulang saat server menyala kembali (*crash-resilient*).
4. **Single Page Application (SPA) Serving**:
   - Aset frontend SvelteKit disajikan langsung oleh Fiber v2 melalui embedded static routing dengan header keamanan modern (CSP, HSTS, X-Frame-Options).

---

## 2. KEBUTUHAN PERANGKAT KERAS (HARDWARE REQUIREMENTS)

| Skala Pengujian | Kapasitas Peserta | CPU Server | RAM Server | Jenis Media Penyimpanan |
| :--- | :--- | :--- | :--- | :--- |
| **Simulasi Mandiri** | 1 – 10 Siswa | Dual-Core (Intel Core i3 / Celeron gen 10+) | 4 GB | HDD / SSD apa pun |
| **1 Laboratorium Komputer** | 30 – 50 Siswa | Quad-Core (Intel Core i5 gen 8+ / Ryzen 5 3000+) | 8 GB | **Wajib SSD** (SATA III 500 MB/s) |
| **Multi-Lab / 1 Gedung** | 60 – 120 Siswa | Hexa-Core (Intel Core i5/i7 gen 10+ / Ryzen 5 5600+) | 16 GB | **Wajib NVMe SSD** (M.2 PCIe 3.0/4.0) |
| **Skala Sekolah Penuh** | 150 – 250 Siswa | Octa-Core (Intel Core i7/i9 gen 11+ / Xeon E-2200+) | 16 – 32 GB | **Wajib NVMe SSD Enterprise / High Endurance** |

> [!IMPORTANT]
> **Mengapa SSD Wajib untuk Skala >30 Siswa?**  
> Pada database SQLite WAL, setiap kali siswa mengumpulkan jawaban atau mengirim deteksi anticheat, sistem melakukan operasi I/O write ke berkas jurnal WAL. Hard disk mekanik (HDD) memiliki latensi *seek time* tinggi (10–15 ms) yang dapat memicu antrean `SQLITE_BUSY`. SSD NVMe memiliki latensi <0.1 ms yang mampu melayani ratusan request per detik tanpa jeda.

---

## 3. TOPOLOGI & DESAIN JARINGAN UJIAN SEKOLAH

### A. Diagram Topologi Jaringan yang Direkomendasikan
```
                             ┌──────────────────────────────────────┐
                             │    ROUTER UTAMA (MIKROTIK / CISCO)   │
                             │   - IP Gateway: 192.168.1.1/24       │
                             │   - DHCP Server: 192.168.1.50 - 250  │
                             └──────────────────┬───────────────────┘
                                                │ (Kabel Gigabit Cat6)
                                                ▼
                             ┌──────────────────────────────────────┐
                             │       GIGABIT SWITCH (UNMANAGED)     │
                             └──────┬───────────────┬───────────────┘
                                    │               │
            ┌───────────────────────┘               └───────────────────────┐
            ▼                                                               ▼
┌───────────────────────────────┐                               ┌───────────────────────────────┐
│     PC SERVER CBT SEKOLAH     │                               │      WIRELESS ACCESS POINT    │
│  - IP: 192.168.1.10 (Static)  │                               │    (Ruckus / Ubiquiti / Ruijie│
│  - Port: 3000                 │                               │  - 5 GHz Dual Band            │
│  - Kabel LAN UTP Gigabit      │                               │  - AP Isolation: DISABLED     │
└───────────────────────────────┘                               └───────────────┬───────────────┘
                                                                                │ (WiFi 5 GHz)
                                                        ┌───────────────────────┼───────────────────────┐
                                                        ▼                       ▼                       ▼
                                                ┌───────────────┐       ┌───────────────┐       ┌───────────────┐
                                                │   HP SISWA 1  │       │  TABLET SISWA │       │  LAPTOP SISWA │
                                                │ 192.168.1.51  │       │ 192.168.1.52  │       │ 192.168.1.53  │
                                                └───────────────┘       └───────────────┘       └───────────────┘
```

### B. Aturan Konfigurasi Router & Access Point (AP)

1. **Koneksi Server Wajib Menggunakan Kabel (Wired Ethernet)**:
   - Hubungkan PC Server langsung ke Switch / Router menggunakan kabel LAN UTP minimal **Cat5e atau Cat6**.
   - **Jangan pernah menggunakan koneksi WiFi untuk PC Server** karena paket data server akan bersaing langsung dengan puluhan perangkat siswa pada media frekuensi nirkabel yang sama.
2. **Nonaktifkan "AP Isolation" / "Client Isolation"**:
   - Di menu Access Point atau Router WiFi, cari pengaturan:
     `AP Isolation`, `Client Isolation`, `Station Isolation`, atau `Guest Network Isolation`.
   - **WAJIB DINONAKTIFKAN (DISABLED)**.
   - *Alasan:* Fitur ini memblokir komunikasi antar-perangkat di subnet lokal. Jika aktif, HP siswa tidak akan dapat membuka alamat IP PC Server (`http://192.168.1.10:3000`).
3. **Optimasi Frekuensi WiFi (2.4 GHz vs 5 GHz)**:
   - **Gunakan Frekuensi 5 GHz (802.11ac / WiFi 5 / WiFi 6)**: Kanal frekuensi 5 GHz memiliki lebar *bandwidth* tinggi dan tidak rentan interferensi Bluetooth atau hotspot pribadi.
   - **Kapasitas Access Point**: Rata-rata AP *consumer* (rumahan) hanya stabil untuk 15–25 perangkat. Untuk 30–50 siswa dalam satu ruangan ujian, gunakan **Access Point Enterprise/SOHO** (seperti Ubiquiti UniFi, Ruijie Reyee, Aruba Instant On, atau Mikrotik cAP ac).
4. **Alokasi DHCP Pool & Waktu Sewa (Lease Time)**:
   - Set DHCP Range: Contoh `192.168.1.50` sampai `192.168.1.250` (Netmask `/24`).
   - Set DHCP Lease Time: **120 Menit (2 Jam)** atau **240 Menit (4 Jam)**. Hindari lease time terlalu singkat agar perangkat siswa tidak melakukan renegotiation IP di tengah pengerjaan soal.
5. **Mode Ujian Terisolasi (Offline Murni / White-listed WAN)**:
   - Untuk mencegah siswa mencari kunci jawaban di internet, **cabut kabel internet (WAN)** dari router saat ujian berlangsung. Aether CBT bekerja 100% tanpa internet.
   - Jika router menggunakan Mikrotik, buat firewall rule:
     ```routeros
     /ip firewall filter
     add chain=forward src-address=192.168.1.0/24 dst-address=192.168.1.10 action=accept comment="Allow CBT Server"
     add chain=forward src-address=192.168.1.0/24 action=drop comment="Block Internet for Students"
     ```

---

## 4. KONFIGURASI SISTEM OPERASI PC SERVER (WINDOWS)

Lakukan konfigurasi berikut pada komputer yang bertindak sebagai server:

### A. IP Address Static untuk PC Server
Tetapkan alamat IP statis pada adapter LAN server agar alamat tidak berubah-ubah saat router me-reboot:
1. Buka **Control Panel** -> **Network and Sharing Center** -> **Change adapter settings**.
2. Klik kanan pada **Ethernet** -> **Properties** -> Pilih **Internet Protocol Version 4 (TCP/IPv4)** -> Klik **Properties**.
3. Pilih *"Use the following IP address"*:
   - **IP address**: `192.168.1.10`
   - **Subnet mask**: `255.255.255.0`
   - **Default gateway**: `192.168.1.1` (IP Router Anda)
4. Klik **OK**.

### B. Buka Port 3000 di Windows Firewall
Jalankan **PowerShell as Administrator**, lalu ketik perintah berikut:
```powershell
New-NetFirewallRule -DisplayName "Aether CBT Server Port 3000" -Direction Inbound -LocalPort 3000 -Protocol TCP -Action Allow -Profile Any
```

### C. Set Tipe Jaringan ke "Private Network"
```powershell
Set-NetConnectionProfile -InterfaceAlias "Ethernet" -NetworkCategory Private
```

### D. Nonaktifkan Sleep / Hibernate & Windows Update Otomatis
1. Buka **Settings** -> **System** -> **Power & sleep** -> Set **Never** saat dicolokkan listrik (*plugged in*).
2. Di laptop, buka **Control Panel** -> **Power Options** -> **Choose what closing the lid does** -> Ubah ke **Do nothing** agar laptop tetap menyala saat layar ditutup.
3. Jeda (*Pause*) Windows Update selama 7 hari sebelum masa ujian agar server tidak melakukan restart otomatis di tengah ujian.

---

## 5. PROSEDUR OPERASIONAL UJIAN (SOP PELAKSANAAN)

### FASE 1: H-1 Persiapan (Pra-Ujian)
1. **Upload / Sinkronisasi Paket Soal**:
   - Login ke Admin Panel (`http://localhost:3000/admin`).
   - Masuk ke menu **Bank Soal**, buat mata pelajaran dan upload paket soal iSpring (berkas ZIP HTML5).
   - Pastikan soal telah terverifikasi dan statusnya siap.
2. **Buat Jadwal Sesi Ujian**:
   - Tentukan Nama Ujian, Tanggal Mulai, Tanggal Selesai, dan Durasi Ujian.
   - Hubungkan Sesi Ujian dengan Kelas dan Ruang Ujian.
   - Buat Token Sesi (contoh: `AETHER1` atau acak 6 karakter).
3. **Import Data Peserta**:
   - Siapkan berkas CSV daftar siswa (`NO_ID, NAMA_PESERTA, KELAS_ID, RUANG_ID, PASSWORD`).
   - Unggah melalui menu **Import Siswa** di panel admin.

### FASE 2: Hari H Pelaksanaan Ujian
1. **Nyalakan Server**:
   - Jalankan binary `aether-cbt.exe` (atau melalui skrip runner).
   - Perhatikan konsol log: pastikan log menampilkan `Aether CBT starting on port 3000` dan `[WORKER] Started`.
2. **Pengawas Masuk Ruangan**:
   - Pengawas membuka peramban di meja pengawas: `http://192.168.1.10:3000/supervisor/login`.
   - Login dengan akun ruang masing-masing (misal `lab1` / `password123`).
   - Monitor akan menampilkan daftar siswa yang terdaftar di ruangan tersebut.
3. **Rilis Token ke Siswa**:
   - Proktor membagikan Token Ujian kepada siswa di papan tulis.
   - Siswa membuka browser di HP/Laptop: `http://192.168.1.10:3000/student/login`.
   - Siswa memasukkan No. Ujian, Password, dan Token Ujian.
4. **Monitoring Real-time & Penanganan Kendala Siswa**:
   - **Pindah Tab / Curang**: Layar pengawas akan menampilkan jumlah peringatan (*Tab Switch*). Jika mencapai batas ($\ge 3$), status siswa menjadi `TERKUNCI`.
   - **Buka Kunci Siswa**: Jika siswa terkunci karena ketidaksengajaan, pengawas cukup menekan tombol kuning **"Buka Kunci"** di tabel monitoring. Layar siswa akan otomatis aktif kembali dalam 3 detik tanpa mengulang ujian dari awal.
   - **Listrik / Perangkat Siswa Mati**: Jika HP/laptop siswa mati kehabisan baterai, siswa dapat login kembali menggunakan perangkat lain. Sistem akan melanjutkan sisa waktu dan sesi ujian sebelumnya secara otomatis (*seamless resume*).

### FASE 3: Pasca-Ujian
1. **Pengumpulan Lembar Jawaban**:
   - Seluruh siswa mengklik **Submit** di player kuis iSpring.
   - Pastikan di layar pengawas seluruh siswa berstatus `"Selesai"`.
2. **Ekspor Rekap Nilai**:
   - Administrator membuka menu **Rekap Nilai** di Admin Panel.
   - Klik **Ekspor CSV** (telah dilengkapi UTF-8 BOM untuk Microsoft Excel) atau **Cetak PDF**.
3. **Backup Database**:
   - Matikan server dengan menekan `Ctrl + C` di jendela konsol.
   - Salin file database `data/cbt_aether.db` ke flashdisk atau penyimpanan cloud eksternal sebagai arsip resmi sekolah.

---

## 6. PANDUAN PEMECAHAN MASALAH (TROUBLESHOOTING GUIDE)

| Gejala Masalah | Kemungkinan Penyebab | Solusi Tindakan |
| :--- | :--- | :--- |
| **HP/Laptop siswa tidak bisa membuka URL web server (Timeout / ERR_CONNECTION_TIMED_OUT)** | 1. Windows Firewall server aktif.<br>2. Profil jaringan server berstatus *Public*.<br>3. *AP Isolation* di router menyala.<br>4. Siswa terhubung ke WiFi lain. | 1. Jalankan perintah `New-NetFirewallRule` di Bagian 4.B.<br>2. Ubah profil ke *Private* (Bagian 4.C).<br>3. Nonaktifkan *AP Isolation* di pengaturan WiFi router.<br>4. Pastikan SSID WiFi sama. |
| **Siswa login muncul pesan: `"session has ended"`** | Jam / tanggal di PC server tidak cocok dengan rentang waktu sesi ujian. | Buka Windows Settings -> Date & Time di PC Server. Pastikan tanggal dan jam server sinkron dan berada di dalam jadwal ujian. |
| **Siswa login muncul pesan: `"Invalid exam token"`** | Token yang diketik siswa salah atau status sesi ujian belum diubah ke `"aktif"`. | Periksa token di papan pengawas / admin panel. Pastikan status sesi ujian di admin adalah **Aktif**. |
| **Siswa login muncul pesan: `"Invalid credentials"`** | Nomor peserta atau password tidak cocok dengan database. | Cek data siswa di Admin Panel menu Peserta. Pastikan penulisan huruf besar/kecil password sesuai. |
| **Siswa terkunci saat ujian (`TERKUNCI`)** | Siswa terdeteksi membuka aplikasi lain, jendela pop-up, atau memencet tombol navigasi. | Pengawas memeriksa alasan pelanggaran, lalu mengklik tombol **"Buka Kunci"** di monitor pengawas. |
| **Player soal iSpring tidak muncul (Blank Putih)** | Paket soal iSpring belum di-ekstrak atau cookie `aether_exam` terblokir oleh browser siswa. | 1. Pastikan siswa menggunakan browser modern (Chrome/Safari) dan tidak memblokir cookie pihak pertama.<br>2. Cek apakah folder paket soal di `data/soal/` ada dan memiliki izin baca. |

---

*Dokumentasi ini adalah bagian resmi dari repositori Aether CBT. Disusun dengan pengujian empiris dan standar keamanan server Computer-Based Testing.*
