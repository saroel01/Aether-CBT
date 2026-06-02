package db

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// PoolConfig holds the SQLite connection pool tuning parameters. It is defined
// here (rather than importing the config package) to keep the db package free of
// configuration dependencies and avoid import cycles.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DefaultPoolConfig returns conservative pool settings suitable for CLI tools and
// tests where explicit configuration is not provided.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
	}
}

// Connect opens the SQLite database (WAL mode, foreign keys on, busy timeout) and
// applies the connection pool configuration. Explicit pool limits are required for
// reliable concurrency at scale (Requirement 13.1): WAL permits many parallel
// readers with a single writer, and _busy_timeout serializes writes safely.
func Connect(databasePath string, pool PoolConfig) error {
	var err error
	DB, err = sql.Open("sqlite", databasePath+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return err
	}

	applyPoolConfig(DB, pool)

	if err = DB.Ping(); err != nil {
		return err
	}

	log.Printf("Database connected successfully (SQLite + WAL, max_open=%d, max_idle=%d)",
		pool.MaxOpenConns, pool.MaxIdleConns)
	return nil
}

// applyPoolConfig applies pool limits, ignoring non-positive values so callers can
// opt out of a particular limit by leaving it at zero.
func applyPoolConfig(database *sql.DB, pool PoolConfig) {
	if pool.MaxOpenConns > 0 {
		database.SetMaxOpenConns(pool.MaxOpenConns)
	}
	if pool.MaxIdleConns > 0 {
		database.SetMaxIdleConns(pool.MaxIdleConns)
	}
	if pool.ConnMaxLifetime > 0 {
		database.SetConnMaxLifetime(pool.ConnMaxLifetime)
	}
}

func Close() error {
	return DB.Close()
}
