package services

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	readability "github.com/go-shiori/go-readability"
	"github.com/microcosm-cc/bluemonday"
)

// CrawlerService 网页内容抓取服务
type CrawlerService struct {
	client    *http.Client
	sanitizer *bluemonday.Policy
}

// NewCrawlerService 创建抓取服务实例
func NewCrawlerService() *CrawlerService {
	return &CrawlerService{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		sanitizer: bluemonday.UGCPolicy(), // 允许用户生成内容的安全策略
	}
}

// 全局单例
var crawlerService *CrawlerService

// GetCrawlerService 获取 Crawler 服务单例
func GetCrawlerService() *CrawlerService {
	if crawlerService == nil {
		crawlerService = NewCrawlerService()
	}
	return crawlerService
}

// FetchArticleContent 从 URL 抓取正文内容
// 使用 go-readability 提取正文，然后用 bluemonday 清洗
func (s *CrawlerService) FetchArticleContent(url string) (string, error) {
	// 1. 发送 HTTP 请求获取 HTML
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置 User-Agent 模拟浏览器
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 2. 使用 go-readability 提取正文
	article, err := readability.FromReader(strings.NewReader(string(body)), nil)
	if err != nil {
		return "", fmt.Errorf("解析正文失败: %w", err)
	}

	// 3. 使用 bluemonday 清洗 HTML（移除潜在的恶意内容）
	cleanContent := s.sanitizer.Sanitize(article.Content)

	return cleanContent, nil
}

// FetchWithFallback 尝试抓取内容，失败时返回空字符串而不是错误
func (s *CrawlerService) FetchWithFallback(url string) string {
	content, err := s.FetchArticleContent(url)
	if err != nil {
		return ""
	}
	return content
}
