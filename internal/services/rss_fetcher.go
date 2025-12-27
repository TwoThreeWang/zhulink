package services

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/models"

	"github.com/mmcdole/gofeed"
)

// RSSFetcher RSS 订阅源抓取服务
type RSSFetcher struct {
	parser *gofeed.Parser
}

// NewRSSFetcher 创建 RSS 抓取服务实例
func NewRSSFetcher() *RSSFetcher {
	// 创建自定义 HTTP 客户端，设置超时
	httpClient := &http.Client{
		Timeout: 30 * time.Second, // 30 秒超时
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 2,
		},
	}

	parser := gofeed.NewParser()
	parser.Client = httpClient

	return &RSSFetcher{
		parser: parser,
	}
}

// 全局单例
var rssFetcher *RSSFetcher

// GetRSSFetcher 获取 RSS 抓取服务单例
func GetRSSFetcher() *RSSFetcher {
	if rssFetcher == nil {
		rssFetcher = NewRSSFetcher()
	}
	return rssFetcher
}

// normalizeRSSURL 规范化 RSS URL
// 如果 URL 以 rsshub:// 开头,则替换为自定义的 RSSHub 实例地址
func normalizeRSSURL(rssURL string) string {
	if strings.HasPrefix(rssURL, "rsshub://") {
		// 从环境变量获取 RSSHub 实例地址
		rsshubInstance := os.Getenv("RSSHUB_INSTANCE_URL")
		if rsshubInstance == "" {
			// 默认使用官方实例
			rsshubInstance = "https://rsshub.app"
		}

		// 移除 rsshub:// 前缀,替换为实例地址
		path := strings.TrimPrefix(rssURL, "rsshub://")
		// 确保实例地址不以 / 结尾
		rsshubInstance = strings.TrimSuffix(rsshubInstance, "/")
		return rsshubInstance + "/" + path
	}
	return rssURL
}

// DiscoverFeed 从 RSS URL 发现订阅源元信息
func (f *RSSFetcher) DiscoverFeed(rssURL string) (*models.Feed, error) {
	// 保存原始 URL (可能包含 rsshub:// 前缀)
	originalURL := rssURL

	// 规范化 URL 用于实际请求 (处理 rsshub:// 前缀)
	actualURL := normalizeRSSURL(rssURL)

	feed, err := f.parser.ParseURL(actualURL)
	if err != nil {
		return nil, fmt.Errorf("解析 RSS 失败: %w", err)
	}

	// 尝试获取图标
	iconURL := ""
	if feed.Image != nil {
		iconURL = feed.Image.URL
	}

	return &models.Feed{
		URL:     originalURL, // 保存原始 URL 到数据库
		Title:   feed.Title,
		IconURL: iconURL,
	}, nil
}

// ParseAndStoreFeed 解析 RSS URL 并存储条目
func (f *RSSFetcher) ParseAndStoreFeed(feedID uint, rssURL string) error {
	// 规范化 URL 用于实际请求 (处理 rsshub:// 前缀)
	actualURL := normalizeRSSURL(rssURL)

	feed, err := f.parser.ParseURL(actualURL)
	if err != nil {
		return fmt.Errorf("解析 RSS 失败: %w", err)
	}

	for _, item := range feed.Items {
		// 检查 GUID 是否已存在
		var exists int64
		guid := item.GUID
		if guid == "" {
			guid = item.Link // 如果没有 GUID，使用 Link 作为唯一标识
		}

		db.DB.Model(&models.FeedItem{}).Where("guid = ?", guid).Count(&exists)
		if exists > 0 {
			continue // 跳过已存在的条目
		}

		// 解析发布时间
		publishedAt := time.Now()
		if item.PublishedParsed != nil {
			publishedAt = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			publishedAt = *item.UpdatedParsed
		}

		// 获取内容
		// 优先使用 content:encoded，其次是 description
		content := ""
		if item.Content != "" {
			content = item.Content
		}

		description := ""
		if item.Description != "" {
			description = item.Description
		}

		feedItem := models.FeedItem{
			FeedID:      feedID,
			GUID:        guid,
			Title:       item.Title,
			Link:        item.Link,
			Description: description,
			Content:     content,
			PublishedAt: publishedAt,
		}

		if err := db.DB.Create(&feedItem).Error; err != nil {
			log.Printf("存储 RSS 条目失败: %v", err)
			continue
		}
	}

	// 更新 Feed 的最后抓取时间
	now := time.Now()
	db.DB.Model(&models.Feed{}).Where("id = ?", feedID).Update("last_fetch_at", &now)

	return nil
}

