package handlers

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/models"
	"zhulink/internal/services"
	"zhulink/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SEOHandler struct{}

const sitemapPostPageSize = 45000

func NewSEOHandler() *SEOHandler {
	return &SEOHandler{}
}

// getSiteURL 从环境变量获取网站URL,如果未设置则使用默认值
func getSiteURL() string {
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "https://zhulink.vip"
	}
	return siteURL
}

// RobotsTxt 返回robots.txt内容(备用,优先使用静态文件)
func (h *SEOHandler) RobotsTxt(c *gin.Context) {
	siteURL := getSiteURL()
	content := fmt.Sprintf(`User-agent: *
Allow: /

# 禁止爬取用户后台和管理后台
Disallow: /dashboard/
Disallow: /admin/

# 禁止爬取登录注册页面
Disallow: /login
Disallow: /signup

# 禁止爬取API端点
Disallow: /vote/
Disallow: /bookmark/
Disallow: /report/

# Sitemap位置
Sitemap: %s/sitemap.xml

`, siteURL)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, content)
}

// SitemapIndex 动态生成 sitemap index。
func (h *SEOHandler) SitemapIndex(c *gin.Context) {
	siteURL := getSiteURL()
	now := time.Now().Format("2006-01-02")

	var postCount int64
	indexablePostsQuery().Count(&postCount)
	postPages := int((postCount + sitemapPostPageSize - 1) / sitemapPostPageSize)

	var xml strings.Builder
	xml.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
`)
	xml.WriteString(fmt.Sprintf(`  <sitemap>
    <loc>%s/sitemap-static.xml</loc>
    <lastmod>%s</lastmod>
  </sitemap>
`, siteURL, now))

	for page := 1; page <= postPages; page++ {
		xml.WriteString(fmt.Sprintf(`  <sitemap>
    <loc>%s/sitemap-posts/%d.xml</loc>
    <lastmod>%s</lastmod>
  </sitemap>
`, siteURL, page, now))
	}

	xml.WriteString(`</sitemapindex>`)

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(http.StatusOK, xml.String())
}

// SitemapStaticXML 生成公共静态页和节点页 sitemap。
func (h *SEOHandler) SitemapStaticXML(c *gin.Context) {
	siteURL := getSiteURL()
	now := time.Now().Format("2006-01-02")

	var xml strings.Builder
	xml.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
`)

	xml.WriteString(fmt.Sprintf(`  <url>
    <loc>%s/</loc>
    <lastmod>%s</lastmod>
    <changefreq>daily</changefreq>
    <priority>1.0</priority>
  </url>
`, siteURL, now))

	xml.WriteString(fmt.Sprintf(`  <url>
    <loc>%s/new</loc>
    <lastmod>%s</lastmod>
    <changefreq>hourly</changefreq>
    <priority>0.9</priority>
  </url>
`, siteURL, now))

	xml.WriteString(fmt.Sprintf(`  <url>
    <loc>%s/nodes</loc>
    <lastmod>%s</lastmod>
    <changefreq>weekly</changefreq>
    <priority>0.8</priority>
  </url>
`, siteURL, now))

	var nodes []models.Node
	db.DB.Find(&nodes)

	// 一次性查询每个节点的最新文章更新时间，避免 N+1
	type nodeLatest struct {
		NodeID  uint
		Lastmod time.Time
	}
	var nodeLatestRows []nodeLatest
	indexablePostsQuery().
		Select("node_id, MAX(updated_at) AS lastmod").
		Group("node_id").
		Scan(&nodeLatestRows)
	nodeLastmodMap := make(map[uint]string, len(nodeLatestRows))
	for _, r := range nodeLatestRows {
		nodeLastmodMap[r.NodeID] = r.Lastmod.Format("2006-01-02")
	}

	for _, node := range nodes {
		nodeLastmod, ok := nodeLastmodMap[node.ID]
		if !ok {
			continue
		}

		xml.WriteString(fmt.Sprintf(`  <url>
    <loc>%s/t/%s</loc>
    <lastmod>%s</lastmod>
    <changefreq>daily</changefreq>
    <priority>0.7</priority>
  </url>
`, siteURL, url.PathEscape(node.Name), nodeLastmod))
	}

	xml.WriteString(`</urlset>`)

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(http.StatusOK, xml.String())
}

// SitemapPostsXML 生成可索引帖子 sitemap 分片。
func (h *SEOHandler) SitemapPostsXML(c *gin.Context) {
	siteURL := getSiteURL()
	pageStr := strings.TrimSuffix(c.Param("page"), ".xml")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		c.String(http.StatusNotFound, "Not Found")
		return
	}

	var posts []models.Post
	indexablePostsQuery().
		Order("created_at DESC").
		Offset((page - 1) * sitemapPostPageSize).
		Limit(sitemapPostPageSize).
		Find(&posts)

	if len(posts) == 0 && page > 1 {
		c.String(http.StatusNotFound, "Not Found")
		return
	}

	var xml strings.Builder
	xml.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
