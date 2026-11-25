package config

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Config centraliza la configuración inyectada por variables de entorno.
// Nunca se codifican URLs ni credenciales en el código fuente.
type Config struct {
	Port              string
	Version           string
	LogLevel          string
	DatabaseURL       string
	DataSource        string // "mock" | "http"
	URLInventario     string
	URLAlertas        string
	URLReportes       string
	URLPronosticos    string
	UpstreamTimeout   time.Duration
	MaxConcurrentJobs int
	DefaultWorkers    int
	RetryAfterSeconds int
	SeedDemoData      bool
}

func Load() Config {
	defaultWorkers := runtime.NumCPU()
	if defaultWorkers < 2 {
		defaultWorkers = 2
	}
	return Config{
		Port:              getEnv("API_PORT", "8080"),
		Version:           getEnv("SERVICE_VERSION", "1.0.0"),
		LogLevel:          strings.ToLower(getEnv("LOG_LEVEL", "info")),
		DatabaseURL:       getEnv("DATABASE_URL", ""),
		DataSource:        strings.ToLower(getEnv("DATA_SOURCE", "mock")),
		URLInventario:     strings.TrimRight(getEnv("URL_INVENTARIO", ""), "/"),
		URLAlertas:        strings.TrimRight(getEnv("URL_ALERTAS", ""), "/"),
		URLReportes:       strings.TrimRight(getEnv("URL_REPORTES", ""), "/"),
		URLPronosticos:    strings.TrimRight(getEnv("URL_PRONOSTICOS", ""), "/"),
		UpstreamTimeout:   time.Duration(getEnvInt("UPSTREAM_TIMEOUT_SECONDS", 3)) * time.Second,
		MaxConcurrentJobs: getEnvInt("MAX_CONCURRENT_JOBS", 2),
		DefaultWorkers:    getEnvInt("DEFAULT_WORKERS", defaultWorkers),
		RetryAfterSeconds: getEnvInt("RETRY_AFTER_SECONDS", 2),
		SeedDemoData:      getEnvBool("SEED_DEMO_DATA", true),
	}
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

