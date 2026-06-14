# Requirements Document

Penjadwalan Ujian Detail & Pengiriman Konten iSpring — Aether CBT

## Introduction

Aether CBT saat ini sudah memiliki fondasi yang kuat: backend Go/Fiber, SQLite (WAL), frontend SvelteKit, multi-tenant, JWT + role middleware, webhook iSpring dengan validasi `attempt_token`, serta filesystem submission queue yang andal. Namun ada dua kesenjangan fungsional yang membuat aplikasi belum dapat menjalankan ujian sungguhan:

1. **Konten ujian belum nyata.** Halaman ujian siswa (`web/src/routes/student/exam/+page.svelte`) masih membuat soal pilihan ganda secara hardcoded melalui `generateQuestions()` dan menghasilkan XML iSpring tiruan. Tidak ada mekanisme untuk mengunggah paket iSpring HTML5 hasil ekspor guru, menyimpannya, menautkannya ke ujian yang tepat, dan menyajikannya ke siswa yang berhak.

2. **Tidak ada penjadwalan.** Sistem hanya memiliki satu token global per tenant (`settings.token`), satu flag `is_exam_active`, dan `durasi_menit` per mapel. Tidak ada konsep tingkatan (X/XI/XII), definisi ujian, sesi/gelombang dengan jendela waktu (mulai–selesai), maupun token per-sesi. Padahal kebutuhan operasional adalah menangani hingga 500 siswa yang terbagi dalam beberapa tingkatan dan kelas, dengan mata pelajaran dan jadwal yang berbeda-beda.

Spesifikasi ini menggabungkan dua pekerjaan yang saling bergantung (penjadwalan detail + pengiriman konten iSpring) menjadi satu lingkup yang utuh, karena sebuah sesi ujian tidak berguna tanpa paket soal yang tertaut, dan paket soal tidak dapat disajikan secara aman tanpa konteks sesi.

### Lingkup (In Scope)

- Tingkatan pada kelas; entitas definisi ujian; entitas sesi ujian (gelombang) dengan jendela waktu dan token per-sesi.
- Unggah, validasi, penyimpanan, penautan, dan penyajian paket iSpring HTML5 per tenant.
- Penjadwalan ulang pengiriman hasil iSpring (shim) agar bebas dari URL hardcoded dan menyuntikkan `attempt_token`/`tenant_id`/`sid` secara otomatis.
- Penegakan sesi di sisi server (jendela waktu, sesi tunggal, kunci sesi/lock) sebagai bagian intrinsik dari penjadwalan dan anti-cheat dasar.
- UI admin untuk mengelola tingkatan, ujian, sesi, unggah paket, dan penautan.
- Pengerasan skala untuk 500 siswa konkuren (connection pool SQLite, debounce progress, bukti uji beban).
- Migrasi data dan kompatibilitas mundur dengan data serta alur yang sudah ada.

### Di Luar Lingkup (Out of Scope)

- Mesin soal native (bank soal + grading 100% sisi server) sebagai pengganti iSpring — direncanakan sebagai fase terpisah berikutnya.
- Konverter native ke HTML5.
- Mode kiosk/lockdown tingkat sistem operasi (akan didokumentasikan sebagai panduan deployment, bukan kode).
- Migrasi database dari SQLite ke Postgres (disiapkan jalannya melalui lapisan repository, namun tidak dieksekusi dalam spec ini).

## Glossary

- **Tingkat (tingkatan):** jenjang kelas, mis. X, XI, XII.
- **Ujian (definisi ujian):** kombinasi tetap dari mata pelajaran + tingkat + paket soal + durasi + KKM + pengaturan pengacakan. Dapat dijalankan berkali-kali melalui sesi.
- **Sesi ujian (gelombang):** instans terjadwal dari sebuah ujian dengan jendela waktu (mulai–selesai), token unik per-sesi, serta daftar kelas dan ruang peserta.
- **Paket soal (soal package):** hasil ekspor iSpring QuizMaker HTML5 (folder berisi `index.html`, `data/`, dll.) yang diunggah, disimpan per tenant, dan ditautkan ke satu atau lebih ujian.
- **attempt_token:** rahasia per attempt yang diterbitkan saat siswa memulai sesi; wajib dikirim ulang bersama hasil.

