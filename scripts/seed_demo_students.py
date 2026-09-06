import sqlite3

db_path = 'release-test/data/cbt_aether.db'
conn = sqlite3.connect(db_path)
cur = conn.cursor()

# Exact bcrypt hash for "siswa123"
password_hash = '$2a$14$Yna6OCXEZW12zmdWfce4UOgsHu1Ba1/N0S.5rgrFovgMch1IIzEWO'
tenant_id = 1

students = [
    ("2026001", "Ahmad Fauzi", 1, 1, "L"),
    ("2026002", "Budi Santoso", 1, 1, "L"),
    ("2026003", "Citra Lestari", 1, 1, "P"),
    ("2026004", "Dewi Anggraini", 1, 1, "P"),
    ("2026005", "Eko Prasetyo", 1, 1, "L"),
    ("2026006", "Fajar Hidayat", 1, 2, "L"),
    ("2026007", "Gita Permata", 1, 2, "P"),
    ("2026008", "Hadi Saputra", 1, 2, "L"),
    ("2026009", "Intan Rahmawati", 1, 2, "P"),
    ("2026010", "Joko Widodo", 1, 2, "L"),
]

for no_id, nama, kelas_id, ruang_id, jk in students:
    cur.execute("""
        UPDATE peserta
        SET password = ?, nama_peserta = ?, kelas_id = ?, ruang_id = ?, jenis_kelamin = ?, updated_at = datetime('now')
        WHERE no_id = ? AND tenant_id = ?
    """, (password_hash, nama, kelas_id, ruang_id, jk, no_id, tenant_id))
    if cur.rowcount == 0:
        cur.execute("""
            INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id, jenis_kelamin, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
        """, (tenant_id, no_id, password_hash, nama, kelas_id, ruang_id, jk))

# Ensure exam_session 1 (token AETHER1) is active and wide open
cur.execute("""
    UPDATE exam_session
    SET waktu_mulai = '2026-01-01 00:00:00',
        waktu_selesai = '2030-12-31 23:59:59',
        status = 'aktif',
        updated_at = datetime('now')
    WHERE id = 1 AND tenant_id = ?
""", (tenant_id,))

conn.commit()

print("Verification of passwords in DB:")
for r in cur.execute("SELECT no_id, password, nama_peserta, kelas_id, ruang_id FROM peserta").fetchall():
    print(r)

conn.close()
