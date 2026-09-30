package service

import (
	"testing"
	"time"
)

func TestActivationResendAtWaitsCooldownAfterSending(t *testing.T) {
	if activationResendAt(nil) != nil {
		t.Fatal("no link sent yet: resend must be allowed")
	}

	sentAt := time.Date(2026, 9, 26, 14, 0, 0, 0, time.UTC)
	expiresAt := sentAt.Add(activationTTL)
	got := activationResendAt(&expiresAt)
	if got == nil || !got.Equal(sentAt.Add(activationResendCooldown)) {
		t.Fatalf("resend at = %v, want %v", got, sentAt.Add(activationResendCooldown))
	}
}
