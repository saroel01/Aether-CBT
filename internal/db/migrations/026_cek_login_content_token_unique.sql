-- Jadikan content_token sebagai capability key yang dijamin unik 1:1 ke satu baris
-- cek_login (satu sesi aktif). Ini menutup celah defense-in-depth: lookup penyajian
-- konten (GetByContentToken) memakai token sebagai satu-satunya otoritas, jadi token
-- tak boleh dimiliki dua sesi sekaligus. Token dibangkitkan 256-bit crypto/rand sehingga
-- tabrakan praktis mustahil, namun indeks unik menegaskan invarian ini di lapisan data
-- (bukan sekadar andalkan keacakan). Indeks parsial mengecualikan baris content_token
-- NULL (sesi yang belum memulai konten) agar tidak saling bertabrakan.
CREATE UNIQUE INDEX IF NOT EXISTS idx_cek_login_content_token_unique
    ON cek_login(content_token) WHERE content_token IS NOT NULL;
