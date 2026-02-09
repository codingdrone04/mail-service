package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port            string
	AllowedOrigins  []string
	SMTPHost        string
	SMTPPort        string
	SMTPUser        string
	SMTPPassword    string
	MailFrom        string
	MailTo          string
	RateLimitPerMin int
}

func Load() Config {
	cfg := Config{
		Port:            getEnvOrDefault("SERVER_PORT", "8080"),
		AllowedOrigins:  parseOrigins(getEnvOrDefault("CORS_ALLOWED_ORIGINS", "*")),
		SMTPHost:        requireEnv("SMTP_HOST"),
		SMTPPort:        getEnvOrDefault("SMTP_PORT", "587"),
		SMTPUser:        requireEnv("SMTP_USER"),
		SMTPPassword:    requireEnv("SMTP_PASSWORD"),
		MailFrom:        requireEnv("MAIL_FROM"),
		MailTo:          requireEnv("MAIL_TO"),
		RateLimitPerMin: getEnvAsInt("RATE_LIMIT_PER_MINUTE", 5),
	}
	return cfg
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return val
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}

func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}
