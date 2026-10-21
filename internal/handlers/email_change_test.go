package handlers

import (
	"testing"
	"time"
)

func TestTakePendingEmail(t *testing.T) {
	add := func(id uint, expires time.Time) {
		pendingEmailChanges.Add(id, &pendingEmailChange{email: "new@example.com", code: "123456", expires: expires})
	}

	add(1, time.Now().Add(time.Minute))
	if _, ok := takePendingEmail(1, ""); ok {
		t.Fatal("empty code accepted")
	}
	if email, ok := takePendingEmail(1, "123456"); !ok || email != "new@example.com" {
		t.Fatalf("right code rejected: %q %v", email, ok)
	}
	if _, ok := takePendingEmail(1, "123456"); ok {
		t.Fatal("code reused after success")
	}

	add(2, time.Now().Add(time.Minute))
	for i := 0; i < maxVerifyAttempts; i++ {
		takePendingEmail(2, "000000")
	}
	if _, ok := takePendingEmail(2, "123456"); ok {
		t.Fatal("right code accepted after too many attempts")
	}

	add(3, time.Now().Add(-time.Second))
	if _, ok := takePendingEmail(3, "123456"); ok {
		t.Fatal("expired code accepted")
	}
	if getPendingEmail(3) != "" {
		t.Fatal("expired change still shown as pending")
	}
}
