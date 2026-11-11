package management

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"projectflow-backend/internal/auth"
)

func TestValidateProjectCreateNormalizesValues(t *testing.T) {
	input := ProjectCreateInput{
		Name:           "  Project Alpha  ",
		Description:    stringPointer("  Description  "),
		Category:       stringPointer("  R-KAP "),
		DepartmentID:   "650e8400-e29b-41d4-a716-446655440000",
		OwnerID:        "550e8400-e29b-41d4-a716-446655440000",
		ManagerID:      "750e8400-e29b-41d4-a716-446655440000",
		StartDate:      "2026-11-01",
		TargetEndDate:  "2027-03-31",
		ApprovedBudget: "500000000.00",
		Currency:       "idr",
		Priority:       "high",
		Status:         "",
	}

	got, err := validateProjectCreate(input)
	if err != nil {
		t.Fatalf("validateProjectCreate() error = %v", err)
	}
	if got.Name != "Project Alpha" || got.Description != "Description" || got.Category != "R-KAP" {
		t.Fatalf("unexpected normalized project text: %#v", got)
	}
	if got.Currency != "IDR" || got.Priority != "HIGH" || got.Status != "DRAFT" {
		t.Fatalf("unexpected normalized project enums: %#v", got)
	}
	if got.ApprovedBudget != "500000000.00" {
		t.Fatalf("approved budget = %q", got.ApprovedBudget)
	}
}

func TestValidateProjectCreateRejectsInvalidBusinessFields(t *testing.T) {
	base := ProjectCreateInput{
		Name:           "Project Alpha",
		DepartmentID:   "650e8400-e29b-41d4-a716-446655440000",
		OwnerID:        "550e8400-e29b-41d4-a716-446655440000",
		ManagerID:      "750e8400-e29b-41d4-a716-446655440000",
		StartDate:      "2026-11-01",
		TargetEndDate:  "2027-03-31",
		ApprovedBudget: "500000000.00",
		Currency:       "IDR",
		Priority:       "HIGH",
	}
	tests := []struct {
		name string
		edit func(*ProjectCreateInput)
	}{
		{name: "invalid UUID", edit: func(input *ProjectCreateInput) { input.OwnerID = "not-a-uuid" }},
		{name: "invalid date", edit: func(input *ProjectCreateInput) { input.StartDate = "2026-02-30" }},
		{name: "reversed dates", edit: func(input *ProjectCreateInput) { input.TargetEndDate = "2026-10-31" }},
		{name: "excess budget precision", edit: func(input *ProjectCreateInput) { input.ApprovedBudget = "1.001" }},
		{name: "negative budget", edit: func(input *ProjectCreateInput) { input.ApprovedBudget = "-1.00" }},
		{name: "invalid priority", edit: func(input *ProjectCreateInput) { input.Priority = "URGENT" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := base
			test.edit(&input)
			if _, err := validateProjectCreate(input); !errors.Is(err, ErrInvalid) {
				t.Fatalf("validateProjectCreate() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestProjectCodeSegments(t *testing.T) {
	if got := projectOwnerCode("Ada Lovelace"); got != "AL" {
		t.Fatalf("projectOwnerCode() = %q, want AL", got)
	}
	if got := projectOwnerCode("Madonna"); got != "MAD" {
		t.Fatalf("projectOwnerCode() = %q, want MAD", got)
	}
	if got := projectCategoryCode(stringPointer("R-KAP / 2026")); got != "RKAP2026" {
		t.Fatalf("projectCategoryCode() = %q, want RKAP2026", got)
	}
	if got := projectCategoryCode(stringPointer("東京")); got != "GENERAL" {
		t.Fatalf("projectCategoryCode() = %q, want GENERAL", got)
	}
}

func TestProjectWhereUsesCorrectParameterizedScope(t *testing.T) {
	principal := auth.Principal{
		User: auth.User{
			ID:           "750e8400-e29b-41d4-a716-446655440000",
			DepartmentID: stringPointer("650e8400-e29b-41d4-a716-446655440000"),
		},
	}
	where, args, err := projectWhere(ProjectFilter{
		Search:       "Alpha",
		Status:       "in_progress",
		DepartmentID: *principal.User.DepartmentID,
		ManagerID:    principal.User.ID,
		Priority:     "high",
	}, principal)
	if err != nil {
		t.Fatalf("projectWhere() error = %v", err)
	}
	if !strings.Contains(where, "p.archived_at IS NULL") ||
		!strings.Contains(where, "p.name ILIKE $1 OR p.project_code ILIKE $1") ||
		!strings.Contains(where, "p.department_id = $3") ||
		!strings.Contains(where, "p.manager_id = $4") ||
		!strings.Contains(where, "p.department_id = $7") {
		t.Fatalf("projectWhere() returned unexpected filters: %s", where)
	}
	wantArgs := []any{
		"%Alpha%", "IN_PROGRESS", *principal.User.DepartmentID,
		principal.User.ID, "HIGH", principal.User.ID, *principal.User.DepartmentID,
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("projectWhere() args = %#v, want %#v", args, wantArgs)
	}
}

func TestProjectReadScope(t *testing.T) {
	departmentID := "650e8400-e29b-41d4-a716-446655440000"
	project := Project{
		DepartmentID: departmentID,
		OwnerID:      "550e8400-e29b-41d4-a716-446655440000",
		ManagerID:    "750e8400-e29b-41d4-a716-446655440000",
	}
	tests := []struct {
		name      string
		principal auth.Principal
		want      bool
	}{
		{
			name:      "same department",
			principal: auth.Principal{User: auth.User{DepartmentID: &departmentID}},
			want:      true,
		},
		{
			name:      "executive cross department",
			principal: auth.Principal{Roles: []string{"EXECUTIVE"}},
			want:      true,
		},
		{
			name:      "other department",
			principal: auth.Principal{User: auth.User{DepartmentID: stringPointer("850e8400-e29b-41d4-a716-446655440000")}},
			want:      false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := canReadProject(project, test.principal); got != test.want {
				t.Fatalf("canReadProject() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestProjectCreationScope(t *testing.T) {
	departmentID := "650e8400-e29b-41d4-a716-446655440000"
	ownerID := "550e8400-e29b-41d4-a716-446655440000"
	managerID := "750e8400-e29b-41d4-a716-446655440000"
	input := cleanProjectInput{DepartmentID: departmentID, OwnerID: ownerID, ManagerID: managerID}
	tests := []struct {
		name      string
		principal auth.Principal
		want      bool
	}{
		{
			name:      "same department manager",
			principal: auth.Principal{User: auth.User{ID: managerID, DepartmentID: &departmentID}},
			want:      true,
		},
		{
			name:      "same department unrelated user",
			principal: auth.Principal{User: auth.User{ID: "950e8400-e29b-41d4-a716-446655440000", DepartmentID: &departmentID}},
			want:      false,
		},
		{
			name:      "administrator across departments",
			principal: auth.Principal{User: auth.User{ID: "950e8400-e29b-41d4-a716-446655440000"}, Roles: []string{"ADMIN"}},
			want:      true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := canCreateProject(test.principal, input); got != test.want {
				t.Fatalf("canCreateProject() = %t, want %t", got, test.want)
			}
		})
	}
}

func stringPointer(value string) *string {
	return &value
}
