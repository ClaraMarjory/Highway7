package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	DataDir string
	DBPath  string
}

func Load() *Config {
	// Data stored alongside binary
	exe, _ := os.Executable()
	dataDir := filepath.Join(filepath.Dir(exe), "data")
	os.MkdirAll(dataDir, 0755)

	return &Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "highway.db"),
	}
}

func DataDir() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "data")
}

func DBPath() string {
	return filepath.Join(DataDir(), "highway.db")
}
