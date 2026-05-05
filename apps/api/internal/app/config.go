package app

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const embeddedIndexPath = "/data/index.heindall.ivf8192.bin"

type Config struct {
	Port              int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	BodyLimitBytes    int64
	SocketPath        string
	IndexPath         string
	ReferencesPath    string
	ANNNProbe         int
	ANNAmbiguousProbe int
	ANNRepair         bool
}

func LoadConfig() Config {
	return Config{
		Port:              getenvInt("PORT", 8080),
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      2 * time.Second,
		IdleTimeout:       30 * time.Second,
		BodyLimitBytes:    int64(getenvInt("BODY_LIMIT_BYTES", 16<<10)),
		SocketPath:        getenv("SOCKET_PATH", ""),
		IndexPath:         getenv("INDEX_PATH", embeddedIndexPath),
		ReferencesPath:    getenv("REFERENCES_PATH", "resources/references.json.gz"),
		ANNNProbe:         getenvInt("ANN_NPROBE", 8),
		ANNAmbiguousProbe: getenvInt("ANN_AMBIGUOUS_NPROBE", 32),
		ANNRepair:         getenvBool("ANN_REPAIR", true),
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

func getenvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}
