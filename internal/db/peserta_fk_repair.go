package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// Gate 0 schema prerequisite for codebase-bug-sweep clause 2.2.
//
// peserta.kelas_id and peserta.ruang_id are FOREIGN KEYs to kelas(id) / ruang(id), and both
// were originally declared NOT NULL. That combination has no way to say "not assigned": the
// write paths therefore stored the integer 0, which no parent row carries, so every such row
// is a referential-integrity violation waiting for foreign_keys to be enabled (clause 1.1).
//
// Migration 009 now declares both columns nullable, which fixes fresh installs. Existing
// databases need the column constraint relaxed, and SQLite has no ALTER COLUMN — it takes the
// standard table rebuild. That rebuild needs three things a plain statement runner cannot
// give it, which is why it lives in Go rather than in a .sql file:
//
//  1. A single connection. PRAGMA foreign_keys is per-connection, and a *sql.DB hands out
//     pooled connections, so pragma state set by one statement is not guaranteed to apply to
//     the next.
//  2. foreign_keys OFF for the duration. With enforcement on, DROP TABLE peserta performs an
//     implicit DELETE that trips the FKs in hasil_tes / cek_login, and ALTER TABLE RENAME
//     rewrites those child REFERENCES clauses to point at the temporary table name.
//  3. A transaction. A half-finished rebuild would lose every participant row.
//
// The step is conditional and idempotent: it no-ops when peserta does not exist yet (fresh
// install) or already has nullable reference columns (already repaired).

// pesertaRebuildTable is the temporary name the rebuilt table is created under.
const pesertaRebuildTable = "peserta_fk_repair_tmp"

// pesertaExpectedColumns is the exact column set this rebuild knows how to reproduce, in
// declaration order. If a future migration adds a column without updating pesertaRebuildDDL,
// the rebuild refuses to run rather than silently dropping data.
var pesertaExpectedColumns = []string{
	"id", "tenant_id", "no_id", "password", "nama_peserta", "kelas_id",
	"jenis_kelamin", "ruang_id", "foto", "created_at", "updated_at", "deleted_at",
}

// pesertaRebuildDDL mirrors migration 009 exactly, except that kelas_id and ruang_id are
// nullable. The temporary name is substituted at build time.
const pesertaRebuildDDL = `CREATE TABLE %s (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    no_id TEXT NOT NULL,
    password TEXT NOT NULL,
    nama_peserta TEXT NOT NULL,
    kelas_id INTEGER,
    jenis_kelamin TEXT CHECK(jenis_kelamin IN ('L', 'P')),
    ruang_id INTEGER,
    foto TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (kelas_id) REFERENCES kelas(id),
    FOREIGN KEY (ruang_id) REFERENCES ruang(id),
    UNIQUE(tenant_id, no_id)
)`

