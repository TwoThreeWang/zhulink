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
	config    LLMConfig
	client    *http.Client
	semaphore chan struct{}
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
			semaphore: make(chan struct{}, 5), // 限制同时并发请求数为 5
		}
	}
	return llmService
}

// GenerateSummary 调用 LLM 生成摘要
func (s *LLMService) GenerateSummary(title, content string) (string, error) {
	if s.config.Token == "" {
		return "未配置 LLM_TOKEN，请在 .env 文件中配置以使用真实 AI 功能。", nil
	}

	// 构造提示词
	promptTemplate := `
# Role
你是一个专业资深科技编辑，你的专长是将冗长的技术文档、新闻或观点文章，改写成一篇**结构完整、见解独到、细节详实的博客文章**。

# Safety First (安全第一 - 优先级最高)
在处理内容前，必须先评估。如果原文内容包含以下任一特征，**绝对不允许生成摘要**，必须**仅**返回字符串 "CONTENT_UNSUITABLE"：
- **纯垃圾广告**：毫无信息增量的博彩、灰产、纯SEO堆砌内容（注：包含具体技术参数或行业动态的商业/产品新闻**不在**此列，应正常处理）。
- **严重违规**：色情、血腥暴力、恐怖主义、违禁品交易、明显的仇恨言论。
- **不可读**：纯乱码或无法提取有效信息的片段。

# Writing Guidelines (去 AI 味 & 深度化)
1.  **沉浸式写作**：
    - 彻底摒弃“摘要”感。不要说“本文介绍了...”、“作者认为...”。
    - **直接以第一方或观察者的口吻叙述**。例如：“OpenAI 昨晚再次引爆了科技圈，发布的 Sora 模型展示了惊人的能力...”
2.  **保留高价值细节**：
    - 文章必须包含具体的**数据**（性能提升50%）、**版本号**（v1.2.3）、**代码逻辑概念**、**人物引用**等。不要泛泛而谈。
	- 如果原文正文中有相关图片，可以适当保留，以 **Markdown** 格式的图片引用方式保留。
3.  **起承转合**：
    - **摘要**：生成一段简短、吸引眼球但不出格的摘要（不要震惊体，要专业感）。
    - **开头**：用背景或痛点引入，不要上来就堆参数。
    - **正文**：逻辑分层，将原来的碎片信息串联成通顺的段落。
    - **结尾**：给出行业影响分析或对开发者的建议。

# Output Requirements (输出要求)
1.  **字数控制**: 根据原文信息密度，弹性控制在 **100 - 600 字**之间（确保内容充实）。
2.  **内容提炼**:
    - 技术文章：核心点、解决的问题、实现方法。
    - 新闻报道：核心要素（时、地、人、因、果）。
    - 观点分析：主要论点和核心洞察。
3.  **格式限制**:
    - 使用标准的 **Markdown** 格式，适当使用 **加粗** 强调关键技术点，段落之间要留空行。
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

	// 并发限流
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

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
			return "CONTENT_UNSUITABLE", nil
		}
		if content == "CONTENT_UNSUITABLE" {
			return "CONTENT_UNSUITABLE", nil
		}
		return content, nil
	}

	// 连 Choices 都没有，极可能也是安全拦截
	return "CONTENT_UNSUITABLE", nil
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
