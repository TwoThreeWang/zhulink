package utils

import (
	"bytes"
	"html/template"
	"strconv"
	"strings"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

var (
	rendererOptions = goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithXHTML(),
	)
	// 正文渲染器：标题自动生成 id，供文章目录跳转
	articleMD = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		rendererOptions,
	)
	// 评论等片段渲染器：不生成标题 id，避免与正文锚点重名
	snippetMD = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		rendererOptions,
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
	// Allow iframe for video embedding.
	// 不放行 style/class：否则可用 position:fixed 或站内 Tailwind 类盖住整页做钓鱼
	policy.AllowElements("iframe")
	policy.AllowAttrs("src", "frameborder", "allowfullscreen", "allow", "width", "height").OnElements("iframe")
}

// RenderMarkdown 渲染评论等片段，标题不带 id
func RenderMarkdown(source string) template.HTML {
	var buf bytes.Buffer
	if err := snippetMD.Convert([]byte(source), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(source)) // Fallback
	}
	return sanitizeAndEnhance(buf.Bytes())
}

// TOCItem 文章目录中的一项
type TOCItem struct {
	ID    string
	Text  string
	Depth int // 相对缩进层级，0 为文中出现的最高一级标题
}

// tocMinItems 标题少于这个数时不展示目录
const tocMinItems = 3

// RenderArticle 渲染文章正文，并提取 h1~h3 作为目录（标题不足 tocMinItems 个时返回 nil）
func RenderArticle(source string) (template.HTML, []TOCItem) {
	src := []byte(source)
	ctx := parser.NewContext(parser.WithIDs(newHeadingIDs()))
	doc := articleMD.Parser().Parse(text.NewReader(src), parser.WithContext(ctx))

	var buf bytes.Buffer
	if err := articleMD.Renderer().Render(&buf, src, doc); err != nil {
		return template.HTML(template.HTMLEscapeString(source)), nil
	}
	return sanitizeAndEnhance(buf.Bytes()), extractTOC(doc, src)
}

func sanitizeAndEnhance(raw []byte) template.HTML {
	return EnhanceHTMLContent(string(policy.SanitizeBytes(raw)))
}

func extractTOC(doc ast.Node, src []byte) []TOCItem {
	type heading struct {
		level    int
		id, text string
	}
	var hs []heading
	minLevel := 4
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		h, ok := n.(*ast.Heading)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		if h.Level <= 3 {
			id, _ := h.AttributeString("id")
			idBytes, _ := id.([]byte)
			if t := strings.TrimSpace(plainText(h, src)); t != "" && len(idBytes) > 0 {
				hs = append(hs, heading{h.Level, string(idBytes), t})
				minLevel = min(minLevel, h.Level)
			}
		}
		return ast.WalkSkipChildren, nil
	})
	if len(hs) < tocMinItems {
		return nil
	}
	items := make([]TOCItem, len(hs))
	for i, h := range hs {
		items[i] = TOCItem{ID: h.id, Text: h.text, Depth: h.level - minLevel}
	}
	return items
}

// plainText 拼出标题的纯文本（去掉强调、行内代码、链接等标记）
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := c.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(src))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// headingIDs 生成形如 toc-背景介绍 的标题 id：
// 固定前缀避免与页面自身的锚点（如 #comments）冲突，保留中文让链接可读
type headingIDs struct {
	used map[string]bool
}

func newHeadingIDs() *headingIDs {
	return &headingIDs{used: map[string]bool{}}
}

const headingIDMaxRunes = 48

func (s *headingIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	base := "toc-" + slugify(string(value))
	id := base
	for i := 2; s.used[id]; i++ {
		id = base + "-" + strconv.Itoa(i)
	}
	s.used[id] = true
	return []byte(id)
}

func (s *headingIDs) Put(value []byte) {
	s.used[string(value)] = true
}

// slugify 保留各语言的字母和数字，空白、连字符、下划线折叠为单个 "-"，其余符号丢弃
func slugify(s string) string {
	var b strings.Builder
	n, pendingDash := 0, false
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if pendingDash && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingDash = false
			b.WriteRune(r)
			if n++; n >= headingIDMaxRunes {
				return b.String()
			}
		case unicode.IsSpace(r) || r == '-' || r == '_':
			pendingDash = true
		}
	}
	if b.Len() == 0 {
		return "section"
	}
	return b.String()
}
