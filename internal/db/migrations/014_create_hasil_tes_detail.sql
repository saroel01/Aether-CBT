-- +goose Up
CREATE TABLE IF NOT EXISTS hasil_tes_detail (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hasil_tes_id INTEGER NOT NULL,
    question_id TEXT NOT NULL,
    question_text TEXT,
    question_type TEXT,
    status TEXT,                            -- correct / incorrect / partial
    awarded_points REAL,
    max_points REAL,
    user_answer TEXT,
    correct_answer TEXT,
    attempts_used INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (hasil_tes_id) REFERENCES hasil_tes(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_detail_hasil ON hasil_tes_detail(hasil_tes_id);
