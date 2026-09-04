package httpapi

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the server's runtime configuration, read from the environment.
type Config struct {
	Addr            string
	DatabaseURL     string
	CORSOrigins     []string
	RateLimitPerMin int
	// TrustProxy makes the rate limiter believe X-Forwarded-For. Only set it
	// where a proxy actually rewrites that header, or the limit is bypassable
	// by anyone willing to send one.
	TrustProxy      bool
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// LoadConfig reads and validates the environment. Every problem is reported at
// once: fixing a deployment one variable per restart is miserable.
func LoadConfig() (Config, error) {
	cfg := Config{
		Addr:            envOr("API_ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		CORSOrigins:     splitOrigins(envOr("API_CORS_ORIGINS", "http://localhost:5173")),
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     2 * time.Minute,
		ShutdownTimeout: 20 * time.Second,
	}

	var errs []error
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is not set"))
	}

	limit, err := envInt("API_RATE_LIMIT", 60)
	if err != nil {
		errs = append(errs, err)
	} else if limit < 1 {
		errs = append(errs, fmt.Errorf("API_RATE_LIMIT must be at least 1, got %d", limit))
	}
	cfg.RateLimitPerMin = limit

	trust, err := envBool("API_TRUST_PROXY", false)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.TrustProxy = trust

	if len(cfg.CORSOrigins) == 0 {
		errs = append(errs, errors.New("API_CORS_ORIGINS is empty; set at least one origin"))
	}
	for _, o := range cfg.CORSOrigins {
		if o == "*" {
			errs = append(errs, errors.New(`API_CORS_ORIGINS may not be "*"; name the frontend origin`))
		}
	}

	return cfg, errors.Join(errs...)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number, got %q", key, raw)
	}
	return n, nil
}

func envBool(key string, def bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, got %q", key, raw)
	}
	return b, nil
}

func splitOrigins(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
