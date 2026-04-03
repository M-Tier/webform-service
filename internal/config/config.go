package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// Server settings
	Port    string
	DevMode bool // Allows localhost origins for any site

	// SMTP settings
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string

	// Security settings
	RateLimitPerHour   int
	MinFormTimeSeconds int

	// Redis settings (optional - empty means in-memory only)
	RedisURL string

	// Sites configuration
	SitesConfigPath string
}

func Load() (*Config, error) {
	smtpPort, err := getEnvInt("SMTP_PORT", 587)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT: %w", err)
	}

	rateLimitPerHour, err := getEnvInt("RATE_LIMIT_PER_HOUR", 3)
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_PER_HOUR: %w", err)
	}

	minFormTime, err := getEnvInt("MIN_FORM_TIME_SECONDS", 3)
	if err != nil {
		return nil, fmt.Errorf("invalid MIN_FORM_TIME_SECONDS: %w", err)
	}

	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		DevMode:            getEnvBool("DEV_MODE", false),
		SMTPHost:           getEnv("SMTP_HOST", ""),
		SMTPPort:           smtpPort,
		SMTPUser:           getEnv("SMTP_USER", ""),
		SMTPPass:           getEnv("SMTP_PASS", ""),
		RateLimitPerHour:   rateLimitPerHour,
		MinFormTimeSeconds: minFormTime,
		RedisURL:           getEnv("REDIS_URL", ""),
		SitesConfigPath:    getEnv("SITES_CONFIG_PATH", "./sites.json"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.SMTPHost == "" {
		return fmt.Errorf("SMTP_HOST is required")
	}
	if c.SMTPUser == "" {
		return fmt.Errorf("SMTP_USER is required")
	}
	if c.SMTPPass == "" {
		return fmt.Errorf("SMTP_PASS is required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) (int, error) {
	if value := os.Getenv(key); value != "" {
		return strconv.Atoi(value)
	}
	return defaultValue, nil
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		lower := strings.ToLower(value)
		return lower == "true" || lower == "1" || lower == "yes"
	}
	return defaultValue
}
