# Cara Mendapatkan XML Asli dari iSpring QuizMaker

Dokumen ini menjelaskan cara paling **paling efektif dan realistis** untuk mendapatkan file `quizReport` XML asli yang dihasilkan langsung oleh iSpring QuizMaker.

## Mengapa Cara Ini Diperlukan?

Parser Aether CBT (`internal/ispring/parser.go`) sudah diuji dengan XML sintetis. Namun untuk production, kita **membutuhkan XML asli** yang dihasilkan oleh versi iSpring yang benar-benar dipakai sekolah (termasuk namespace, struktur, dan edge case yang mungkin berbeda).

---

## Cara Terbaik: Gunakan Tool Capture Otomatis

Kami sudah menyediakan dua script yang akan:

- Otomatis mengubah URL webhook di quiz menjadi `localhost`
- Menyajikan quiz di browser Anda
- Menangkap XML `quizReport` yang dikirim iSpring saat submit
- Menyimpannya sebagai fixture siap pakai

### Opsi 1: Node.js (Direkomendasikan - Lebih Stabil)

```bash
node scripts/capture-real-ispring-xml.mjs
```

### Opsi 2: PowerShell (Windows Native)

```powershell
.\scripts\capture-real-ispring-xml.ps1
```

---

## Langkah-langkah Lengkap

1. **Jalankan salah satu script di atas** dari root folder proyek.

2. **Buka URL yang muncul** di browser (Chrome atau Edge sangat direkomendasikan):
   ```
   http://localhost:4000/
   ```

3. **Isi form otorisasi** (Authorization Form):
   - Pilih Sekolah
   - Pilih Nama Tes
   - Pilih Kelas
   - Isi No Ujian
   - Isi Nama Lengkap
   - Klik tombol **JAWAB** atau **MULAI**

4. **Jawab beberapa soal** (minimal 5–10 soal). Jawaban boleh sembarangan untuk keperluan testing.

5. **Submit / Finish** kuis.

6. **Lihat hasil di terminal**:
   Script akan otomatis mendeteksi dan menyimpan XML asli dari iSpring.

7. **File akan tersimpan di**:
   ```
   tests/fixtures/ispring/
   ```

   Contoh nama file:
   - `kimia-xii-uas-2025-real-2026-05-25T14-30-22.xml`
   - `kimia-xii-uas-2025-real-2026-05-25T14-30-22.json` (metadata)

---

## Setelah Mendapatkan XML

1. Jalankan test parser:

   ```bash
   go test ./internal/ispring -run TestParseDetailedResults -v
   ```

2. Tambahkan file XML tersebut ke dalam test jika diperlukan.

3. Update dokumentasi jika menemukan edge case baru.

---

## Catatan Penting

- **Jangan gunakan headless browser** (Puppeteer, Playwright, dll) untuk capture ini. iSpring QuizMaker sangat kompleks dan sulit diotomasi.
- Gunakan **browser sungguhan** (Chrome/Edge) agar XML yang dihasilkan 100% asli.
- Satu kali submit sudah cukup untuk mendapatkan satu fixture berkualitas tinggi.
- Anda bisa menjalankan script ini berkali-kali untuk mendapatkan variasi XML (skor berbeda, jawaban berbeda, dll).

---

## Troubleshooting

| Masalah | Solusi |
|---------|--------|
| Port 4000 sudah dipakai | Matikan aplikasi lain atau ubah port di script |
| Quiz tidak load | Pastikan folder `contoh_soal/KIMIA_XII_UAS_2025 (Published)` masih utuh |
| Tidak ada XML yang tertangkap | Pastikan Anda benar-benar klik tombol submit/finish sampai muncul halaman hasil |
| Error di PowerShell | Coba jalankan sebagai Administrator atau gunakan versi Node.js |

---

## Tujuan Akhir

Dengan cara ini kita akan mendapatkan **fixture XML asli** dari iSpring yang benar-benar digunakan sekolah, sehingga parser Aether CBT teruji dengan data produksi sebelum ujian pilot.

Setelah mendapatkan minimal 1–2 file XML asli, lanjutkan ke task berikutnya di roadmap (backup procedure, credential rotation, load testing, dll).

---

**Good luck!** Jalankan script-nya sekarang dan kirim hasil XML yang didapat ke tim. 
