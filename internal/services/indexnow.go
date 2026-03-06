package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// IndexNowService handles URL submissions to IndexNow API
type IndexNowService struct {
	apiKey      string
	siteURL     string
	keyLocation string
	client      *http.Client
	semaphore   chan struct{} // Rate limiting semaphore
}

var indexNowService *IndexNowService
var indexNowOnce sync.Once

// GetIndexNowService returns the singleton instance of IndexNowService
func GetIndexNowService() *IndexNowService {
	indexNowOnce.Do(func() {
		siteURL := os.Getenv("SITE_URL")
		if siteURL == "" {
			siteURL = "https://zhulink.vip"
		}

		apiKey := os.Getenv("INDEXNOW_API_KEY")
		keyLocation := ""
		if apiKey != "" {
			// keyLocation 指向网站根目录的验证文件
			keyLocation = fmt.Sprintf("%s/%s.txt", siteURL, apiKey)
		}

		indexNowService = &IndexNowService{
			apiKey:      apiKey,
			siteURL:     siteURL,
			keyLocation: keyLocation,
			client:      &http.Client{Timeout: 10 * time.Second},
			semaphore:   make(chan struct{}, 3), // Limit concurrency to 3
		}
	})
	return indexNowService
}

// GetKey returns the configured API key
func (s *IndexNowService) GetKey() string {
	return s.apiKey
}

// GetKeyLocation returns the URL of the key verification file
func (s *IndexNowService) GetKeyLocation() string {
	return s.keyLocation
}

// SubmitURL asynchronously submits a single URL to IndexNow
// This is a fire-and-forget operation that runs in a goroutine
func (s *IndexNowService) SubmitURL(postPid string) {
	// Skip if API key is not configured
	if s.apiKey == "" {
		return
	}

	// Execute in goroutine to avoid blocking the main flow
	go func() {
		s.semaphore <- struct{}{}
		defer func() { <-s.semaphore }()

		postURL := fmt.Sprintf("%s/p/%s", s.siteURL, postPid)

		payload := map[string]interface{}{
			"host":        strings.TrimPrefix(s.siteURL, "https://"),
			"key":         s.apiKey,
			"keyLocation": s.keyLocation,
			"urlList":     []string{postURL},
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			log.Printf("[IndexNow] JSON marshaling failed: %v", err)
			return
		}

		resp, err := s.client.Post(
			"https://api.indexnow.org/IndexNow",
			"application/json",
			bytes.NewBuffer(jsonData),
		)

		if err != nil {
			log.Printf("[IndexNow] Submission failed: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			log.Printf("[IndexNow] Successfully submitted: %s", postURL)
		} else {
			log.Printf("[IndexNow] API returned non-200 status: %s", resp.Status)
		}
	}()
}
