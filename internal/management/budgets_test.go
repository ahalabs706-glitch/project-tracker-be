package management

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"projectflow-backend/internal/auth"
)

func TestValidateBudgetAllocationRequiresPreciseNonnegativeAmounts(t *testing.T) {
	categoryID := "a50e8400-e29b-41d4-a716-446655440000"
	got, err := validateBudgetAllocation(BudgetAllocationInput{
		CategoryID: categoryID, PlannedAmount: "100000.10",
		ApprovedAmount: "90000", Currency: "idr",
	})
	if err != nil {
		t.Fatalf("validateBudgetAllocation() error = %v", err)
	}
	if got.Currency != "IDR" || got.ApprovedAmount != "90000" {
		t.Fatalf("validateBudgetAllocation() = %#v", got)
	}
	for _, amount := range []string{"-1", "1.001", "1000000000000000000", "1e2"} {
		_, err := validateBudgetAllocation(BudgetAllocationInput{
			CategoryID: categoryID, PlannedAmount: amount,
			ApprovedAmount: "0", Currency: "IDR",
		})
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("validateBudgetAllocation(%q) error = %v, want ErrInvalid", amount, err)
		}
	}
}

func TestValidateBudgetTransactionRules(t *testing.T) {
	input := BudgetTransactionInput{
		BudgetID:          "b50e8400-e29b-41d4-a716-446655440000",
		TransactionNumber: "  TRX-2026-001 ", TransactionType: "commitment",
		Amount: "75000000.00", TransactionDate: "2026-11-09",
	}
	got, err := validateBudgetTransaction(input)
	if err != nil {
		t.Fatalf("validateBudgetTransaction() error = %v", err)
	}
	if got.TransactionNumber != "TRX-2026-001" || got.Status != "DRAFT" || got.TransactionType != "COMMITMENT" {
		t.Fatalf("unexpected normalized transaction: %#v", got)
	}
	tests := []struct {
		name string
		edit func(*BudgetTransactionInput)
	}{
		{"zero amount", func(value *BudgetTransactionInput) { value.Amount = "0.00" }},
		{"fractional precision", func(value *BudgetTransactionInput) { value.Amount = "1.001" }},
		{"invalid date", func(value *BudgetTransactionInput) { value.TransactionDate = "2026-02-30" }},
		{"invalid type", func(value *BudgetTransactionInput) { value.TransactionType = "REFUND" }},
		{"invalid initial status", func(value *BudgetTransactionInput) { value.Status = "CANCELLED" }},
		{"commitment linked to another commitment", func(value *BudgetTransactionInput) {
			value.OriginalCommitmentID = stringPointer("c50e8400-e29b-41d4-a716-446655440000")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := input
			test.edit(&value)
			if _, err := validateBudgetTransaction(value); !errors.Is(err, ErrInvalid) {
				t.Fatalf("validateBudgetTransaction() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestValidateBudgetCorrectionRequiresReasonAndValidAmount(t *testing.T) {
	input := BudgetCorrectionInput{
		TransactionNumber: "TRX-CORR-001", Amount: "15.25",
		TransactionDate: "2026-11-09", Reason: "Corrected invoice amount",
	}
	if _, err := validateBudgetCorrection(input); err != nil {
		t.Fatalf("validateBudgetCorrection() error = %v", err)
	}
	input.Reason = " "
	if _, err := validateBudgetCorrection(input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("validateBudgetCorrection() error = %v, want ErrInvalid for missing reason", err)
	}
}

func TestBudgetUtilizationThresholds(t *testing.T) {
	tests := []struct {
		actual, approved string
		wantPercent      float64
		wantStatus       string
	}{
		{"79.99", "100", 79.99, "NORMAL"},
		{"80", "100", 80, "WARNING"},
		{"89.99", "100", 89.99, "WARNING"},
		{"90", "100", 90, "CRITICAL"},
		{"100", "100", 100, "CRITICAL"},
		{"100.01", "100", 100.01, "OVER_BUDGET"},
		{"0", "0", 0, "NORMAL"},
		{"1", "0", 0, "OVER_BUDGET"},
	}
	for _, test := range tests {
		gotPercent, gotStatus, err := budgetUtilization(test.actual, test.approved)
		if err != nil {
			t.Fatalf("budgetUtilization(%q, %q) error = %v", test.actual, test.approved, err)
		}
		if gotPercent != test.wantPercent || gotStatus != test.wantStatus {
			t.Errorf("budgetUtilization(%q, %q) = %v, %q; want %v, %q",
				test.actual, test.approved, gotPercent, gotStatus, test.wantPercent, test.wantStatus)
		}
	}
}

func TestBudgetTransactionFilterUsesParameterizedOperators(t *testing.T) {
	filter := BudgetTransactionFilter{
		CategoryID:      "a50e8400-e29b-41d4-a716-446655440000",
		TransactionType: "EXPENSE", Status: "POSTED",
		DateFrom: "2026-11-01", DateTo: "2026-11-30",
	}
	where, args := budgetTransactionWhere("850e8400-e29b-41d4-a716-446655440000", filter)
	for _, fragment := range []string{
		"pb.project_id=$1", "pb.category_id = $2", "bt.transaction_type = $3",
		"bt.status = $4", "bt.transaction_date >= $5", "bt.transaction_date <= $6",
	} {
		if !strings.Contains(where, fragment) {
			t.Errorf("budget transaction filter %q missing from %s", fragment, where)
		}
	}
	want := []any{
		"850e8400-e29b-41d4-a716-446655440000",
		"a50e8400-e29b-41d4-a716-446655440000", "EXPENSE", "POSTED",
		"2026-11-01", "2026-11-30",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("budgetTransactionWhere() args = %#v, want %#v", args, want)
	}
}

func TestBudgetWriteScope(t *testing.T) {
	departmentID := "650e8400-e29b-41d4-a716-446655440000"
	project := Project{DepartmentID: departmentID}
	tests := []struct {
		name      string
		principal auth.Principal
		want      bool
	}{
		{"finance in department", auth.Principal{User: auth.User{DepartmentID: &departmentID}, Roles: []string{"FINANCE"}}, true},
		{"admin across departments", auth.Principal{Roles: []string{"ADMIN"}}, true},
		{"project manager read-only", auth.Principal{User: auth.User{DepartmentID: &departmentID}, Roles: []string{"PROJECT_MANAGER"}}, false},
		{"finance outside department", auth.Principal{User: auth.User{DepartmentID: stringPointer("850e8400-e29b-41d4-a716-446655440000")}, Roles: []string{"FINANCE"}}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := canManageBudget(project, test.principal)
			if got != test.want {
				t.Fatalf("budget write access = %t, want %t", got, test.want)
			}
		})
	}
}
