-- Relasi sesi ujian ke kelas dan ruang peserta.
CREATE TABLE IF NOT EXISTS exam_session_kelas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    kelas_id INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES exam_session(id) ON DELETE CASCADE,
    FOREIGN KEY (kelas_id) REFERENCES kelas(id),
    UNIQUE(session_id, kelas_id)
);

CREATE TABLE IF NOT EXISTS exam_session_ruang (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    ruang_id INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES exam_session(id) ON DELETE CASCADE,
    FOREIGN KEY (ruang_id) REFERENCES ruang(id),
    UNIQUE(session_id, ruang_id)
);

CREATE INDEX IF NOT EXISTS idx_session_kelas_session ON exam_session_kelas(session_id);
CREATE INDEX IF NOT EXISTS idx_session_ruang_session ON exam_session_ruang(session_id);
