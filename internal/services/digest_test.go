package services

import (
	"strings"
	"testing"
	"time"
)

func TestNextDigestTime(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	cases := []struct{ now, want string }{
		{"2026-11-07 19:59:59", "2026-11-07 20:00:00"},
		{"2026-11-07 20:00:00", "2026-11-08 20:00:00"}, // 正好到点：本次已经轮到，下一次是明天
		{"2026-11-07 23:30:00", "2026-11-08 20:00:00"},
		{"2026-12-31 21:00:00", "2027-01-01 20:00:00"},
	}
	for _, c := range cases {
		now, _ := time.ParseInLocation("2006-01-02 15:04:05", c.now, loc)
		got := nextDigestTime(now, 20).Format("2006-01-02 15:04:05")
		if got != c.want {
			t.Errorf("now=%s got %s want %s", c.now, got, c.want)
		}
	}
}

func TestDigestHour(t *testing.T) {
	for env, want := range map[string]int{"": 20, "8": 8, "0": 0, "24": 20, "abc": 20} {
		t.Setenv("DIGEST_HOUR", env)
		if got := digestHour(); got != want {
			t.Errorf("DIGEST_HOUR=%q got %d want %d", env, got, want)
		}
	}
}

func TestCommentIDFromReason(t *testing.T) {
	reason := `在文章 <a href="/p/abc123#comment-42" target="_blank">《标题》</a> 中回复了您的评论`
	if got := commentIDFromReason(reason); got != 42 {
		t.Fatalf("got %d", got)
	}
	if got := commentIDFromReason("系统通知"); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestCommentExcerpt(t *testing.T) {
	got := commentExcerpt("↳ 回复 [#3 @alice](#comment-9)\n\n说得对，\n我也这么想")
	if got != "说得对， 我也这么想" {
		t.Fatalf("got %q", got)
	}
	long := commentExcerpt(strings.Repeat("竹", 100))
	if !strings.HasSuffix(long, "…") || len([]rune(long)) != digestExcerptLen+1 {
		t.Fatalf("got %q", long)
	}
}

func TestUnsubscribeSignature(t *testing.T) {
	t.Setenv("SITE_URL", "https://zhulink.vip/")
	url := DigestUnsubscribeURL(7)
	if !strings.HasPrefix(url, "https://zhulink.vip/unsubscribe?u=7&s=") {
		t.Fatalf("got %s", url)
	}
	sig := url[strings.Index(url, "&s=")+3:]
	if id, ok := VerifyUnsubscribe("7", sig); !ok || id != 7 {
		t.Fatal("合法签名应通过")
	}
	if _, ok := VerifyUnsubscribe("8", sig); ok {
		t.Fatal("换了用户 ID 不应通过")
	}
	if _, ok := VerifyUnsubscribe("7", ""); ok {
		t.Fatal("空签名不应通过")
	}
	if _, ok := VerifyUnsubscribe("abc", sig); ok {
		t.Fatal("非法 ID 不应通过")
	}
}
