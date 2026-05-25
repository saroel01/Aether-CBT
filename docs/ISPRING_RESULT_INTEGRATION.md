# iSpring Result Integration

Dokumen ini adalah rujukan final untuk penerimaan hasil iSpring di Aether CBT.

## Status Implementasi

Aether CBT menerima hasil kuis melalui `POST /api/ispring/webhook`. Endpoint ini menerima parameter standar iSpring, memvalidasi bahwa siswa masih memiliki sesi aktif di `cek_login`, menolak kiriman yang melewati masa toleransi waktu, menyimpan XML mentah dari `dr`, dan memecah jawaban per soal ke `hasil_tes_detail`.

Parser detail iSpring berada di `internal/ispring`. Parser ini membaca bentuk `quizReport` iSpring, termasuk XML dengan namespace `http://www.ispringsolutions.com/ispring/quizbuilder/quizresults`.

## Parameter POST Yang Dipakai

| Parameter | Fungsi |
| --- | --- |
| `sid` | Nomor peserta. Diprioritaskan untuk mencocokkan `peserta.no_id`. |
| `USER_NAME` | Fallback nomor peserta jika `sid` tidak dikirim. |
| `sp` | Skor yang diperoleh. |
| `tp` | Skor maksimum/total poin. |
| `dr` | XML detail hasil iSpring. Disimpan mentah dan diparse untuk analisis soal. |
| `attempt_token` | Token per sesi ujian yang diterbitkan oleh `POST /api/student/start`. Wajib cocok dengan `cek_login.attempt_token`. |
| `v`, `qt`, `t`, `ps`, `psp`, `ut`, `fut` | Diterima sebagai bagian format iSpring, tetapi belum semuanya disimpan sebagai kolom terpisah. |

## Bentuk XML Detail

`dr` yang kompatibel menggunakan root `quizReport`, bukan format internal seperti `<report>` atau `<quiz>`.

```xml
<quizReport xmlns="http://www.ispringsolutions.com/ispring/quizbuilder/quizresults" version="9">
  <quizSettings>
    <passingPercent>70</passingPercent>
  </quizSettings>
  <summary passed="true" percent="85" finishTimestamp="2026-05-25T09:00:00Z" />
  <questions>
    <multipleChoiceQuestion id="q1" evaluationEnabled="true" maxPoints="10" awardedPoints="10" status="correct">
      <direction><text>Contoh soal?</text></direction>
      <answers correctAnswerIndex="0" userAnswerIndex="0">
        <answer><text>Jawaban benar</text></answer>
      </answers>
    </multipleChoiceQuestion>
  </questions>
  <groups />
</quizReport>
```

## Tipe Soal Yang Diparse

Parser mendukung tipe utama berikut:

- `trueFalseQuestion`
- `multipleChoiceQuestion`
- `multipleResponseQuestion`
- `matchingQuestion`
- `sequenceQuestion`
- `typeInQuestion`
- `fillInTheBlankQuestion`
- `fillInTheBlankQuestionEx`
- `essayQuestion`
- `multipleChoiceTextQuestion`
- `wordBankQuestion`
- `numericQuestion`
- `dndQuestion`

Tipe yang belum dikenal tetap tidak boleh dianggap sebagai jawaban valid otomatis. Parser menyimpan fallback sebatas jawaban teks atau atribut `userAnswer` bila tersedia.

## Aturan Penyimpanan

- `hasil_tes.validasi` menggunakan format `tenant_id + no_id + mapel_id`.
- `hasil_tes(tenant_id, validasi)` memiliki unique index agar satu peserta hanya memiliki satu hasil final per mata pelajaran dalam satu tenant.
- `cek_login(tenant_id, peserta_id, mapel_id)` memiliki unique index agar sesi aktif tidak dobel.
- `cek_login.attempt_token` menyimpan rahasia per sesi. Kiriman hasil tanpa token ini ditolak dengan HTTP `403`.
- Setelah hasil diterima, sesi aktif siswa dihapus dari `cek_login`.

## Batasan Saat Ini

Aether CBT belum melakukan validasi XSD penuh seperti sample PHP iSpring. Parser memakai kontrak struktur XML yang diuji otomatis, dan XML invalid ditolak dengan HTTP `400`. Untuk deployment sekolah, fixture acceptance test dari output iSpring QuizMaker asli yang dipakai sekolah tetap wajib disiapkan sebelum hari ujian.
