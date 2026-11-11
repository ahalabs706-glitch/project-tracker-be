package management

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNotificationPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		query       string
		wantPage    int
		wantPerPage int
		wantError   bool
	}{
		{name: "defaults", query: "", wantPage: 1, wantPerPage: 20},
		{name: "custom values", query: "?page=2&per_page=50", wantPage: 2, wantPerPage: 50},
		{name: "invalid page", query: "?page=0", wantError: true},
		{name: "invalid per page", query: "?per_page=101", wantError: true},
		{name: "not an integer", query: "?page=abc", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest("GET", "/api/v1/notifications"+test.query, nil)
			page, perPage, err := notificationPagination(context)
			if (err != nil) != test.wantError {
				t.Fatalf("notificationPagination() error = %v, wantError %t", err, test.wantError)
			}
			if err == nil && (page != test.wantPage || perPage != test.wantPerPage) {
				t.Fatalf("notificationPagination() = (%d, %d), want (%d, %d)",
					page, perPage, test.wantPage, test.wantPerPage)
			}
		})
	}
}
