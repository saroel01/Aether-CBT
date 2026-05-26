package db

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Connect(databasePath string) error {
	var err error
	DB, err = sql.Open("sqlite", databasePath+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		return err
	}

	log.Println("Database connected successfully (SQLite + WAL)")
	return nil
}

func Close() error {
	return DB.Close()
}
