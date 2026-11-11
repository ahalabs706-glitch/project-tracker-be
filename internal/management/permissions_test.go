package management

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalizePermissionCodes(t *testing.T) {
	allowed := projectScopedPermissionSet()
	got, err := normalizePermissionCodes([]string{" tasks.update ", "tasks.read"}, allowed)
	if err != nil {
		t.Fatalf("normalizePermissionCodes() error = %v", err)
	}
	if want := []string{"tasks.read", "tasks.update"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizePermissionCodes() = %#v, want %#v", got, want)
	}
}

func TestNormalizePermissionCodesRejectsInvalidProjectGrant(t *testing.T) {
	_, err := normalizePermissionCodes([]string{"users.create"}, projectScopedPermissionSet())
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("normalizePermissionCodes() error = %v, want ErrInvalid", err)
	}
}

func TestNormalizePermissionCodesRejectsDuplicates(t *testing.T) {
	_, err := normalizePermissionCodes([]string{"tasks.read", " TASKS.READ "}, projectScopedPermissionSet())
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("normalizePermissionCodes() error = %v, want ErrInvalid", err)
	}
}

func TestProjectRoleDefaultPermissions(t *testing.T) {
	tests := []struct {
		role       string
		permission string
		want       bool
	}{
		{role: "MEMBER", permission: "tasks.assign", want: true},
		{role: "MEMBER", permission: "tasks.create"},
		{role: "LEAD", permission: "tasks.create", want: true},
		{role: "LEAD", permission: "milestones.update", want: true},
		{role: "CONTRIBUTOR", permission: "tasks.assign"},
		{role: "CONTRIBUTOR", permission: "milestones.read", want: true},
	}
	for _, test := range tests {
		t.Run(test.role+"/"+test.permission, func(t *testing.T) {
			got := false
			for _, permission := range projectRoleDefaults[test.role] {
				if permission == test.permission {
					got = true
					break
				}
			}
			if got != test.want {
				t.Fatalf("default permission %q for %s = %v, want %v", test.permission, test.role, got, test.want)
			}
		})
	}
}
