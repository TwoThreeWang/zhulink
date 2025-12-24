package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGenerateSummary(t *testing.T) {
	// 模拟 API 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}

		resp := ChatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{Content: "这是一个测试摘要"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 设置环境变量
	os.Setenv("LLM_BASE_URL", server.URL)
	os.Setenv("LLM_TOKEN", "test-token")
	os.Setenv("LLM_MODEL", "test-model")

	// 获取服务（重置单例以便重新加载配置）
	llmService = nil
	s := GetLLMService()

	// 测试正常生成
	summary, err := s.GenerateSummary("测试标题", "测试内容")
	if err != nil {
		t.Fatalf("GenerateSummary failed: %v", err)
	}
	expected := "[AI 摘要] 这是一个测试摘要"
	if summary != expected {
		t.Errorf("Expected %s, got %s", expected, summary)
	}

	// 测试违规内容
	summary, err = s.GenerateSummary("违规标题", "测试内容")
	if err != nil {
		t.Fatalf("GenerateSummary failed: %v", err)
	}
	if summary != "CONTENT_UNSUITABLE" {
		t.Errorf("Expected CONTENT_UNSUITABLE, got %s", summary)
	}
}
