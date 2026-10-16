package utils

import (
	"bytes"
	"html/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	mdParser = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)
	policy = bluemonday.UGCPolicy()
)

func init() {
	// Allow images
	policy.AllowImages()
	// AllowImages 的 alt 正则不含中文标点（，：（）等），含一个就整条丢掉；alt 输出时会转义，放开无风险
	policy.AllowAttrs("alt").OnElements("img")
	// Force links to open in new tab
	policy.AddTargetBlankToFullyQualifiedLinks(true)
	// Add noopener and follow security best practices, but don't force no-referrer to keep referrer data for targets
	// We will handle rel attributes manually in EnhanceHTMLContent
	policy.RequireNoReferrerOnLinks(false)
	// Allow iframe for video embedding in RSS content.
	// 不放行 style/class：否则可用 position:fixed 或站内 Tailwind 类盖住整页做钓鱼
	policy.AllowElements("iframe")
	policy.AllowAttrs("src", "frameborder", "allowfullscreen", "allow", "width", "height").OnElements("iframe")
}

func RenderMarkdown(source string) template.HTML {
	var buf bytes.Buffer
	if err := mdParser.Convert([]byte(source), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(source)) // Fallback
	}

	// Sanitize HTML
	sanitized := policy.SanitizeBytes(buf.Bytes())

	// Enhance Image Attributes
	return EnhanceHTMLContent(string(sanitized))
}