// pesertaRebuildIndexes recreates the indexes migration 009 declares. They are dropped along
// with the old table, so they must be re-created inside the same transaction.
var pesertaRebuildIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_peserta_tenant ON peserta(tenant_id)`,
	`CREATE INDEX IF NOT EXISTS idx_peserta_no_id ON peserta(tenant_id, no_id)`,
	`CREATE INDEX IF NOT EXISTS idx_peserta_ruang ON peserta(ruang_id)`,
}

// repairPesertaNullableFK makes peserta.kelas_id / peserta.ruang_id nullable on databases that
// still declare them NOT NULL. It is a no-op on fresh and already-repaired databases.
func repairPesertaNullableFK(database *sql.DB) error {
	ctx := context.Background()

	columns, err := tableColumns(ctx, database, "peserta")
	if err != nil {
		return fmt.Errorf("inspect peserta columns: %w", err)
	}
	if len(columns) == 0 {
		return nil // fresh database: migration 009 creates the nullable shape directly
	}
	if !columns["kelas_id"].notNull && !columns["ruang_id"].notNull {
		return nil // already nullable
	}

	// Refuse rather than guess if the table is not the shape this rebuild reproduces.
	if missing, extra := diffColumnSet(columns, pesertaExpectedColumns); len(missing) > 0 || len(extra) > 0 {
		log.Printf("WARNING: skipping the peserta foreign-key repair: unexpected columns "+
			"(missing: %v, extra: %v). peserta.kelas_id / ruang_id stay NOT NULL, so a student "+
			"without a class or room cannot be stored and PRAGMA foreign_key_check may report "+
			"sentinel rows. Update pesertaRebuildDDL in internal/db/peserta_fk_repair.go.",
			missing, extra)
		return nil
	}

	conn, err := database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire dedicated connection for peserta repair: %w", err)
	}
	defer conn.Close()

	// PRAGMA foreign_keys is per-connection and a no-op inside a transaction, so it is set
	// before BEGIN and restored before the connection goes back to the pool.
	var previousFK int
	if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&previousFK); err != nil {
		return fmt.Errorf("read foreign_keys pragma: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = off`); err != nil {
		return fmt.Errorf("disable foreign_keys for peserta repair: %w", err)
	}
	restoreFK := func() {
		if previousFK != 1 {
			return
		}
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = on`); err != nil {
			log.Printf("WARNING: could not restore foreign_keys after the peserta repair: %v", err)
		}
	}

	if err := rebuildPesertaInTx(ctx, conn); err != nil {
		restoreFK()
		return err
	}
	restoreFK()

	log.Printf("Repaired peserta schema: kelas_id and ruang_id are now nullable (codebase-bug-sweep clause 2.2)")
	return nil
}

// rebuildPesertaInTx performs the rebuild itself, atomically, on the given connection.
func rebuildPesertaInTx(ctx context.Context, conn *sql.Conn) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin peserta repair transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit succeeded

	columnList := strings.Join(pesertaExpectedColumns, ", ")
	steps := []string{
		fmt.Sprintf(`DROP TABLE IF EXISTS %s`, pesertaRebuildTable),
		fmt.Sprintf(pesertaRebuildDDL, pesertaRebuildTable),
		fmt.Sprintf(`INSERT INTO %s (%s) SELECT %s FROM peserta`, pesertaRebuildTable, columnList, columnList),
		`DROP TABLE peserta`,
		fmt.Sprintf(`ALTER TABLE %s RENAME TO peserta`, pesertaRebuildTable),
	}
	steps = append(steps, pesertaRebuildIndexes...)

	for _, stmt := range steps {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("peserta repair step failed (%s): %w", firstLine(stmt), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit peserta repair: %w", err)
	}
	return nil
}

// columnInfo is the subset of pragma_table_info this package needs.
type columnInfo struct {
	notNull bool
}

// tableColumns returns the columns of the given table, or an empty map when it does not exist.
func tableColumns(ctx context.Context, database *sql.DB, table string) (map[string]columnInfo, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT name, "notnull" FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]columnInfo)
	for rows.Next() {
		var name string
		var notNull int
		if err := rows.Scan(&name, &notNull); err != nil {
			return nil, err
		}
		out[name] = columnInfo{notNull: notNull != 0}
	}
	return out, rows.Err()
}

// diffColumnSet compares actual columns against the expected list.
func diffColumnSet(actual map[string]columnInfo, expected []string) (missing, extra []string) {
	expectedSet := make(map[string]struct{}, len(expected))
	for _, name := range expected {
		expectedSet[name] = struct{}{}
		if _, ok := actual[name]; !ok {
			missing = append(missing, name)
		}
	}
	for name := range actual {
		if _, ok := expectedSet[name]; !ok {
			extra = append(extra, name)
		}
	}
	return missing, extra
}

// firstLine trims a statement to its first line for error messages.
func firstLine(stmt string) string {
	if i := strings.IndexByte(stmt, '\n'); i >= 0 {
		return stmt[:i]
	}
	return stmt
}