---

## Requirements

### Requirement 1: Tingkatan pada Kelas

**User Story:** Sebagai admin sekolah, saya ingin menetapkan tingkatan pada setiap kelas, sehingga saya dapat menjadwalkan ujian per tingkatan secara terpisah.

#### Acceptance Criteria

1. WHEN admin membuat atau menyunting kelas THEN sistem SHALL menyimpan atribut `tingkat` (mis. "X", "XI", "XII") yang terkait kelas tersebut dalam konteks tenant aktif.
2. WHEN migrasi database dijalankan pada database yang sudah berisi tabel `kelas` THEN sistem SHALL menambahkan kolom `tingkat` tanpa menghapus atau merusak data kelas yang sudah ada, dan SHALL tetap idempoten pada eksekusi berulang sesuai pola `RunMigrations` yang berlaku.
3. WHERE sebuah kelas belum memiliki `tingkat` (data lama) THE sistem SHALL memperlakukan nilai kosong sebagai "belum ditetapkan" dan SHALL tetap mengizinkan operasi baca tanpa error.
4. WHEN admin mengambil daftar kelas THEN sistem SHALL menyertakan `tingkat` pada setiap baris.
5. THE sistem SHALL menjaga isolasi tenant: kelas dari tenant lain TIDAK SHALL pernah terbaca atau termodifikasi.

### Requirement 2: Definisi Ujian

**User Story:** Sebagai admin, saya ingin mendefinisikan ujian (mapel + tingkat + paket soal + durasi + KKM + pengacakan), sehingga ujian dapat dijadwalkan ulang melalui sesi tanpa mendefinisikan ulang.

#### Acceptance Criteria

1. WHEN admin membuat definisi ujian THEN sistem SHALL menyimpan: `tenant_id`, `mapel_id`, `tingkat`, `soal_package_id` (boleh kosong saat draft), `durasi_menit`, `kkm`, dan flag pengaturan pengacakan soal/jawaban.
2. IF `mapel_id` yang dirujuk tidak ada atau bukan milik tenant aktif THEN sistem SHALL menolak pembuatan dengan respons error validasi (HTTP 400) dan TIDAK SHALL membuat baris.
3. WHEN admin menautkan `soal_package_id` ke ujian THEN sistem SHALL memvalidasi bahwa paket tersebut ada dan milik tenant aktif sebelum menyimpan.
4. WHEN admin mengambil daftar ujian THEN sistem SHALL mengembalikan hanya ujian milik tenant aktif beserta nama mapel, tingkat, status ketersediaan paket, dan jumlah sesi terkait.
5. IF admin mencoba menghapus ujian yang masih memiliki sesi berstatus terjadwal atau aktif THEN sistem SHALL menolak penghapusan dan SHALL mengembalikan pesan yang menjelaskan alasannya.
6. THE sistem SHALL menerapkan soft delete pada definisi ujian, konsisten dengan tabel master lain.

### Requirement 3: Unggah Paket Soal iSpring

**User Story:** Sebagai admin, saya ingin mengunggah berkas ZIP hasil ekspor iSpring QuizMaker HTML5, sehingga konten ujian asli dapat disimpan di server dan dipakai untuk ujian.

#### Acceptance Criteria

