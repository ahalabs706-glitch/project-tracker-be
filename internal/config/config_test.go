package config

import (
	"testing"
)

func TestLoadUsesDevelopmentDefaults(t *testing.T) {
	cfg, err := load(mapLookup(map[string]string{
		"DATABASE_URL": "postgres://user:pass@localhost:5432/projectflow?sslmode=disable",
	}))
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}

	if cfg.Port != "4000" || cfg.Environment != "development" ||
		cfg.Timezone != "Asia/Jakarta" || cfg.WebOrigin != "http://localhost:5173" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
	}{
		{
			name: "invalid port",
			values: map[string]string{
				"APP_PORT":     "70000",
				"DATABASE_URL": "postgres://user:pass@localhost:5432/projectflow",
			},
		},
		{
			name: "missing database URL",
			values: map[string]string{
				"APP_PORT": "4000",
			},
		},
		{
			name: "invalid web origin",
			values: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost:5432/projectflow",
				"WEB_ORIGIN":   "https://example.com/app",
			},
		},
		{
			name: "production requires a secret",
			values: map[string]string{
				"APP_ENV":      "production",
				"DATABASE_URL": "postgres://user:pass@localhost:5432/projectflow",
			},
		},
		{
			name: "access token secret is too short",
			values: map[string]string{
				"JWT_ACCESS_SECRET": "too-short",
				"DATABASE_URL":      "postgres://user:pass@localhost:5432/projectflow",
			},
		},
		{
			name: "refresh token lifetime must exceed access lifetime",
			values: map[string]string{
				"DATABASE_URL":    "postgres://user:pass@localhost:5432/projectflow",
				"JWT_ACCESS_TTL":  "1h",
				"JWT_REFRESH_TTL": "30m",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := load(mapLookup(test.values)); err == nil {
				t.Fatal("expected invalid configuration to fail")
			}
		})
	}
}

func TestLoadNotificationConfiguration(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL": "postgres://localhost:5432/projectflow?sslmode=disable",
	}
	cfg, err := load(mapLookup(values))
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}
	if cfg.RedisAddress != "localhost:6379" || cfg.RedisDB != 0 || cfg.SMTPPort != "587" {
		t.Fatalf("unexpected notification defaults: %+v", cfg)
	}

	for _, update := range []map[string]string{
		{"REDIS_ADDR": "redis"},
		{"SMTP_HOST": "smtp.example.com"},
	} {
		for key, value := range update {
			values[key] = value
		}
		if _, err := load(mapLookup(values)); err == nil {
			t.Errorf("load accepted invalid notification configuration %v", update)
		}
		for key := range update {
			delete(values, key)
		}
	}
	values["APP_ENV"] = "production"
	values["JWT_ACCESS_SECRET"] = "a-production-secret-with-more-than-32-characters"
	if _, err := load(mapLookup(values)); err == nil {
		t.Fatal("load accepted the production Redis development password")
	}
	values["REDIS_PASSWORD"] = "unique-production-redis-password"
	if _, err := load(mapLookup(values)); err != nil {
		t.Fatalf("load rejected valid production Redis configuration: %v", err)
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
