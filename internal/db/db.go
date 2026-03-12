package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func Init() error {
	exe, _ := os.Executable()
	dataDir := filepath.Join(filepath.Dir(exe), "data")
	os.MkdirAll(dataDir, 0755)
	dbPath := filepath.Join(dataDir, "highway.db")

	var err error
	DB, err = sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	return migrate()
}

func migrate() error {
	sqls := []string{
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS servers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			host TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 22,
			user TEXT NOT NULL DEFAULT 'root',
			auth_type TEXT NOT NULL DEFAULT 'key',
			auth_value TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'landing',
			status TEXT NOT NULL DEFAULT 'unknown',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS forwards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			server_id INTEGER NOT NULL,
			listen_port INTEGER NOT NULL,
			target_host TEXT NOT NULL,
			target_port INTEGER NOT NULL,
			protocol TEXT NOT NULL DEFAULT 'tcp',
			status TEXT NOT NULL DEFAULT 'inactive',
			bytes_up INTEGER NOT NULL DEFAULT 0,
			bytes_down INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (server_id) REFERENCES servers(id)
		)`,
		`CREATE TABLE IF NOT EXISTS ss_nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			server_id INTEGER NOT NULL,
			port INTEGER NOT NULL,
			password TEXT NOT NULL,
			method TEXT NOT NULL DEFAULT 'none',
			status TEXT NOT NULL DEFAULT 'inactive',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (server_id) REFERENCES servers(id)
		)`,
	}

	for _, s := range sqls {
		if _, err := DB.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}

func hashPassword(pass string) string {
	h := sha256.Sum256([]byte(pass))
	return hex.EncodeToString(h[:])
}

func SetAdminPassword(pass string) error {
	hashed := hashPassword(pass)
	_, err := DB.Exec(
		`INSERT INTO settings (key, value) VALUES ('admin_pass', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		hashed,
	)
	return err
}

func HasAdminPassword() bool {
	var count int
	DB.QueryRow(`SELECT COUNT(*) FROM settings WHERE key = 'admin_pass'`).Scan(&count)
	return count > 0
}

func CheckAdminPassword(pass string) bool {
	hashed := hashPassword(pass)
	var stored string
	err := DB.QueryRow(`SELECT value FROM settings WHERE key = 'admin_pass'`).Scan(&stored)
	if err != nil {
		return false
	}
	return hashed == stored
}
