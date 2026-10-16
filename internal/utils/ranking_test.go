package utils

import (
	"testing"
	"time"
)

func TestCalculateScoreHotBeatsFresh(t *testing.T) {
	now := time.Now()
	// 3 天前 30 赞 10 评论 vs 1 小时前 1 赞：老爆款应排在前面
	old := CalculateScore(now.Add(-72*time.Hour), 30, 0, 0, 0, 10)
	fresh := CalculateScore(now.Add(-1*time.Hour), 1, 0, 0, 0, 0)
	if old <= fresh {
		t.Fatalf("old hot %.1f should beat fresh %.1f", old, fresh)
	}
	// 同一时间发布，互动多的分高；取整后仍能区分 1 赞和 2 赞
	if int(CalculateScore(now, 2, 0, 0, 0, 0)) <= int(CalculateScore(now, 1, 0, 0, 0, 0)) {
		t.Fatal("1 vs 2 upvotes tie after int truncation")
	}
	if CalculateScore(now, 0, 5, 0, 0, 0) != 0 {
		t.Fatal("negative weighted sum should clamp to 0")
	}
}