1. WHEN admin mengunggah berkas melalui endpoint unggah paket THEN sistem SHALL menerima berkas ZIP dan SHALL menolak tipe berkas selain ZIP.
2. IF ukuran berkas melebihi batas terkonfigurasi THEN sistem SHALL menolak unggahan dengan HTTP 413 (atau 400 dengan pesan jelas) dan TIDAK SHALL menyimpan berkas parsial.
3. WHEN sistem mengekstrak ZIP THEN sistem SHALL mencegah path traversal (zip-slip): setiap entri yang resolusinya keluar dari direktori tujuan SHALL ditolak dan seluruh unggahan SHALL dibatalkan.
4. IF arsip tidak memuat `index.html` di akar paket THEN sistem SHALL menolak unggahan sebagai paket iSpring tidak valid.
5. WHEN ekstraksi berhasil THEN sistem SHALL menyimpan berkas di `data/soal/{tenant_slug}/{package_uuid}/` sehingga terisolasi per tenant dan antar paket.
6. WHEN sistem mencatat metadata paket THEN sistem SHALL menyimpan: `tenant_id`, nama tampil, `entry_path` (relatif, default `index.html`), versi iSpring yang terdeteksi dari komentar header `index.html` bila ada, ukuran total, waktu unggah, dan identitas pengunggah.
6a. WHEN sistem mendeteksi versi iSpring dari komentar header `index.html` THEN deteksi SHALL bersifat best-effort: jika pola versi tidak ditemukan, sistem SHALL menyimpan versi sebagai tidak diketahui dan TIDAK SHALL menggagalkan unggahan karena alasan ini.
7. IF proses ekstraksi gagal di tengah jalan THEN sistem SHALL membersihkan berkas yang sudah terekstrak sebagian sehingga tidak meninggalkan paket korup.
8. THE endpoint unggah SHALL hanya dapat diakses oleh role admin/superadmin dan SHALL menolak role lain dengan HTTP 403.
9. WHEN admin meminta daftar paket THEN sistem SHALL mengembalikan hanya paket milik tenant aktif.
10. WHEN admin menghapus paket yang tidak tertaut ke ujian aktif mana pun THEN sistem SHALL menghapus baris metadata dan berkas paket dari disk; IF paket masih tertaut ke ujian THEN sistem SHALL menolak penghapusan dengan pesan jelas.

### Requirement 4: Sesi Ujian (Gelombang) dengan Jendela Waktu dan Token Per-Sesi

**User Story:** Sebagai admin, saya ingin menjadwalkan sesi ujian dengan waktu mulai–selesai, token unik, serta daftar kelas dan ruang peserta, sehingga 500 siswa dapat dibagi ke beberapa gelombang yang berbeda jadwal.

#### Acceptance Criteria

1. WHEN admin membuat sesi untuk sebuah ujian THEN sistem SHALL menyimpan: `ujian_id`, `waktu_mulai`, `waktu_selesai`, `token` (unik per tenant), status (`draft`/`terjadwal`/`aktif`/`selesai`/`dibatalkan`), serta keterkaitan ke kelas dan ruang peserta.
2. IF `waktu_selesai` tidak lebih besar dari `waktu_mulai` THEN sistem SHALL menolak pembuatan/penyuntingan dengan error validasi.
3. IF ujian yang dirujuk belum memiliki `soal_package_id` yang valid THEN sistem SHALL menolak transisi sesi ke status `terjadwal`/`aktif` dan SHALL mengembalikan pesan yang menjelaskan paket soal belum ditautkan.
4. WHEN admin menetapkan token sesi THEN token SHALL unik dalam lingkup tenant pada rentang waktu sesi yang tumpang tindih, sehingga siswa tidak dapat tertukar antar sesi aktif.
5. THE sistem SHALL menghitung status efektif sesi dari waktu server: sesi dianggap dapat dimasuki HANYA WHEN waktu server berada dalam `[waktu_mulai, waktu_selesai]` DAN status administratif mengizinkan.
6. WHEN admin mengambil daftar sesi THEN sistem SHALL mengembalikan hanya sesi milik tenant aktif beserta nama ujian, mapel, tingkat, jumlah peserta tertaut, dan status efektif.
7. IF admin mengaitkan kelas/ruang yang bukan milik tenant aktif ke sesi THEN sistem SHALL menolak operasi tersebut.
8. WHERE beberapa sesi dijadwalkan untuk tingkat dan mapel yang sama pada waktu berbeda THE sistem SHALL memperlakukannya sebagai gelombang independen dengan token masing-masing.

