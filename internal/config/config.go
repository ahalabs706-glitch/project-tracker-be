package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port          string
	Environment   string
	Timezone      string
	DatabaseURL   string
	WebOrigin     string
	JWTSecret     string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	RedisAddress  string
	RedisPassword string
	RedisDB       int
	SMTPHost      string
	SMTPPort      string
	SMTPUsername  string
	SMTPPassword  string
	SMTPFrom      string
}

func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{
		Port:          value(lookup, "APP_PORT", "4000"),
		Environment:   value(lookup, "APP_ENV", "development"),
		Timezone:      value(lookup, "APP_TIMEZONE", "Asia/Jakarta"),
		DatabaseURL:   strings.TrimSpace(environmentValue(lookup, "DATABASE_URL")),
		WebOrigin:     value(lookup, "WEB_ORIGIN", "http://localhost:5173"),
		JWTSecret:     strings.TrimSpace(environmentValue(lookup, "JWT_ACCESS_SECRET")),
		RedisAddress:  value(lookup, "REDIS_ADDR", "localhost:6379"),
		RedisPassword: strings.TrimSpace(environmentValue(lookup, "REDIS_PASSWORD")),
		SMTPHost:      strings.TrimSpace(environmentValue(lookup, "SMTP_HOST")),
		SMTPPort:      value(lookup, "SMTP_PORT", "587"),
		SMTPUsername:  strings.TrimSpace(environmentValue(lookup, "SMTP_USERNAME")),
		SMTPPassword:  strings.TrimSpace(environmentValue(lookup, "SMTP_PASSWORD")),
		SMTPFrom:      strings.TrimSpace(environmentValue(lookup, "SMTP_FROM")),
	}

	if cfg.JWTSecret == "" && !strings.EqualFold(cfg.Environment, "production") {
		cfg.JWTSecret = "local-development-only-change-this-secret"
	}
	if len(cfg.JWTSecret) < 32 || (strings.EqualFold(cfg.Environment, "production") &&
		cfg.JWTSecret == "local-development-only-change-this-secret") {
		return Config{}, fmt.Errorf("JWT_ACCESS_SECRET must be at least 32 characters and must not use a development secret in production")
	}
	accessTTL, err := time.ParseDuration(value(lookup, "JWT_ACCESS_TTL", "15m"))
	if err != nil || accessTTL <= 0 {
		return Config{}, fmt.Errorf("JWT_ACCESS_TTL must be a positive Go duration")
	}
	refreshTTL, err := time.ParseDuration(value(lookup, "JWT_REFRESH_TTL", "168h"))
	if err != nil || refreshTTL <= accessTTL {
		return Config{}, fmt.Errorf("JWT_REFRESH_TTL must be a Go duration greater than JWT_ACCESS_TTL")
	}
	cfg.AccessTTL = accessTTL
	cfg.RefreshTTL = refreshTTL
	redisDB, err := strconv.Atoi(value(lookup, "REDIS_DB", "0"))
	if err != nil || redisDB < 0 || redisDB > 15 {
		return Config{}, fmt.Errorf("REDIS_DB must be an integer between 0 and 15")
	}
	cfg.RedisDB = redisDB
	if strings.EqualFold(cfg.Environment, "production") &&
		(cfg.RedisPassword == "" || cfg.RedisPassword == "local-redis-development-only") {
		return Config{}, fmt.Errorf("REDIS_PASSWORD must be set to a unique value in production")
	}

	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("APP_PORT must be an integer between 1 and 65535")
	}
	if cfg.Environment == "" {
		return Config{}, fmt.Errorf("APP_ENV must not be empty")
	}
	if cfg.Timezone == "" {
		return Config{}, fmt.Errorf("APP_TIMEZONE must not be empty")
	}
	if err := validateDatabaseURL(cfg.DatabaseURL); err != nil {
		return Config{}, err
	}
	if err := validateWebOrigin(cfg.WebOrigin); err != nil {
		return Config{}, err
	}
	if err := validateRedisAddress(cfg.RedisAddress); err != nil {
		return Config{}, err
	}
	if err := validateSMTP(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validateRedisAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || strings.TrimSpace(host) == "" {
		return fmt.Errorf("REDIS_ADDR must be a host:port address")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("REDIS_ADDR must contain a valid port")
	}
	return nil
}

func validateSMTP(cfg Config) error {
	configured := cfg.SMTPHost != "" || cfg.SMTPUsername != "" || cfg.SMTPPassword != "" || cfg.SMTPFrom != ""
	if !configured {
		return nil
	}
	if cfg.SMTPHost == "" || cfg.SMTPFrom == "" {
		return fmt.Errorf("SMTP_HOST and SMTP_FROM are required when SMTP is configured")
	}
	port, err := strconv.Atoi(cfg.SMTPPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("SMTP_PORT must be an integer between 1 and 65535")
	}
	if (cfg.SMTPUsername == "") != (cfg.SMTPPassword == "") {
		return fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be configured together")
	}
	return nil
}

func value(lookup func(string) (string, bool), key, fallback string) string {
	if value := strings.TrimSpace(environmentValue(lookup, key)); value != "" {
		return value
	}
	return fallback
}

func environmentValue(lookup func(string) (string, bool), key string) string {
	value, _ := lookup(key)
	return value
}

func validateDatabaseURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" {
		return fmt.Errorf("DATABASE_URL must be a valid PostgreSQL URL")
	}
	if port := parsed.Port(); port != "" {
		if _, err := strconv.Atoi(port); err != nil {
			return fmt.Errorf("DATABASE_URL must contain a valid port")
		}
	}
	return nil
}

func validateWebOrigin(rawOrigin string) error {
	parsed, err := url.Parse(rawOrigin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("WEB_ORIGIN must be an HTTP(S) origin without a path")
	}
	if _, _, err := net.SplitHostPort(parsed.Host); err != nil && strings.Contains(parsed.Host, ":") {
		return fmt.Errorf("WEB_ORIGIN must contain a valid host and port")
	}
	return nil
}
