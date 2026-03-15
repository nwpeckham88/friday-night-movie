package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// Init database and run migrations
func Init() error {
	workDir, _ := os.Getwd()
	dbDir := filepath.Join(workDir, "data")
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		os.MkdirAll(dbDir, 0755)
	}

	dbPath := filepath.Join(dbDir, "fnm.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	DB = db

	if err := migrate(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Printf("Database initialized at %s", dbPath)
	return nil
}

func migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT
	);

	CREATE TABLE IF NOT EXISTS state (
		key TEXT PRIMARY KEY,
		value TEXT
	);

	CREATE TABLE IF NOT EXISTS history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		date TEXT
	);

	CREATE TABLE IF NOT EXISTS suggestions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		poster_path TEXT,
		overview TEXT,
		rating REAL,
		is_suggested BOOLEAN,
		status TEXT
	);

	CREATE TABLE IF NOT EXISTS rejected (
		title TEXT PRIMARY KEY
	);
	`
	_, err := DB.Exec(schema)
	return err
}

// Settings methods
func SaveSetting(key, value string) error {
	_, err := DB.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value)
	return err
}

func GetSetting(key string) (string, error) {
	var value string
	err := DB.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// State methods
func SaveStateValue(key, value string) error {
	_, err := DB.Exec("INSERT OR REPLACE INTO state (key, value) VALUES (?, ?)", key, value)
	return err
}

func GetStateValue(key string) (string, error) {
	var value string
	err := DB.QueryRow("SELECT value FROM state WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}
