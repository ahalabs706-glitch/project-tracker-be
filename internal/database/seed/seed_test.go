package seed

import "testing"

func TestShouldSeedDemoData(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		setting     string
		wantSeed    bool
		wantError   bool
	}{
		{name: "local development seeds by default", environment: "development", wantSeed: true},
		{name: "non-production deployment seeds by default", environment: "staging", wantSeed: true},
		{name: "test deployment seeds by default", environment: "test", wantSeed: true},
		{name: "production never seeds by default", environment: "production"},
		{name: "production seed request is rejected", environment: "production", setting: "true", wantError: true},
		{name: "production environment matching is case-insensitive", environment: "Production", setting: "true", wantError: true},
		{name: "development can opt out", environment: "development", setting: "false"},
		{name: "non-production can opt out", environment: "staging", setting: "false"},
		{name: "unknown environment can opt in", environment: "preview", setting: "true", wantSeed: true},
		{name: "invalid setting is rejected", environment: "development", setting: "yes", wantError: true},
		{name: "missing environment is rejected", setting: "true", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ShouldSeedDemoData(test.environment, test.setting)
			if (err != nil) != test.wantError {
				t.Fatalf("ShouldSeedDemoData() error = %v, wantError %v", err, test.wantError)
			}
			if got != test.wantSeed {
				t.Fatalf("ShouldSeedDemoData() = %v, want %v", got, test.wantSeed)
			}
		})
	}
}
