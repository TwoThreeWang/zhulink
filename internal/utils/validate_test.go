package utils

import (
	"strings"
	"testing"
)

func TestIsHTTPURL(t *testing.T) {
	for _, u := range []string{"https://example.com", "http://example.com/a?b=c#d", "HTTPS://Example.com/路径"} {
		if !IsHTTPURL(u) {
			t.Errorf("应接受: %q", u)
		}
	}
	for _, u := range []string{
		"", "javascript:alert(1)", "JaVaScRiPt:alert(1)//", " javascript:alert(1)", "data:text/html,<script>alert(1)</script>",
		"//example.com/x", "/p/abc", "example.com", "http://", "vbscript:x", "https://a.example/\x01",
		"https://example.com/" + strings.Repeat("a", MaxURLBytes),
	} {
		if IsHTTPURL(u) {
			t.Errorf("应拒绝: %q", u)
		}
	}
}

func TestNormalizeHTTPURL(t *testing.T) {
	if u, ok := NormalizeHTTPURL("  "); !ok || u != "" {
		t.Errorf("空白应视为没填: %q %v", u, ok)
	}
	if u, ok := NormalizeHTTPURL(" https://example.com/a \n"); !ok || u != "https://example.com/a" {
		t.Errorf("应去掉首尾空白: %q %v", u, ok)
	}
	if _, ok := NormalizeHTTPURL("javascript:alert(1)"); ok {
		t.Error("javascript: 协议未被拒绝")
	}
}

func TestValidUsername(t *testing.T) {
	for _, name := range []string{"a", "张三", "john_doe-1.2", "Jean Pierre", "竹林用户２", strings.Repeat("竹", MaxUsernameRunes)} {
		if !ValidUsername(name) {
			t.Errorf("应接受: %q", name)
		}
	}
	for _, name := range []string{
		"", " a", "a ", "a  b", "a\tb", "a\nb", "x](http://evil)", "<b>x</b>", "a@b", "管理员​", "🌱",
		strings.Repeat("竹", MaxUsernameRunes+1),
	} {
		if ValidUsername(name) {
			t.Errorf("应拒绝: %q", name)
		}
	}
}

func TestSanitizeUsername(t *testing.T) {
	cases := map[string]string{
		"john+tag":              "johntag",
		"  Jean   Pierre  ":     "Jean Pierre",
		"x](javascript:1)":      "xjavascript1",
		"+++":                   "竹友",
		"":                      "竹友",
		strings.Repeat("a", 50): strings.Repeat("a", MaxUsernameRunes),
	}
	for in, want := range cases {
		got := SanitizeUsername(in)
		if got != want {
			t.Errorf("SanitizeUsername(%q) = %q，期望 %q", in, got, want)
		}
		if !ValidUsername(got) {
			t.Errorf("整理后的用户名不合法: %q -> %q", in, got)
		}
	}
}

func TestValidAvatar(t *testing.T) {
	for _, a := range append(GetCommonEmojis(), "🏳️‍🌈", "👨‍👩‍👧‍👦", "👍🏽") {
		if !ValidAvatar(a) {
			t.Errorf("应接受: %q", a)
		}
	}
	for _, a := range []string{"", "A", "管理员", "admin", "🌱 x", "<img>", "1", strings.Repeat("🌱", MaxAvatarRunes+1), "\xff"} {
		if ValidAvatar(a) {
			t.Errorf("应拒绝: %q", a)
		}
	}
}
