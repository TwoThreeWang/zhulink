package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type LLMConfig struct {
	BaseURL string
	Model   string
	Token   string
}

type LLMService struct {
	config LLMConfig
	client *http.Client
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

var llmService *LLMService

func GetLLMService() *LLMService {
	if llmService == nil {
		config := LLMConfig{
			BaseURL: os.Getenv("LLM_BASE_URL"),
			Model:   os.Getenv("LLM_MODEL"),
			Token:   os.Getenv("LLM_TOKEN"),
		}

		// 默认配置
		if config.BaseURL == "" {
			config.BaseURL = "https://generativelanguage.googleapis.com/v1beta/openai/"
		}
		if config.Model == "" {
			config.Model = "gemini-1.5-flash"
		}

		llmService = &LLMService{
			config: config,
			client: &http.Client{
				Timeout: 30 * time.Second,
			},
		}
	}
	return llmService
}

// GenerateSummary 调用 LLM 生成摘要
func (s *LLMService) GenerateSummary(title, content string) (string, error) {
	if s.config.Token == "" {
		return "[✨AI 摘要] 未配置 LLM_TOKEN，请在 .env 文件中配置以使用真实 AI 功能。", nil
	}

	// 构造提示词
	promptTemplate := `
# Role
你是一个专业的资讯分析师和摘要生成专家，专注于为技术和知识社区提供简洁、客观、易懂的中文摘要。

# Task
请仔细阅读以下文章的标题、内容描述或完整内容。你的任务是为 ZhuLink 社区生成一段**约 100-120 字**的**纯文本中文摘要**。

# Output Requirements (输出要求)
1.  **字数限制**: 严格控制在 **100 到 120 字之间**。如果原文非常短，可适当缩减，但不得少于 50 字。
2.  **内容类型**:
    *   针对**技术文章**：应提炼核心技术点、解决的问题、关键概念或实现方法。
    *   针对**新闻报道**：应包含事件的核心要素（时间、地点、人物、起因、结果）。
    *   针对**观点分析**：应总结主要论点和核心洞察。
3.  **语言风格**:
    *   **纯文本**: 绝对不允许包含任何 HTML 标签、Markdown 格式（如 **粗体**, ## 标题, *列表*）、表情符号、代码块或特殊字符。
    *   **中立客观**: 仅总结文章内容，不允许加入任何个人观点、评价或推荐语。
    *   **流畅自然**: 摘要应语义连贯，语言精炼，没有语病。
4.  **防范措施 (Safeguards)**:
    *   **拒绝生成**: 如果原文内容明显为**垃圾信息、广告、色情、暴力、仇恨言论或任何不适合公共社区传播的内容**，请不要生成摘要，而是直接返回字符串 "CONTENT_UNSUITABLE"。
    *   **去除冗余**: 自动过滤掉文章中常见的“免责声明”、“版权信息”、“推广信息”等与核心内容无关的部分。

# Input Data (输入数据)

### Article Title:
%s

### Article Content:
%s
`
	prompt := fmt.Sprintf(promptTemplate, title, content)

	reqBody := ChatRequest{
		Model: s.config.Model,
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %v", err)
	}

	apiURL := strings.TrimSuffix(s.config.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request failed: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("api request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api returned non-200 status: %s", resp.Status)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("decode response failed: %v", err)
	}

	if len(chatResp.Choices) > 0 {
		return "[✨AI 摘要] " + chatResp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no summary generated in response")
}
