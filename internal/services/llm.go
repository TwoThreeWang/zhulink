package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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
必须返回简体中文，不要返回其他语言。
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
		log.Printf("[LLM] API 请求失败: %v", err)
		return "", fmt.Errorf("api request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[LLM] API 返回非 200 状态码: %s", resp.Status)
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

// SEOMetadata 包含生成的 SEO 元数据
type SEOMetadata struct {
	Keywords    string // 逗号分隔的关键词列表
	Description string // 150 字以内的页面描述
}

// GenerateSEOMetadata 调用 LLM 生成 SEO 关键词和描述
func (s *LLMService) GenerateSEOMetadata(title, content string) (*SEOMetadata, error) {
	if s.config.Token == "" {
		return nil, fmt.Errorf("LLM_TOKEN 未配置")
	}

	// 构造提示词
	promptTemplate := `
# Role
你是一个专业的 SEO 优化专家，精通搜索引擎优化和内容营销。

# Task
基于提供的文章标题和内容，生成有利于 SEO 的关键词和页面描述。

# Output Requirements
请严格按照以下 JSON 格式返回，不要包含任何其他文字：
{"keywords":"关键词1,关键词2,关键词3,...","description":"页面描述"}

## Keywords 要求:
1. 生成 5-8 个与文章高度相关的中文关键词
2. 关键词之间用英文逗号分隔
3. 包含文章的核心主题、技术栈、行业术语

## Description 要求:
1. 100-150 个中文字符
2. 简洁概括文章的核心内容和价值
3. 语言流畅自然，适合在搜索结果中展示
4. 不要包含 emoji 或特殊符号

# Input Data
### Title: %s
### Content: %s

# Reminder
只返回 JSON，不要有任何其他文字。
`
	// 截取内容，避免过长
	contentForPrompt := content
	if len([]rune(content)) > 500 {
		contentForPrompt = string([]rune(content)[:500]) + "..."
	}

	prompt := fmt.Sprintf(promptTemplate, title, contentForPrompt)

	reqBody := ChatRequest{
		Model: s.config.Model,
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %v", err)
	}

	apiURL := strings.TrimSuffix(s.config.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("[LLM-SEO] API 请求失败: %v", err)
		return nil, fmt.Errorf("api request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[LLM-SEO] API 返回非 200 状态码: %s", resp.Status)
		return nil, fmt.Errorf("api returned non-200 status: %s", resp.Status)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response failed: %v", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	responseContent := strings.TrimSpace(chatResp.Choices[0].Message.Content)

	// 尝试提取 JSON（有时候 LLM 会在 JSON 前后添加文字）
	startIdx := strings.Index(responseContent, "{")
	endIdx := strings.LastIndex(responseContent, "}")
	if startIdx == -1 || endIdx == -1 || startIdx > endIdx {
		log.Printf("[LLM-SEO] 无法解析响应: %s", responseContent)
		return nil, fmt.Errorf("invalid JSON response")
	}
	jsonStr := responseContent[startIdx : endIdx+1]

	var seoResult SEOMetadata
	if err := json.Unmarshal([]byte(jsonStr), &seoResult); err != nil {
		log.Printf("[LLM-SEO] JSON 解析失败: %v, 原始内容: %s", err, jsonStr)
		return nil, fmt.Errorf("parse SEO metadata failed: %v", err)
	}

	return &seoResult, nil
}
