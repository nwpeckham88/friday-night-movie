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
		tmdb_id INTEGER,
		title TEXT,
		year INTEGER,
		overview TEXT,
		poster_path TEXT,
		rating REAL,
		trailer_key TEXT,
		reasoning TEXT,
		path_theme TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
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

// Suggestions methods
type DBSuggestion struct {
	ID          int     `json:"id"`
	TMDBID      int     `json:"tmdb_id"`
	Title       string  `json:"title"`
	Year        int     `json:"year"`
	Overview    string  `json:"overview"`
	PosterPath  string  `json:"poster_path"`
	Rating      float64 `json:"rating"`
	TrailerKey  string  `json:"trailer_key"`
	Reasoning   string  `json:"reasoning"`
	PathTheme   string  `json:"path_theme"`
	CreatedAt   string  `json:"created_at"`
}

func SaveSuggestion(s DBSuggestion) error {
	_, err := DB.Exec(`
		INSERT INTO suggestions (tmdb_id, title, year, overview, poster_path, rating, trailer_key, reasoning, path_theme)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.TMDBID, s.Title, s.Year, s.Overview, s.PosterPath, s.Rating, s.TrailerKey, s.Reasoning, s.PathTheme)
	return err
}

func GetSuggestions() ([]DBSuggestion, error) {
	rows, err := DB.Query("SELECT id, tmdb_id, title, year, overview, poster_path, rating, trailer_key, reasoning, path_theme, created_at FROM suggestions ORDER BY created_at DESC LIMIT 50")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var suggestions []DBSuggestion
	for rows.Next() {
		var s DBSuggestion
		err := rows.Scan(&s.ID, &s.TMDBID, &s.Title, &s.Year, &s.Overview, &s.PosterPath, &s.Rating, &s.TrailerKey, &s.Reasoning, &s.PathTheme, &s.CreatedAt)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, s)
	}
	return suggestions, nil
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}