### Requirement 5: Penautan Peserta ke Sesi

**User Story:** Sebagai admin, saya ingin peserta otomatis tertaut ke sesi berdasarkan kelas/ruang, sehingga saya tidak perlu menetapkan peserta satu per satu.

#### Acceptance Criteria

1. WHEN sebuah sesi dikaitkan dengan satu atau lebih kelas THEN sistem SHALL menganggap seluruh peserta aktif pada kelas tersebut (dalam tenant yang sama) sebagai peserta sesi.
2. WHERE sesi juga dibatasi ke ruang tertentu THE sistem SHALL hanya menganggap peserta yang `ruang_id`-nya termasuk dalam ruang sesi sebagai peserta yang berhak.
3. WHEN sistem menentukan kelayakan peserta untuk memulai sesi THEN penentuan SHALL dilakukan di sisi server berdasarkan keanggotaan kelas/ruang dan status efektif sesi, BUKAN berdasarkan input dari klien.
4. IF seorang peserta tidak termasuk dalam kelas/ruang sesi THEN sistem SHALL menolak permintaan memulai sesi tersebut dengan HTTP 403.

### Requirement 6: Login Siswa & Validasi Token Berbasis Sesi

**User Story:** Sebagai siswa, saya ingin masuk menggunakan nomor peserta, kata sandi, dan token sesi yang sah, sehingga saya hanya bisa mengakses ujian yang dijadwalkan untuk saya.

#### Acceptance Criteria

1. WHEN siswa mengirim `no_id`, password, dan token THEN sistem SHALL memvalidasi token terhadap sesi yang berstatus efektif dapat dimasuki pada waktu server saat itu.
2. IF token tidak cocok dengan sesi mana pun yang dapat dimasuki THEN sistem SHALL menolak login dengan HTTP 401 dan pesan token tidak valid/sesi belum dibuka.
3. IF token cocok dengan sesi yang ada tetapi waktu server di luar jendela `[waktu_mulai, waktu_selesai]` THEN sistem SHALL menolak dengan pesan yang membedakan "sesi belum dimulai" dan "sesi telah berakhir".
4. WHEN kredensial dan token valid DAN peserta layak untuk sesi tersebut THEN sistem SHALL menerbitkan JWT role `student` yang membawa konteks tenant, dan respons SHALL menyertakan identitas sesi yang berhak diikuti.
5. THE sistem SHALL terus mendukung verifikasi password bcrypt maupun plaintext legacy melalui mekanisme yang sudah ada (`CheckPasswordOrPlaintext`).
6. THE sistem SHALL mempertahankan kompatibilitas: alur lama berbasis `settings.token` global tetap berfungsi selama masa transisi, ATAU diganti sepenuhnya oleh token sesi dengan jalur migrasi yang terdokumentasi (keputusan final ditetapkan pada tahap design).

### Requirement 7: Memulai Sesi & Penerbitan attempt_token

**User Story:** Sebagai siswa yang sudah login, saya ingin memulai ujian pada sesi saya, sehingga server mencatat sesi aktif saya dan menerbitkan token attempt untuk pengiriman hasil.

#### Acceptance Criteria

