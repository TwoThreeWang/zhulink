package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
		sanitizer: bluemonday.UGCPolicy(),
	}
}

var (
	crawlerService *CrawlerService
	crawlerOnce    sync.Once
)

// GetCrawlerService 获取 Crawler 服务单例
func GetCrawlerService() *CrawlerService {
	crawlerOnce.Do(func() {
		crawlerService = NewCrawlerService()
	})
	return crawlerService
}

// FetchArticleContent 从 URL 抓取正文内容
// 优先直接抓取 + readability 提取；失败则通过 fetchoo API 兜底
func (s *CrawlerService) FetchArticleContent(articleURL string) (string, error) {
	// 1. 直接抓取
	content, err := s.fetchDirect(articleURL)
	if err == nil && content != "" {
		return content, nil
	}

	// 2. fetchoo API 兜底
	content, err = s.fetchViaProxy(articleURL)
	if err == nil && content != "" {
		return content, nil
	}

	return "", fmt.Errorf("两种方式均抓取失败")
}

// fetchDirect 直接 HTTP GET + readability 提取
func (s *CrawlerService) fetchDirect(articleURL string) (string, error) {
	req, err := http.NewRequest("GET", articleURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	article, err := readability.FromReader(strings.NewReader(string(body)), nil)
	if err != nil {
		return "", fmt.Errorf("解析正文失败: %w", err)
	}

	return s.sanitizer.Sanitize(article.Content), nil
}

// fetchViaProxy 通过 fetchoo API 代理抓取
func (s *CrawlerService) fetchViaProxy(articleURL string) (string, error) {
	apiURL := "https://fetchoo.c2v2.com/fetch?url=" + url.QueryEscape(articleURL)

	resp, err := s.client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("代理请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("代理返回状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取代理响应失败: %w", err)
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析代理响应失败: %w", err)
	}

	if result.Code != 200 || result.Data.Content == "" {
		return "", fmt.Errorf("代理返回无效内容")
	}

	return s.sanitizer.Sanitize(result.Data.Content), nil
}

// FetchWithFallback 尝试抓取内容，失败时返回空字符串而不是错误
func (s *CrawlerService) FetchWithFallback(url string) string {
	content, err := s.FetchArticleContent(url)
	if err != nil {
		return ""
	}
	return content
}
