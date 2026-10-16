package handlers

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCleanHTMLAndTruncateKeepsValidUTF8(t *testing.T) {
	in := "<p>" + strings.Repeat("竹林", 20) + "。" + strings.Repeat("中文摘要", 20) + "</p>"
	out := cleanHTMLAndTruncate(in, 50)
	if !utf8.ValidString(out) {
		t.Fatalf("invalid utf8: %q", out)
	}
	if out != strings.Repeat("竹林", 20)+"。" {
		t.Fatalf("should cut at the full-width period, got %q", out)
	}
}