1. WHEN siswa memulai sesi yang sah THEN sistem SHALL membuat/memperbarui baris sesi aktif (`cek_login`) yang mereferensikan sesi terjadwal, dan SHALL menerbitkan `attempt_token` acak yang aman.
2. THE sistem SHALL menjamin satu sesi aktif per (tenant, peserta, sesi) melalui batasan keunikan database, konsisten dengan mekanisme indeks unik yang sudah ada.
3. IF siswa mencoba memulai sesi di luar jendela waktu efektif THEN sistem SHALL menolak dengan HTTP 403.
4. IF sesi aktif siswa dalam keadaan terkunci (lihat Requirement 10) THEN sistem SHALL menolak untuk memulai/melanjutkan hingga pengawas membuka kunci.
5. WHEN sesi aktif dibuat THEN waktu mulai otoritatif SHALL berasal dari server, dan sisa waktu SHALL dihitung sebagai `min(durasi_ujian, waktu_selesai_sesi − now)` agar siswa tidak dapat melampaui jendela sesi maupun durasi ujian.

### Requirement 8: Penyajian Konten iSpring yang Terotorisasi

**User Story:** Sebagai siswa yang berhak, saya ingin paket iSpring asli tampil di halaman ujian saya, sehingga saya mengerjakan soal yang sebenarnya.

#### Acceptance Criteria

1. WHEN siswa pemilik sesi aktif meminta konten ujian THEN sistem SHALL menyajikan berkas paket iSpring yang tertaut ke ujian sesi tersebut dari direktori tenant yang sesuai.
2. IF peminta bukan pemilik sesi aktif yang valid, ATAU sesi di luar jendela waktu, ATAU sesi terkunci THEN sistem SHALL menolak penyajian konten dengan HTTP 403.
3. THE penyajian berkas SHALL mencegah path traversal: permintaan berkas yang resolusinya keluar dari direktori paket SHALL ditolak.
4. THE sistem SHALL menyajikan seluruh aset relatif paket (`data/...`, font, gambar, `player.js`) dengan tipe konten yang benar sehingga player iSpring berjalan utuh.
5. THE mekanisme penyajian SHALL bekerja identik baik pada deployment LAN (akses via IP) maupun online (akses via domain/subdomain), tanpa konfigurasi URL khusus per deployment.
6. WHERE paket berukuran besar (mis. beberapa MB hingga puluhan MB) THE penyajian SHALL efisien untuk diakses oleh banyak siswa secara bersamaan tanpa memuat seluruh berkas ke memori per permintaan.

### Requirement 9: Penjadwalan Ulang Pengiriman Hasil (Shim) Tanpa URL Hardcoded

**User Story:** Sebagai guru, saya ingin mengekspor paket iSpring tanpa harus mengatur URL server, sehingga paket yang sama dapat dipakai di deployment LAN maupun online tanpa modifikasi.

#### Acceptance Criteria

1. WHEN paket iSpring disajikan ke siswa THEN sistem SHALL menyisipkan shim sisi klien yang mengarahkan ulang pengiriman hasil iSpring ke `POST /api/ispring/webhook` pada origin yang sama (URL relatif).
2. WHEN player iSpring mengirim hasil (payload berisi `dr`, `sp`, `tp`) THEN shim SHALL menyuntikkan `attempt_token`, `tenant_id`, dan `sid` (no_id siswa dari konteks sesi) secara otomatis ke payload.
3. THE shim SHALL mencegat mekanisme pengiriman yang dipakai player iSpring (mis. `XMLHttpRequest`, `fetch`, `sendBeacon`, atau submit form) sehingga URL tujuan asli yang ter-embed di dalam paket TIDAK SHALL dipakai.
4. IF pengiriman hasil gagal pada percobaan pertama THEN shim SHALL menampilkan/meneruskan kegagalan secara jelas sehingga siswa/pengawas mengetahui hasil belum terkirim (konsisten dengan perilaku webhook + queue yang sudah ada).
5. THE penyuntikan shim TIDAK SHALL mengubah berkas paket yang tersimpan di disk (paket asli tetap utuh untuk audit); penyuntikan SHALL dilakukan saat penyajian.
6. THE perilaku shim SHALL diverifikasi terhadap fixture paket iSpring asli (mis. `contoh_soal/KIMIA_XII_UAS_2025 (Published)`, versi 11.9.0.4) melalui pengujian otomatis sejauh dapat dilakukan tanpa browser, dan langkah verifikasi manual SHALL didokumentasikan.

