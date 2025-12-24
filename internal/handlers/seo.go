package handlers

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/models"

	"github.com/gin-gonic/gin"
)

type SEOHandler struct{}

func NewSEOHandler() *SEOHandler {
	return &SEOHandler{}
}

// getSiteURL 从环境变量获取网站URL,如果未设置则使用默认值
func getSiteURL() string {
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "https://zhulink.com"
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

# 爬取延迟(可选,避免服务器压力)
Crawl-delay: 1
`, siteURL)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, content)
}

// SitemapXML 动态生成sitemap.xml
func (h *SEOHandler) SitemapXML(c *gin.Context) {
	siteURL := getSiteURL()
	now := time.Now().Format("2006-01-02")

	// 开始构建XML
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
`

	// 1. 首页 - 最高优先级,每天更新
	xml += fmt.Sprintf(`  <url>
    <loc>%s/</loc>
    <lastmod>%s</lastmod>
    <changefreq>daily</changefreq>
    <priority>1.0</priority>
  </url>
`, siteURL, now)

	// 2. 热门页
	xml += fmt.Sprintf(`  <url>
    <loc>%s/</loc>
    <lastmod>%s</lastmod>
    <changefreq>hourly</changefreq>
    <priority>0.9</priority>
  </url>
`, siteURL, now)

	// 3. 最新页
	xml += fmt.Sprintf(`  <url>
    <loc>%s/new</loc>
    <lastmod>%s</lastmod>
    <changefreq>hourly</changefreq>
    <priority>0.9</priority>
  </url>
`, siteURL, now)

	// 4. 节点列表页
	xml += fmt.Sprintf(`  <url>
    <loc>%s/nodes</loc>
    <lastmod>%s</lastmod>
    <changefreq>weekly</changefreq>
    <priority>0.8</priority>
  </url>
`, siteURL, now)

	// 5. 搜索页
	xml += fmt.Sprintf(`  <url>
    <loc>%s/search</loc>
    <lastmod>%s</lastmod>
    <changefreq>weekly</changefreq>
    <priority>0.7</priority>
  </url>
`, siteURL, now)

	// 6. RSS苗圃页
	xml += fmt.Sprintf(`  <url>
    <loc>%s/rss</loc>
    <lastmod>%s</lastmod>
    <changefreq>daily</changefreq>
    <priority>0.8</priority>
  </url>
`, siteURL, now)

	// 7. 所有节点页面
	var nodes []models.Node
	db.DB.Find(&nodes)
	for _, node := range nodes {
		xml += fmt.Sprintf(`  <url>
    <loc>%s/t/%s</loc>
    <lastmod>%s</lastmod>
    <changefreq>daily</changefreq>
    <priority>0.7</priority>
  </url>
`, siteURL, node.Name, now)
	}

	// 8. 最近的文章详情页(限制500篇,避免sitemap过大)
	var posts []models.Post
	db.DB.Order("created_at DESC").Limit(500).Find(&posts)
	for _, post := range posts {
		lastmod := post.UpdatedAt.Format("2006-01-02")
		// 根据文章新旧程度调整优先级
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

		xml += fmt.Sprintf(`  <url>
    <loc>%s/p/%s</loc>
    <lastmod>%s</lastmod>
    <changefreq>%s</changefreq>
    <priority>%.1f</priority>
  </url>
`, siteURL, post.Pid, lastmod, changefreq, priority)
	}

	// 结束XML
	xml += `</urlset>`

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(http.StatusOK, xml)
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
		// 截取内容前300字符
		content := post.Content
		runes := []rune(content)
		if len(runes) > 300 {
			content = string(runes[:300]) + "..."
		}

		// 转义XML特殊字符
		content = escapeXML(content)
		title := escapeXML(post.Title)
		author := escapeXML(post.User.Username)

		// 构建文章链接
		link := fmt.Sprintf("%s/p/%s", siteURL, post.Pid)

		// 添加item
		rss += `    <item>
      <title>` + title + `</title>
      <link>` + link + `</link>
      <description>` + content + `</description>
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

// escapeXML 转义XML特殊字符
func escapeXML(s string) string {
	// 使用html.EscapeString处理XML转义,它能正确处理中文
	return html.EscapeString(s)
}
