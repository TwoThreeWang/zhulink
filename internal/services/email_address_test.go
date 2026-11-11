package services

import (
	"strings"
	"testing"
)

func TestValidEmailAddress(t *testing.T) {
	for _, e := range []string{"a@example.com", "john.doe+tag@sub.example.co", "用户@example.com"} {
		if !ValidEmailAddress(e) {
			t.Errorf("应接受: %q", e)
		}
	}
	for _, e := range []string{
		"", "a", "@example.com", "a@", "a b@example.com", "a@example.com\r\nBcc: x@evil.example",
		"Name <a@example.com>", "a@example.com, b@example.com", "a@@example.com", "<a@example.com>",
		strings.Repeat("a", 250) + "@example.com",
	} {
		if ValidEmailAddress(e) {
			t.Errorf("应拒绝: %q", e)
		}
	}
}