// RefreshFeed 刷新单个订阅源
func (f *RSSFetcher) RefreshFeed(feed *models.Feed) error {
	return f.ParseAndStoreFeed(feed.ID, feed.URL)
}

// RefreshAllFeeds 刷新所有订阅源（可用于后台定时任务）
func (f *RSSFetcher) RefreshAllFeeds() {
	var feeds []models.Feed
	db.DB.Find(&feeds)

	for _, feed := range feeds {
		if err := f.RefreshFeed(&feed); err != nil {
			log.Printf("刷新订阅源 %s 失败: %v", feed.Title, err)
		}
	}
}

// CreateOrGetFeed 创建或获取订阅源
// 如果 URL 已存在则返回现有的,否则创建新的
func (f *RSSFetcher) CreateOrGetFeed(rssURL string) (*models.Feed, error) {
	// 先检查是否已存在 (使用原始 URL 检查)
	var existingFeed models.Feed
	if err := db.DB.Where("url = ?", rssURL).First(&existingFeed).Error; err == nil {
		return &existingFeed, nil
	}

	// 发现新订阅源 (DiscoverFeed 内部会处理 rsshub:// 转换)
	feed, err := f.DiscoverFeed(rssURL)
	if err != nil {
		return nil, err
	}

	// 保存到数据库
	if err := db.DB.Create(feed).Error; err != nil {
		return nil, fmt.Errorf("保存订阅源失败: %w", err)
	}

	// 异步抓取文章 (传递原始 URL,ParseAndStoreFeed 内部会处理转换)
	go func() {
		if err := f.ParseAndStoreFeed(feed.ID, feed.URL); err != nil {
			log.Printf("抓取订阅源文章失败: %v", err)
		}
	}()

	return feed, nil
}

// StartScheduledFetch 启动定时拉取任务
// 每 30 分钟自动拉取所有订阅源的新文章
func (f *RSSFetcher) StartScheduledFetch() {
	ticker := time.NewTicker(30 * time.Minute)
	go func() {
		// 启动时立即执行一次
		log.Println("开始首次 RSS 订阅源拉取...")
		f.RefreshAllFeeds()
		log.Println("首次 RSS 订阅源拉取完成")

		// 然后按定时器执行
		for range ticker.C {
			log.Println("开始定时 RSS 订阅源拉取...")
			f.RefreshAllFeeds()
			log.Println("定时 RSS 订阅源拉取完成")
		}
	}()
}

// CleanupOldItems 清除发布时间超过 30 天的文章
func (f *RSSFetcher) CleanupOldItems() error {
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	result := db.DB.Where("published_at < ?", thirtyDaysAgo).Delete(&models.FeedItem{})

	if result.Error != nil {
		return fmt.Errorf("清除过期文章失败: %w", result.Error)
	}

	log.Printf("已清除 %d 篇超过 30 天的 RSS 文章", result.RowsAffected)
	return nil
}

// StartScheduledCleanup 启动定时清除任务
// 每天凌晨 2 点清除超过 30 天的文章
func (f *RSSFetcher) StartScheduledCleanup() {
	go func() {
		for {
			// 计算到下一个凌晨 2 点的时间
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			duration := next.Sub(now)

			log.Printf("下次 RSS 文章清除将在 %s 后执行 (预计时间: %s)", duration, next.Format("2006-01-02 15:04:05"))
			time.Sleep(duration)

			log.Println("开始清除过期 RSS 文章...")
			if err := f.CleanupOldItems(); err != nil {
				log.Printf("清除过期 RSS 文章失败: %v", err)
			} else {
				log.Println("清除过期 RSS 文章完成")
			}
		}
	}()
}