### Requirement 10: Penegakan Anti-Cheat di Sisi Server

**User Story:** Sebagai pengawas, saya ingin pelanggaran dan penguncian ditegakkan oleh server, sehingga siswa tidak dapat melewati pembatasan hanya dengan me-refresh halaman.

#### Acceptance Criteria

1. WHEN siswa memicu pelanggaran (mis. berpindah tab/blur) THEN sistem SHALL mencatat hitungan pelanggaran pada sesi aktif di server.
2. WHEN hitungan pelanggaran mencapai ambang batas terkonfigurasi THEN sistem SHALL menandai sesi aktif sebagai terkunci di server (`locked`).
3. WHILE sesi aktif terkunci THE sistem SHALL menolak permintaan melanjutkan ujian dan penyajian konten dengan HTTP 403, terlepas dari state frontend.
4. WHEN pengawas mereset/membuka kunci sesi siswa THEN sistem SHALL menghapus status terkunci dan/atau sesi aktif sehingga siswa dapat masuk kembali sesuai aturan sesi.
5. THE waktu ujian SHALL ditegakkan dari server: WHEN siswa melewati batas waktu efektif THEN pengiriman hasil yang melampaui masa toleransi SHALL ditolak sesuai mekanisme grace period yang sudah ada di processor.
6. THE ambang batas pelanggaran SHALL dapat dikonfigurasi melalui environment dengan nilai default yang wajar.

### Requirement 11: Pemantauan Pengawas Berbasis Sesi

**User Story:** Sebagai pengawas ruang, saya ingin memantau status peserta untuk sesi yang sedang berjalan di ruang saya, sehingga saya dapat menindaklanjuti kendala secara real-time.

#### Acceptance Criteria

1. WHEN pengawas membuka monitor ruang THEN sistem SHALL menampilkan peserta yang relevan dengan ruang dan sesi yang sedang berjalan, beserta status: belum login, sedang mengerjakan (dengan progres), terkunci, dan hasil terkirim.
2. THE tampilan status SHALL membedakan peserta per sesi WHERE lebih dari satu sesi berlangsung yang melibatkan ruang tersebut.
3. WHEN pengawas mereset sesi siswa THEN tindakan SHALL menargetkan sesi aktif yang tepat (tenant + peserta + sesi) dan TIDAK SHALL memengaruhi sesi/peserta lain.
4. THE endpoint pemantauan SHALL hanya dapat diakses role supervisor/admin/superadmin dan menolak role student.
5. THE isolasi tenant SHALL dipertahankan pada seluruh kueri pemantauan.

### Requirement 12: Antarmuka Admin

**User Story:** Sebagai admin, saya ingin antarmuka untuk mengelola tingkatan, ujian, sesi, unggah paket, dan penautan, sehingga seluruh konfigurasi ujian dapat dilakukan tanpa intervensi teknis.

#### Acceptance Criteria

1. WHEN admin membuka manajemen kelas THEN UI SHALL memungkinkan penetapan `tingkat` per kelas.
2. WHEN admin membuka manajemen ujian THEN UI SHALL memungkinkan membuat/menyunting definisi ujian dan menautkan paket soal yang sudah diunggah.
3. WHEN admin membuka manajemen sesi THEN UI SHALL memungkinkan membuat sesi dengan jendela waktu, token (atau pembuatan token otomatis), kelas, dan ruang peserta, serta menampilkan status efektif.
4. WHEN admin membuka manajemen paket soal THEN UI SHALL memungkinkan mengunggah ZIP, melihat daftar paket, dan menghapus paket yang tidak tertaut, dengan umpan balik kemajuan/keberhasilan/kegagalan.
5. THE UI SHALL menggunakan helper API terpusat yang sudah ada (`apiUrl`, `authHeaders`) dan TIDAK SHALL meng-hardcode URL backend atau token tenant.
6. THE UI SHALL menampilkan pesan kesalahan yang jelas dan dapat ditindaklanjuti ketika operasi ditolak server (mis. paket belum ditautkan, token bentrok, waktu tidak valid).

