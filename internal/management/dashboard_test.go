package management

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"projectflow-backend/internal/auth"
)

func TestDashboardProjectWhereAppliesFiltersAndReadScope(t *testing.T) {
	departmentID := "650e8400-e29b-41d4-a716-446655440000"
	userID := "750e8400-e29b-41d4-a716-446655440000"
	filter := DashboardFilter{
		DepartmentID: departmentID, ManagerID: userID, Status: "IN_PROGRESS",
		DateFrom: "2026-11-01", DateTo: "2026-12-31",
	}
	principal := auth.Principal{
		User: auth.User{ID: userID, DepartmentID: &departmentID},
	}
	where, args := dashboardProjectWhere(filter, principal)
	for _, fragment := range []string{
		"p.archived_at IS NULL", "p.department_id=$1", "p.manager_id=$2",
		"p.status=$3", "p.target_end_date >= $4", "p.start_date <= $5",
		"p.department_id=$6",
	} {
		if !strings.Contains(where, fragment) {
			t.Errorf("dashboardProjectWhere() = %q, missing %q", where, fragment)
		}
	}
	want := []any{departmentID, userID, "IN_PROGRESS", "2026-11-01", "2026-12-31", departmentID, userID}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("dashboardProjectWhere() args = %#v, want %#v", args, want)
	}
}

func TestDashboardContractWhereScopesAndFilters(t *testing.T) {
	departmentID := "650e8400-e29b-41d4-a716-446655440000"
	userID := "750e8400-e29b-41d4-a716-446655440000"
	where, args := dashboardContractWhere(DashboardFilter{
		DepartmentID: departmentID, ManagerID: userID, Status: "IN_PROGRESS",
		DateFrom: "2026-11-01", DateTo: "2026-12-31",
	}, auth.Principal{User: auth.User{ID: userID, DepartmentID: &departmentID}})
	for _, fragment := range []string{
		"c.department_id=$1", "cp.manager_id=$2", "cp.status=$3",
		"c.end_date >= $4", "c.end_date <= $5", "c.department_id=$7",
	} {
		if !strings.Contains(where, fragment) {
			t.Errorf("dashboardContractWhere() = %q, missing %q", where, fragment)
		}
	}
	want := []any{departmentID, userID, "IN_PROGRESS", "2026-11-01", "2026-12-31", userID, departmentID}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("dashboardContractWhere() args = %#v, want %#v", args, want)
	}
}

func TestParseDashboardFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{name: "defaults", query: "", wantErr: false},
		{name: "valid filters", query: "?date_from=2026-11-01&date_to=2026-11-30&status=in_progress&page=2&per_page=50&within_days=14", wantErr: false},
		{name: "reversed dates", query: "?date_from=2026-12-01&date_to=2026-11-30", wantErr: true},
		{name: "invalid date", query: "?date_from=2026-02-30", wantErr: true},
		{name: "invalid status", query: "?status=UNKNOWN", wantErr: true},
		{name: "too many results", query: "?per_page=101", wantErr: true},
		{name: "invalid window", query: "?within_days=366", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest("GET", "/api/v1/dashboard/overview"+test.query, nil)
			got, err := parseDashboardFilter(context)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseDashboardFilter() error = %v, wantErr %t", err, test.wantErr)
			}
			if err == nil && test.name == "defaults" {
				if got.Page != 1 || got.PerPage != 20 || got.WithinDays != 30 {
					t.Fatalf("default dashboard filters = %#v", got)
				}
			}
			if err == nil && test.name == "valid filters" {
				if got.Status != "IN_PROGRESS" || got.Page != 2 || got.PerPage != 50 || got.WithinDays != 14 {
					t.Fatalf("parsed dashboard filters = %#v", got)
				}
			}
		})
	}
}

func TestDashboardForecastUsesExactDecimalAddition(t *testing.T) {
	got, ok := newBigDecimalSum("0.10", "0.20")
	if !ok || got != "0.30" {
		t.Fatalf("newBigDecimalSum() = %q, %t; want 0.30, true", got, ok)
	}
	if _, ok := newBigDecimalSum("not-decimal", "1.00"); ok {
		t.Fatal("newBigDecimalSum() accepted an invalid decimal")
	}
}
