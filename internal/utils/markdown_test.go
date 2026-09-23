package utils

import (
	"strings"
	"testing"
)

func TestRenderMarkdownBlocksInjection(t *testing.T) {
	for _, src := range []string{
		`https://www.bilibili.com/video/x"onload="alert(1)`,
		`https://www.youtube.com/watch?v=x"onload="alert(1)`,
		`https://youtu.be/x"onload="alert(1)`,
		`https://b23.tv/x"onmouseover="alert(1)`,
	} {
		if out := string(RenderMarkdown(src)); strings.Contains(out, `onload="`) || strings.Contains(out, `onmouseover="`) {
			t.Errorf("注入未被拦截: %s\n=> %s", src, out)
		}
	}

	if out := SanitizeHTML(`<div style="position:fixed" class="fixed inset-0"><iframe style="position:fixed" class="fixed" src="https://a.example"></iframe></div>`); strings.Contains(out, "style=") || strings.Contains(out, "class=") {
		t.Errorf("style/class 未被过滤: %s", out)
	}

	if out := string(RenderMarkdown("https://www.bilibili.com/video/BV1xx411c7mD")); !strings.Contains(out, "bvid=BV1xx411c7mD") {
		t.Errorf("正常视频链接未嵌入: %s", out)
	}
}
