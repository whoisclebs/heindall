package app

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port              int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	BodyLimitBytes    int64
	IndexPath         string
	ReferencesPath    string
	ANNMinCandidates  int
}

func LoadConfig() Config {
	return Config{
		Port:              getenvInt("PORT", 8080),
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      2 * time.Second,
		IdleTimeout:       30 * time.Second,
		BodyLimitBytes:    int64(getenvInt("BODY_LIMIT_BYTES", 16<<10)),
		IndexPath:         os.Getenv("INDEX_PATH"),
		ReferencesPath:    getenv("REFERENCES_PATH", "resources/references.json.gz"),
		ANNMinCandidates:  getenvInt("ANN_MIN_CANDIDATES", 128),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
