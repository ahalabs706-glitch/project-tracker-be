package management

import (
	"errors"
	"strings"
	"testing"
	"time"

	"projectflow-backend/internal/auth"
)

func TestValidateContractInputNormalizesAndDefaults(t *testing.T) {
	input := validContractInput()
	input.ContractNumber = "  CTR-2026-001 "
	input.Name = "  Infrastructure Support "
	input.Currency = "idr"
	input.RenewalType = "manual"
	input.RenewalNoticeDays = nil
	input.Status = ""

	got, err := validateContractInput(input)
	if err != nil {
		t.Fatalf("validateContractInput() error = %v", err)
	}
	if got.ContractNumber != "CTR-2026-001" || got.Name != "Infrastructure Support" {
		t.Fatalf("contract strings were not normalized: %#v", got)
	}
	if got.Currency != "IDR" || got.RenewalType != "MANUAL" || got.RenewalNoticeDays != 90 || got.Status != "ACTIVE" {
		t.Fatalf("contract defaults or enums are incorrect: %#v", got)
	}
}

func TestValidateContractInputRejectsInvalidBusinessFields(t *testing.T) {
	tests := []struct {
		name string
		edit func(*ContractInput)
	}{
		{name: "invalid owner UUID", edit: func(input *ContractInput) { input.OwnerID = "bad-id" }},
		{name: "invalid date", edit: func(input *ContractInput) { input.StartDate = "2026-02-30" }},
		{name: "effective date before start", edit: func(input *ContractInput) { input.EffectiveDate = "2025-12-31" }},
		{name: "end date before effective", edit: func(input *ContractInput) { input.EndDate = "2025-12-31" }},
		{name: "negative contract value", edit: func(input *ContractInput) { input.ContractValue = "-0.01" }},
		{name: "excess contract value precision", edit: func(input *ContractInput) { input.ContractValue = "10.999" }},
		{name: "invalid renewal type", edit: func(input *ContractInput) { input.RenewalType = "OPTIONAL" }},
		{name: "negative notice period", edit: func(input *ContractInput) {
			notice := -1
			input.RenewalNoticeDays = &notice
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validContractInput()
			test.edit(&input)
			if _, err := validateContractInput(input); !errors.Is(err, ErrInvalid) {
				t.Fatalf("validateContractInput() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestExpirationLevelBoundaries(t *testing.T) {
	tests := []struct {
		days int
		want string
	}{
		{days: 91, want: "NORMAL"},
		{days: 90, want: "EARLY_WARNING"},
		{days: 60, want: "EARLY_WARNING"},
		{days: 59, want: "WARNING"},
		{days: 30, want: "WARNING"},
		{days: 29, want: "HIGH_PRIORITY"},
		{days: 14, want: "HIGH_PRIORITY"},
		{days: 13, want: "CRITICAL"},
		{days: 1, want: "CRITICAL"},
		{days: 0, want: "EXPIRED"},
		{days: -1, want: "EXPIRED"},
	}
	for _, test := range tests {
		if got := expirationLevel(test.days); got != test.want {
			t.Errorf("expirationLevel(%d) = %q, want %q", test.days, got, test.want)
		}
	}
}

func TestDaysUntilEndUsesBusinessCalendarDate(t *testing.T) {
	today, err := time.Parse(time.RFC3339, "2026-11-09T23:30:00-08:00")
	if err != nil {
		t.Fatal(err)
	}
	got, err := daysUntilEnd("2026-11-10", today)
	if err != nil {
		t.Fatalf("daysUntilEnd() error = %v", err)
	}
	if got != 1 {
		t.Fatalf("daysUntilEnd() = %d, want 1", got)
	}
}

func TestContractAuthorizationScopes(t *testing.T) {
	departmentID := "650e8400-e29b-41d4-a716-446655440000"
	otherDepartmentID := "850e8400-e29b-41d4-a716-446655440000"
	ownerID := "550e8400-e29b-41d4-a716-446655440000"
	contract := Contract{
		Owner:      ContractOwner{ID: ownerID},
		Department: ContractDepartment{ID: departmentID},
	}
	tests := []struct {
		name      string
		principal auth.Principal
		canRead   bool
		canManage bool
	}{
		{
			name:      "same department contract manager",
			principal: auth.Principal{User: auth.User{ID: "750e8400-e29b-41d4-a716-446655440000", DepartmentID: &departmentID}, Roles: []string{"CONTRACT_MANAGER"}},
			canRead:   true, canManage: true,
		},
		{
			name:      "owner",
			principal: auth.Principal{User: auth.User{ID: ownerID, DepartmentID: &departmentID}},
			canRead:   true, canManage: true,
		},
		{
			name:      "other department",
			principal: auth.Principal{User: auth.User{ID: "750e8400-e29b-41d4-a716-446655440000", DepartmentID: &otherDepartmentID}, Roles: []string{"CONTRACT_MANAGER"}},
			canRead:   false, canManage: false,
		},
		{
			name:      "executive read only",
			principal: auth.Principal{User: auth.User{ID: "750e8400-e29b-41d4-a716-446655440000"}, Roles: []string{"EXECUTIVE"}},
			canRead:   true, canManage: false,
		},
		{
			name:      "administrator",
			principal: auth.Principal{User: auth.User{ID: "750e8400-e29b-41d4-a716-446655440000"}, Roles: []string{"ADMIN"}},
			canRead:   true, canManage: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := canReadContract(contract, test.principal); got != test.canRead {
				t.Errorf("canReadContract() = %t, want %t", got, test.canRead)
			}
			if got := canManageContract(test.principal, ownerID, departmentID); got != test.canManage {
				t.Errorf("canManageContract() = %t, want %t", got, test.canManage)
			}
		})
	}
}

func TestParseExpiringWithin(t *testing.T) {
	for _, value := range []string{"", "1", "3650"} {
		if _, err := parseExpiringWithin(value); err != nil {
			t.Errorf("parseExpiringWithin(%q) error = %v", value, err)
		}
	}
	for _, value := range []string{"0", "3651", "abc"} {
		if _, err := parseExpiringWithin(value); !errors.Is(err, ErrInvalid) {
			t.Errorf("parseExpiringWithin(%q) error = %v, want ErrInvalid", value, err)
		}
	}
}

func TestCanRenewContract(t *testing.T) {
	tests := []struct {
		renewalType string
		status      string
		want        bool
	}{
		{"MANUAL", "ACTIVE", true},
		{"AUTOMATIC", "EXPIRED", true},
		{"NONE", "ACTIVE", false},
		{"MANUAL", "COMPLETED", false},
		{"MANUAL", "TERMINATED", false},
		{"MANUAL", "DRAFT", false},
	}
	for _, test := range tests {
		if got := canRenewContract(test.renewalType, test.status); got != test.want {
			t.Errorf("canRenewContract(%q, %q) = %t, want %t", test.renewalType, test.status, got, test.want)
		}
	}
}

func TestContractWhereBuildsFiltersAndScope(t *testing.T) {
	departmentID := "650e8400-e29b-41d4-a716-446655440000"
	ownerID := "550e8400-e29b-41d4-a716-446655440000"
	principal := auth.Principal{
		User: auth.User{ID: ownerID, DepartmentID: &departmentID},
	}
	where, args, err := contractWhere(ContractFilter{
		Search: "vendor", Status: "active", DepartmentID: departmentID,
		ExpirationFrom: "2026-11-01", ExpirationTo: "2026-12-31",
	}, principal)
	if err != nil {
		t.Fatalf("contractWhere() error = %v", err)
	}
	for _, fragment := range []string{"c.contract_number ILIKE $1", "c.status = $2", "c.department_id = $3", "c.end_date >= $4", "c.end_date <= $5", "c.department_id = $7"} {
		if !strings.Contains(where, fragment) {
			t.Errorf("contractWhere() = %q, missing %q", where, fragment)
		}
	}
	if len(args) != 7 || args[5] != ownerID || args[6] != departmentID {
		t.Fatalf("contractWhere() args = %#v", args)
	}
}

func validContractInput() ContractInput {
	return ContractInput{
		ContractNumber: "CTR-2026-001",
		Name:           "Infrastructure Support",
		ContractType:   stringPointer("SERVICE"),
		VendorName:     "Example Vendor",
		OwnerID:        "550e8400-e29b-41d4-a716-446655440000",
		DepartmentID:   "650e8400-e29b-41d4-a716-446655440000",
		StartDate:      "2026-01-01",
		EffectiveDate:  "2026-01-01",
		EndDate:        "2026-12-31",
		ContractValue:  "250000000.00",
		Currency:       "IDR",
		RenewalType:    "MANUAL",
	}
}
