package management

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestUserWhereBuildsParameterizedFilters(t *testing.T) {
	active := true
	filter := UserFilter{
		Search:       "Ada",
		DepartmentID: "650e8400-e29b-41d4-a716-446655440000",
		IsActive:     &active,
	}

	where, args, err := userWhere(filter)
	if err != nil {
		t.Fatalf("userWhere() error = %v", err)
	}
	if want := " WHERE (u.full_name ILIKE $1 OR u.email ILIKE $1) AND u.department_id = $2 AND u.is_active = $3"; where != want {
		t.Fatalf("userWhere() where = %q, want %q", where, want)
	}
	wantArgs := []any{"%Ada%", filter.DepartmentID, true}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("userWhere() args = %#v, want %#v", args, wantArgs)
	}
}

func TestUserWhereRejectsInvalidDepartmentID(t *testing.T) {
	_, _, err := userWhere(UserFilter{DepartmentID: "not-a-uuid"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("userWhere() error = %v, want ErrInvalid", err)
	}
}

func TestValidateUserFieldsNormalizesAndDeduplicates(t *testing.T) {
	departmentID := "650e8400-e29b-41d4-a716-446655440000"
	roleID := "750e8400-e29b-41d4-a716-446655440000"
	name, email, roles, err := validateUserFields(
		"  Ada Lovelace  ",
		"  ADA@example.com ",
		&departmentID,
		[]string{roleID, strings.ToUpper(roleID)},
	)
	if err != nil {
		t.Fatalf("validateUserFields() error = %v", err)
	}
	if name != "Ada Lovelace" || email != "ada@example.com" {
		t.Fatalf("normalized user = (%q, %q)", name, email)
	}
	if len(roles) != 1 || roles[0] != roleID {
		t.Fatalf("roles = %#v, want one normalized role ID", roles)
	}
}

func TestValidateUserFieldsCountsUnicodeCharacters(t *testing.T) {
	name := strings.Repeat("界", 150)
	_, _, _, err := validateUserFields(name, "ada@example.com", nil, []string{
		"750e8400-e29b-41d4-a716-446655440000",
	})
	if err != nil {
		t.Fatalf("validateUserFields() rejected 150-character Unicode name: %v", err)
	}
}

func TestValidateDepartmentFieldsNormalizesAndTrims(t *testing.T) {
	description := "  Shared services  "
	code, name, value, err := validateDepartmentFields(DepartmentInput{
		Code:        "  finance ",
		Name:        " Finance ",
		Description: &description,
	})
	if err != nil {
		t.Fatalf("validateDepartmentFields() error = %v", err)
	}
	if code != "FINANCE" || name != "Finance" || value != "Shared services" {
		t.Fatalf("normalized department = (%q, %q, %#v)", code, name, value)
	}
}
