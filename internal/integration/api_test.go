package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunningAPIContract(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("INTEGRATION_BASE_URL"), "/")
	if baseURL == "" {
		t.Skip("set INTEGRATION_BASE_URL to run HTTP integration tests")
	}
	client := &http.Client{Timeout: 5 * time.Second}

	tests := []struct {
		method     string
		path       string
		origin     string
		wantStatus int
		wantCode   string
		wantData   bool
	}{
		{method: http.MethodGet, path: "/health", wantStatus: http.StatusOK, wantData: true},
		{method: http.MethodGet, path: "/ready", wantStatus: http.StatusOK, wantData: true},
		{method: http.MethodGet, path: "/api/v1/projects", wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{method: http.MethodGet, path: "/api/v1/roles", wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{method: http.MethodGet, path: "/api/v1/audit-logs", wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{method: http.MethodGet, path: "/api/v1/contracts", wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{method: http.MethodGet, path: "/api/v1/notifications", wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{method: http.MethodGet, path: "/health", origin: "https://untrusted.example", wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			request, err := http.NewRequest(test.method, baseURL+test.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("got HTTP %d, want %d", response.StatusCode, test.wantStatus)
			}
			if test.wantCode == "" && !test.wantData {
				return
			}
			var body struct {
				Success bool `json:"success"`
				Data    struct {
					Status string `json:"status"`
				} `json:"data"`
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode standard API response: %v", err)
			}
			if test.wantData {
				if !body.Success || body.Data.Status != "ok" {
					t.Fatalf("unexpected success envelope: %+v", body)
				}
				return
			}
			if body.Success || body.Error.Code != test.wantCode {
				t.Fatalf("unexpected API error envelope: %+v", body)
			}
		})
	}

}
