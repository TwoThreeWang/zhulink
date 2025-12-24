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
	// Force links to open in new tab
	policy.AddTargetBlankToFullyQualifiedLinks(true)
	// Add noopener or noreferrer and follow security best practices
	policy.RequireNoReferrerOnLinks(true)
}

func RenderMarkdown(source string) template.HTML {
	var buf bytes.Buffer
	if err := mdParser.Convert([]byte(source), &buf); err != nil {
		return template.HTML(source) // Fallback
	}

	// Sanitize HTML
	sanitized := policy.SanitizeBytes(buf.Bytes())

	// Enhance Image Attributes
	return EnhanceHTMLContent(string(sanitized))
}
