package notifications

import (
	"context"
	"strings"
	"testing"
)

func TestDeterministicEventKeyIncludesRecipientAndOccurrence(t *testing.T) {
	event := Event{
		EventType: EventContractExpiration, EntityID: "contract-1", UserID: "user-1",
		Qualifier: "days-7", EventDate: "2026-11-09",
	}
	key := DeterministicEventKey(event)
	if key != "CONTRACT_EXPIRATION:contract-1:user-1:days-7:2026-11-09" {
		t.Fatalf("DeterministicEventKey() = %q", key)
	}
	event.UserID = "user-2"
	if got := DeterministicEventKey(event); got == key {
		t.Fatal("event key did not change for a different recipient")
	}
	event.UserID = "user-1"
	event.Qualifier = "days-1"
	if got := DeterministicEventKey(event); got == key {
		t.Fatal("event key did not change for a different reminder threshold")
	}
}

func TestSendEmailRejectsMissingConfigurationAndHeaderInjection(t *testing.T) {
	if err := sendEmail(context.Background(), SMTPConfig{}, "user@example.com", "Notice", "Body"); err == nil ||
		!strings.Contains(err.Error(), "disabled") {
		t.Fatalf("sendEmail() error = %v, want explicit disabled SMTP error", err)
	}
	if err := sendEmail(context.Background(), SMTPConfig{
		Host: "smtp.example.com", Port: "587", From: "notifications@example.com",
	}, "user@example.com", "Notice\r\nBcc: other@example.com", "Body"); err == nil ||
		!strings.Contains(err.Error(), "header") {
		t.Fatalf("sendEmail() error = %v, want header validation error", err)
	}
}

func TestTruncateError(t *testing.T) {
	if got := truncateError("short", 5); got != "short" {
		t.Fatalf("truncateError() = %q, want short", got)
	}
	if got := truncateError("long error", 4); got != "long" {
		t.Fatalf("truncateError() = %q, want long", got)
	}
}
