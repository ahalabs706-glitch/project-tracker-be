package management

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"projectflow-backend/internal/auth"
)

func TestValidateTaskCreate(t *testing.T) {
	input := TaskCreateInput{
		Title:            "  Prepare design  ",
		Priority:         "high",
		PlannedStartDate: stringPointer("2026-11-09"),
		PlannedDueDate:   "2026-11-12",
		Progress:         floatPointer(15),
	}
	got, err := validateTaskCreate(input)
	if err != nil {
		t.Fatalf("validateTaskCreate() error = %v", err)
	}
	if got.Title != "Prepare design" || got.Priority != "HIGH" || got.Status != "TODO" || got.Progress != 15 {
		t.Fatalf("validateTaskCreate() returned unexpected normalized task: %#v", got)
	}
}

func TestValidateTaskCreateRejectsInvalidBusinessFields(t *testing.T) {
	base := TaskCreateInput{
		Title: "Prepare design", Priority: "HIGH", Status: "TODO",
		PlannedStartDate: stringPointer("2026-11-09"), PlannedDueDate: "2026-11-12",
		Progress: floatPointer(20),
	}
	tests := []struct {
		name string
		edit func(*TaskCreateInput)
	}{
		{name: "blank title", edit: func(input *TaskCreateInput) { input.Title = "  " }},
		{name: "invalid date", edit: func(input *TaskCreateInput) { input.PlannedDueDate = "2026-02-30" }},
		{name: "due date before start", edit: func(input *TaskCreateInput) { input.PlannedDueDate = "2026-11-08" }},
		{name: "invalid assignee", edit: func(input *TaskCreateInput) { input.AssignedTo = stringPointer("not-a-uuid") }},
		{name: "invalid status", edit: func(input *TaskCreateInput) { input.Status = "OVERDUE" }},
		{name: "progress out of range", edit: func(input *TaskCreateInput) { input.Progress = floatPointer(100.01) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := base
			test.edit(&input)
			if _, err := validateTaskCreate(input); !errors.Is(err, ErrInvalid) {
				t.Fatalf("validateTaskCreate() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestDoneTaskIsCompletedWithFullProgress(t *testing.T) {
	task, err := validateTaskCreate(TaskCreateInput{
		Title: "Complete work", Status: "DONE", PlannedDueDate: "2026-11-20", Progress: floatPointer(40),
	})
	if err != nil {
		t.Fatalf("validateTaskCreate() error = %v", err)
	}
	if task.Progress != 100 {
		t.Fatalf("completed task progress = %v, want 100", task.Progress)
	}
}

func TestValidateTaskCreateNormalizesMultipleAssignees(t *testing.T) {
	first := "750e8400-e29b-41d4-a716-446655440000"
	second := "850e8400-e29b-41d4-a716-446655440000"
	task, err := validateTaskCreate(TaskCreateInput{
		Title: "Prepare design", PlannedDueDate: "2026-11-12",
		AssigneeIDs: []string{strings.ToUpper(first), second},
	})
	if err != nil {
		t.Fatalf("validateTaskCreate() error = %v", err)
	}
	if !reflect.DeepEqual(task.AssigneeIDs, []string{first, second}) ||
		task.AssignedTo == nil || *task.AssignedTo != first {
		t.Fatalf("normalized task assignees = %#v, legacy assignee = %v", task.AssigneeIDs, task.AssignedTo)
	}
}

func TestValidateTaskCreateSupportsLegacyAssigneeAndRejectsDuplicateAssignees(t *testing.T) {
	assigneeID := "750e8400-e29b-41d4-a716-446655440000"
	task, err := validateTaskCreate(TaskCreateInput{
		Title: "Prepare design", PlannedDueDate: "2026-11-12", AssignedTo: &assigneeID,
	})
	if err != nil {
		t.Fatalf("validateTaskCreate() legacy input error = %v", err)
	}
	if !reflect.DeepEqual(task.AssigneeIDs, []string{assigneeID}) {
		t.Fatalf("legacy assignee IDs = %#v, want one assignee", task.AssigneeIDs)
	}
	if _, err := validateTaskCreate(TaskCreateInput{
		Title: "Prepare design", PlannedDueDate: "2026-11-12",
		AssigneeIDs: []string{assigneeID, strings.ToUpper(assigneeID)},
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate task assignees error = %v, want ErrInvalid", err)
	}
}

func TestMergeTaskUpdatePreservesAndClearsAssignees(t *testing.T) {
	first := "750e8400-e29b-41d4-a716-446655440000"
	second := "850e8400-e29b-41d4-a716-446655440000"
	current := Task{
		Assignees: []TaskAssignee{{UserID: first}, {UserID: second}},
		Title:     "Prepare design", PlannedDueDate: "2026-11-12", Status: "TODO", Priority: "MEDIUM",
	}
	preserved := mergeTaskUpdate(current, TaskUpdateInput{Title: stringPointer("Updated")})
	if !reflect.DeepEqual(preserved.AssigneeIDs, []string{first, second}) {
		t.Fatalf("omitted assignees were not preserved: %#v", preserved.AssigneeIDs)
	}
	empty := []string{}
	cleared := mergeTaskUpdate(current, TaskUpdateInput{AssigneeIDs: &empty})
	if len(cleared.AssigneeIDs) != 0 {
		t.Fatalf("explicitly cleared assignees = %#v, want none", cleared.AssigneeIDs)
	}
	if !sameTaskAssigneeIDs(current.Assignees, []string{second, first}) {
		t.Fatal("assignee comparison should ignore presentation order")
	}
}

func TestTaskActualDatesAreSetOnceAndPreserved(t *testing.T) {
	today := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	started, completed := taskActualDates("IN_PROGRESS", today, nil, nil)
	if started == nil || *started != "2026-11-09" || completed != nil {
		t.Fatalf("IN_PROGRESS actual dates = %v, %v", started, completed)
	}
	started, completed = taskActualDates("DONE", today.AddDate(0, 0, 2), started, nil)
	if started == nil || *started != "2026-11-09" || completed == nil || *completed != "2026-11-11" {
		t.Fatalf("DONE actual dates = %v, %v", started, completed)
	}
	started, completed = taskActualDates("TODO", today.AddDate(0, 0, 3), started, completed)
	if *started != "2026-11-09" || *completed != "2026-11-11" {
		t.Fatalf("reopened task actual dates were not preserved: %v, %v", started, completed)
	}
}

func TestTaskOverdueUsesBusinessDayAndExcludesTerminalStatuses(t *testing.T) {
	today := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	if isTaskOverdue("TODO", today, today) {
		t.Fatal("task due today should not be overdue")
	}
	if !isTaskOverdue("BLOCKED", today.AddDate(0, 0, -1), today) {
		t.Fatal("blocked task past due should be overdue")
	}
	if isTaskOverdue("DONE", today.AddDate(0, 0, -1), today) ||
		isTaskOverdue("CANCELLED", today.AddDate(0, 0, -1), today) {
		t.Fatal("completed or cancelled tasks should not be overdue")
	}
}

func TestValidateMilestoneCreateAndUpdateMerge(t *testing.T) {
	milestone, err := validateMilestoneCreate(MilestoneCreateInput{
		Name: "  Requirements approval  ", PlannedDate: "2026-12-01",
	})
	if err != nil {
		t.Fatalf("validateMilestoneCreate() error = %v", err)
	}
	if milestone.Name != "Requirements approval" || milestone.Status != "PENDING" {
		t.Fatalf("unexpected milestone values: %#v", milestone)
	}
	current := Milestone{
		OwnerID: stringPointer("750e8400-e29b-41d4-a716-446655440000"),
		Name:    "Requirements approval", PlannedDate: "2026-12-01", Status: "IN_PROGRESS",
	}
	merged := mergeMilestoneUpdate(current, MilestoneUpdateInput{PlannedDate: stringPointer("2026-12-05")})
	if merged.Name != current.Name || merged.Status != current.Status || merged.PlannedDate != "2026-12-05" {
		t.Fatalf("unexpected merged milestone: %#v", merged)
	}
}

func TestTaskFiltersBuildParameterizedScope(t *testing.T) {
	departmentID := "650e8400-e29b-41d4-a716-446655440000"
	userID := "750e8400-e29b-41d4-a716-446655440000"
	filter := TaskFilter{Status: "IN_PROGRESS", Priority: "HIGH", Overdue: boolPointer(true)}
	where, args := taskWhere(filter, auth.Principal{
		User: auth.User{ID: userID, DepartmentID: &departmentID},
	}, "850e8400-e29b-41d4-a716-446655440000", "", time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(where, "t.project_id=$1") ||
		!strings.Contains(where, "t.status=$2") ||
		!strings.Contains(where, "t.priority=$3") ||
		!strings.Contains(where, "t.planned_due_date < $4") ||
		!strings.Contains(where, "permission_code=$6") ||
		!strings.Contains(where, "p.department_id=$7") {
		t.Fatalf("taskWhere() returned unexpected query: %s", where)
	}
	want := []any{
		"850e8400-e29b-41d4-a716-446655440000", "IN_PROGRESS", "HIGH",
		"2026-11-09", userID, "tasks.read", departmentID, userID,
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("taskWhere() args = %#v, want %#v", args, want)
	}
}

func TestTaskAssigneeFilterMatchesAnySharedAssignee(t *testing.T) {
	userID := "750e8400-e29b-41d4-a716-446655440000"
	where, args := taskWhere(TaskFilter{}, auth.Principal{
		Roles: []string{"SUPER_ADMIN"},
	}, "", userID, time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(where, "EXISTS (SELECT 1 FROM task_assignees ta WHERE ta.task_id=t.id AND ta.user_id=$1)") {
		t.Fatalf("taskWhere() did not filter through task assignees: %s", where)
	}
	if !reflect.DeepEqual(args, []any{userID}) {
		t.Fatalf("taskWhere() args = %#v, want user ID", args)
	}
}

func TestMilestoneScheduleChangesAcceptOptionalTaskDates(t *testing.T) {
	if !sameOptionalDate(nil, nil) {
		t.Fatal("two omitted optional dates should be equal")
	}
	if sameOptionalDate(nil, stringPointer("2026-11-09")) {
		t.Fatal("adding a planned start date must be recorded as a change")
	}
	if !sameOptionalDate(stringPointer("2026-11-09"), stringPointer("2026-11-09")) {
		t.Fatal("unchanged planned date should compare equal")
	}
}

func floatPointer(value float64) *float64 {
	return &value
}

func boolPointer(value bool) *bool {
	return &value
}
