package services

import (
	"fmt"
	"log"
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
	return &RSSFetcher{
		parser: gofeed.NewParser(),
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

// DiscoverFeed 从 RSS URL 发现订阅源元信息
func (f *RSSFetcher) DiscoverFeed(rssURL string) (*models.Feed, error) {
	feed, err := f.parser.ParseURL(rssURL)
	if err != nil {
		return nil, fmt.Errorf("解析 RSS 失败: %w", err)
	}

	// 尝试获取图标
	iconURL := ""
	if feed.Image != nil {
		iconURL = feed.Image.URL
	}

	return &models.Feed{
		URL:     rssURL,
		Title:   feed.Title,
		IconURL: iconURL,
	}, nil
}

// ParseAndStoreFeed 解析 RSS URL 并存储条目
func (f *RSSFetcher) ParseAndStoreFeed(feedID uint, rssURL string) error {
	feed, err := f.parser.ParseURL(rssURL)
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
// 如果 URL 已存在则返回现有的，否则创建新的
func (f *RSSFetcher) CreateOrGetFeed(rssURL string) (*models.Feed, error) {
	// 先检查是否已存在
	var existingFeed models.Feed
	if err := db.DB.Where("url = ?", rssURL).First(&existingFeed).Error; err == nil {
		return &existingFeed, nil
	}

	// 发现新订阅源
	feed, err := f.DiscoverFeed(rssURL)
	if err != nil {
		return nil, err
	}

	// 保存到数据库
	if err := db.DB.Create(feed).Error; err != nil {
		return nil, fmt.Errorf("保存订阅源失败: %w", err)
	}

	// 异步抓取文章
	go func() {
		if err := f.ParseAndStoreFeed(feed.ID, rssURL); err != nil {
			log.Printf("抓取订阅源文章失败: %v", err)
		}
	}()

	return feed, nil
}
