package management

import (
	"testing"
)

func TestAuditLogWhere(t *testing.T) {
	filter := AuditLogFilter{
		ActorID:    "10000000-0000-4000-8000-000000000001",
		Action:     "PROJECT_STATUS_UPDATED",
		EntityType: "project",
		EntityID:   "10000000-0000-4000-8000-000000000002",
		DateFrom:   "2026-01-01",
		DateTo:     "2026-01-31",
	}
	where, args, err := auditLogWhere(filter)
	if err != nil {
		t.Fatal(err)
	}
	if want := 6; len(args) != want {
		t.Fatalf("got %d filter arguments, want %d", len(args), want)
	}
	if args[2] != "PROJECT" {
		t.Fatalf("entity type = %v, want PROJECT", args[2])
	}
	if where == "" {
		t.Fatal("expected filter clauses")
	}
}

func TestAuditLogWhereRejectsInvalidFilters(t *testing.T) {
	tests := []AuditLogFilter{
		{ActorID: "not-a-uuid"},
		{EntityType: "project; DROP TABLE audit_logs"},
		{Action: string(make([]byte, 101))},
		{DateFrom: "2026-02-01", DateTo: "2026-01-01"},
		{DateFrom: "01-01-2026"},
	}
	for _, filter := range tests {
		if _, _, err := auditLogWhere(filter); err == nil {
			t.Errorf("auditLogWhere(%+v) returned no error", filter)
		}
	}
}