`)

	for _, post := range posts {
		lastmod := post.UpdatedAt.Format("2006-01-02")
		daysSinceCreated := time.Since(post.CreatedAt).Hours() / 24
		priority := 0.6
		changefreq := "weekly"

		if daysSinceCreated < 7 {
			priority = 0.8
			changefreq = "daily"
		} else if daysSinceCreated < 30 {
			priority = 0.7
			changefreq = "weekly"
		}

		xml.WriteString(fmt.Sprintf(`  <url>
    <loc>%s/p/%s</loc>
    <lastmod>%s</lastmod>
    <changefreq>%s</changefreq>
    <priority>%.1f</priority>
  </url>
`, siteURL, post.Pid, lastmod, changefreq, priority))
	}

	xml.WriteString(`</urlset>`)

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(http.StatusOK, xml.String())
}

func indexablePostsQuery() *gorm.DB {
	return db.DB.Model(&models.Post{}).
		Where("index_status = ? OR index_status = '' OR index_status IS NULL", models.PostIndexStatusIndexable)
}

// RSSFeed 生成RSS 2.0 feed
func (h *SEOHandler) RSSFeed(c *gin.Context) {
	siteURL := getSiteURL()
	now := time.Now()

	// 查询最新20篇文章
	var posts []models.Post
	db.DB.Preload("User").Preload("Node").Order("created_at DESC").Limit(20).Find(&posts)

	// 构建RSS XML
	rss := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
  <channel>
    <title>ZhuLink 竹林</title>
    <link>` + siteURL + `</link>
    <description>一个由用户分享、推荐优质资讯的社区，依靠用户共识挑出值得一读的内容</description>
    <language>zh-CN</language>
    <lastBuildDate>` + now.Format(time.RFC1123Z) + `</lastBuildDate>
    <atom:link href="` + siteURL + `/feed.xml" rel="self" type="application/rss+xml"/>
`

	// 添加文章项
	for _, post := range posts {
		// 构建文章链接
		link := fmt.Sprintf("%s/p/%s", siteURL, post.Pid)

		// 用净化过的渲染结果，不能直接输出用户原文：原文里的 "]]>" 能跳出 CDATA 注入任意 XML
		content := rssItemContent(post.Content, siteURL)
		// 添加查看更多链接
		content += fmt.Sprintf(`<p><br><a href="%s">评论也是内容的一部分，点击查看完整内容与讨论 →</a></p>`, link)

		title := escapeXML(post.Title)
		author := escapeXML(post.User.Username)

		// 添加item（使用CDATA包装HTML内容）
		rss += `    <item>
      <title>` + title + `</title>
      <link>` + link + `</link>
      <description><![CDATA[` + cdataSafe(content) + `]]></description>
      <author>` + author + `</author>
      <category>` + escapeXML(post.Node.Name) + `</category>
      <pubDate>` + post.CreatedAt.Format(time.RFC1123Z) + `</pubDate>
      <guid isPermaLink="true">` + link + `</guid>
    </item>
`
	}

	// 结束RSS
	rss += `  </channel>
</rss>`

	c.Header("Content-Type", "application/rss+xml; charset=utf-8")
	c.String(http.StatusOK, rss)
}

// escapeXML 转义XML特殊字符，并去掉 XML 不允许出现的控制字符
func escapeXML(s string) string {
	// 使用html.EscapeString处理XML转义,它能正确处理中文
	return html.EscapeString(stripInvalidXMLChars(s))
}

// rssItemContent 把帖子正文渲染成净化后的 HTML，取前 3 个块，站内相对地址补成绝对地址
func rssItemContent(markdown, siteURL string) string {
	content := utils.FirstBlocks(string(utils.RenderMarkdown(markdown)), 3)
	return strings.NewReplacer(`src="/`, `src="`+siteURL+`/`, `href="/`, `href="`+siteURL+`/`).Replace(content)
}

// cdataSafe 让内容可以安全放进 CDATA：拆开 "]]>"，去掉非法字符
func cdataSafe(s string) string {
	return strings.ReplaceAll(stripInvalidXMLChars(s), "]]>", "]]]]><![CDATA[>")
}

// stripInvalidXMLChars 去掉 XML 1.0 不允许的字符（除 \t \n \r 外的控制字符等），否则整个 feed 解析失败
func stripInvalidXMLChars(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || (r >= 0x20 && r <= 0xD7FF) || (r >= 0xE000 && r <= 0xFFFD) || r >= 0x10000 {
			return r
		}
		return -1
	}, s)
}

// IndexNowKeyFile 返回 IndexNow 验证文件内容
// 路由格式: /{apiKey}.txt
func (h *SEOHandler) IndexNowKeyFile(c *gin.Context) {
	key := c.Param("key.txt")
	// 去除 .txt 后缀
	key = strings.TrimSuffix(key, ".txt")

	indexNowService := services.GetIndexNowService()
	expectedKey := indexNowService.GetKey()

	// 如果未配置 IndexNow API Key，返回 404
	if expectedKey == "" {
		c.String(http.StatusNotFound, "Not Found IndexNow API Key")
		return
	}

	// 验证请求的 key 是否匹配配置的 key
	if key != expectedKey {
		c.String(http.StatusNotFound, "IndexNow API Key Error")
		return
	}

	// 返回 key 作为文件内容（IndexNow 协议要求文件内容就是 key 本身）
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, expectedKey)
}