### Requirement 13: Skala & Keandalan untuk 500 Siswa Konkuren

**User Story:** Sebagai penyelenggara, saya ingin sistem tetap andal saat 500 siswa mengakses bersamaan, sehingga ujian tidak gagal pada puncak beban.

#### Acceptance Criteria

1. THE koneksi database SHALL dikonfigurasi secara eksplisit untuk konkurensi: batas koneksi yang sesuai untuk SQLite WAL SHALL ditetapkan pada runtime (saat ini belum ada `SetMaxOpenConns` di jalur server), dengan penulisan diserialkan secara aman dan pembacaan dapat berjalan paralel.
2. WHEN siswa memilih jawaban THEN pelaporan progres TIDAK SHALL menulis ke database pada setiap klik secara sinkron; pembaruan progres SHALL di-debounce/di-batch di klien dan/atau diserap di server sehingga tidak menghasilkan beban tulis yang berlebihan pada skala 500 siswa.
3. THE penyajian konten iSpring (Requirement 8) SHALL menggunakan streaming berkas, bukan memuat seluruh paket ke memori per permintaan.
4. THE pengiriman hasil SHALL tetap melalui filesystem submission queue yang sudah ada sehingga lonjakan pengiriman serempak tidak membebani database secara langsung.
5. THE sistem SHALL menyertakan bukti uji beban yang mensimulasikan login, mulai sesi, pembaruan progres, dan pengiriman hasil pada skala mendekati 500 peserta, memanfaatkan kerangka di `tests/load/`.
6. WHEN uji beban dijalankan THEN tidak SHALL terjadi kehilangan hasil yang sudah diterima webhook, konsisten dengan jaminan keandalan queue (rename atomic + retry + recovery).
7. THE konfigurasi default connection pool (batas koneksi & masa hidup) SHALL bersumber dari satu tempat tunggal; literal default TIDAK SHALL diduplikasi lintas paket (mis. antara `db.DefaultPoolConfig()` dan `config.Load()`) sehingga tidak terjadi drift diam-diam saat salah satu diubah.

### Requirement 14: Migrasi Data & Kompatibilitas Mundur

**User Story:** Sebagai operator yang sudah memiliki data, saya ingin pembaruan ini tidak merusak database yang berjalan, sehingga peningkatan dapat diterapkan dengan aman.

#### Acceptance Criteria

1. THE seluruh migrasi baru SHALL mengikuti pola `RunMigrations` yang berlaku: idempoten, aman dijalankan berulang, dan kompatibel dengan mekanisme yang menelan error "duplicate column name"/"already exists".
2. WHEN migrasi dijalankan pada database lama yang memakai `validasi = tenant_id + no_id + mapel_id` THEN sistem SHALL menyediakan jalur agar hasil lama tetap terbaca, dan perubahan kunci `validasi` ke basis sesi SHALL didokumentasikan beserta dampaknya pada indeks unik `hasil_tes`.
3. WHERE alur token global lama masih dipakai THE sistem SHALL tetap berfungsi hingga admin memindahkan konfigurasi ke model sesi, ATAU menyediakan migrasi data yang membuat sesi awal dari konfigurasi lama (keputusan final pada design).
4. THE perubahan TIDAK SHALL menghapus kolom atau tabel yang masih dipakai fitur berjalan tanpa jalur migrasi yang jelas.
5. WHEN aplikasi dimulai setelah pembaruan THEN seluruh migrasi SHALL berhasil diterapkan pada database kosong maupun database existing tanpa intervensi manual.
6. THE runner migrasi SHALL mengeksekusi tiap berkas migrasi per-pernyataan dan menelan error idempotensi ("duplicate column name"/"already exists") per-pernyataan, sehingga migrasi yang sempat diterapkan sebagian dapat menyembuhkan diri pada eksekusi berulang dan TIDAK SHALL meninggalkan skema tidak lengkap secara diam-diam meskipun `RunMigrations` mengembalikan sukses.
7. THE pengujian idempotensi SHALL memverifikasi keberadaan seluruh objek skema baru (tabel, kolom, indeks — termasuk `idx_cek_login_unique_session` dan `idx_cek_login_content_token`) setelah `RunMigrations` dijalankan ulang, bukan sekadar menegaskan tidak terjadi error.

