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

# Safety First (安全第一 - 优先级最高)
在处理内容前，必须先评估。如果原文内容包含以下任一特征，**绝对不允许生成摘要**，必须**仅**返回字符串 "CONTENT_UNSUITABLE"：
- 垃圾信息、广告、推广内容
- 色情、暴力、血腥或恐怖内容
- 仇恨言语、歧视或人身攻击
- 任何不适合在纯净技术社区传播的违规内容
- 原文内容乱码或无法理解

# Task
为 ZhuLink 社区生成一段约 100-120 字的纯文本中文摘要。

# Output Requirements (输出要求)
1.  **字数控制**: 严格在 **100 到 120 字**之间（原文极短除外，但不少于 50 字）。
2.  **内容提炼**:
    - 技术文章：核心点、解决的问题、实现方法。
    - 新闻报道：核心要素（时、地、人、因、果）。
    - 观点分析：主要论点和核心洞察。
3.  **格式限制**:
    - **必须是纯文本**：禁止 HTML、Markdown（粗体、标题、列表等）、表情符号、代码块。
    - **中立客观**：不加入个人评价或推荐。
    - **去除干扰**：自动过滤免责声明、版权、广告等无关内容。

# Input Data
### Title: %s
### Content: %s

# Absolute Reminder
如果你因任何原因无法提供摘要，请务必返回 "CONTENT_UNSUITABLE"。
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
		content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
		if content == "" {
			// 如果返回内容为空，通常是由于 API 安全拦截，降级处理
			return "[✨AI 摘要] CONTENT_UNSUITABLE", nil
		}
		if content == "CONTENT_UNSUITABLE" {
			return "[✨AI 摘要] CONTENT_UNSUITABLE", nil
		}
		return "[✨AI 摘要] " + content, nil
	}

	// 连 Choices 都没有，极可能也是安全拦截
	return "[✨AI 摘要] CONTENT_UNSUITABLE", nil
}