### Requirement 15: Isolasi Multi-Tenant Menyeluruh

**User Story:** Sebagai penyedia layanan multi-sekolah, saya ingin setiap entitas baru terisolasi per tenant, sehingga data antar sekolah tidak pernah bercampur.

#### Acceptance Criteria

1. THE seluruh tabel baru (tingkatan/ujian/sesi/paket dan tabel relasi) SHALL menyertakan `tenant_id` atau terhubung secara transitif ke tenant melalui induk yang ber-`tenant_id`.
2. THE seluruh kueri terhadap entitas baru SHALL memfilter berdasarkan `tenant_id` konteks permintaan.
3. THE berkas paket iSpring SHALL disimpan terisolasi per tenant di `data/soal/{tenant_slug}/...`.
4. WHEN sebuah permintaan tidak memiliki konteks tenant yang valid di lingkungan produksi THEN sistem SHALL menolak sesuai perilaku `TenantMiddleware` yang berlaku.
5. THE penyajian konten dan pengiriman hasil SHALL memverifikasi kecocokan tenant sebelum mengembalikan atau menyimpan data.

### Requirement 16: Lapisan Repository & Pencegahan God-File

**User Story:** Sebagai pengembang pemelihara, saya ingin kode akses data terstruktur dan modular, sehingga tidak muncul god-file dan migrasi stack di masa depan menjadi layak tanpa menulis ulang handler.

#### Acceptance Criteria

1. THE akses data untuk entitas baru (tingkat/ujian/sesi/paket) SHALL ditempatkan pada lapisan repository terpisah, konsisten dengan pola `internal/repository` yang sudah ada, BUKAN sebagai SQL mentah yang tersebar di handler.
2. THE handler baru SHALL fokus pada urusan HTTP (parsing, validasi input, kode status, serialisasi) dan mendelegasikan logika data ke repository/service.
3. THE tidak SHALL ada satu berkas tunggal yang menggabungkan tanggung jawab unggah, penyajian, penjadwalan, dan pemantauan sekaligus; tanggung jawab SHALL dipisah menjadi unit yang kohesif.
4. THE setiap unit logika baru yang signifikan SHALL memiliki pengujian otomatis yang relevan (unit/integration) sehingga perilaku terverifikasi, bukan diasumsikan.
5. THE seluruh perubahan SHALL lulus `go build ./...`, `go vet ./...`, `go test ./...`, dan `npm run build` pada frontend sebelum dianggap selesai.
6. THE kode TIDAK SHALL menyimpan cabang/kontrak mati yang menyesatkan (mis. "opt-out bila nol" yang tidak terjangkau dari jalur mana pun); komentar dokumentasi SHALL akurat mencerminkan perilaku yang benar-benar terjadi.
7. THE helper pengujian SHALL tidak bermutasi pada state global proses (package-global `DB`, `os.Chdir`) dengan cara yang tidak aman terhadap eksekusi paralel, atau SHALL mendokumentasikan batasan non-paralel secara eksplisit; pengujian konkuren TIDAK SHALL saling merusak.
