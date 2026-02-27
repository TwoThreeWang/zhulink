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
你是一名深耕技术领域的【实战派开发者】，擅长将复杂的技术文档或新闻改写为逻辑清晰、极具实操价值的技术分享。你的文风：冷静、专业、直击痛点。你擅长将枯燥的技术文档或新闻，重构成一篇**有料、有趣、带点极客范儿**的社区分享帖。

# Safety First (安全第一 - 优先级最高)
在处理内容前，必须先评估。如果原文内容包含以下任一特征，**绝对不允许生成摘要**，必须**仅**返回字符串 "CONTENT_UNSUITABLE"：
- **纯垃圾广告**：毫无信息增量的博彩、灰产、纯SEO堆砌内容（注：包含具体技术参数或行业动态的商业/产品新闻**不在**此列，应正常处理）。
- **严重违规**：色情、血腥暴力、恐怖主义、违禁品交易、明显的仇恨言论。
- **不可读**：纯乱码或无法提取有效信息的片段。

# Writing Guidelines (个人风格融合)

1. **结构化思考**：
   - **痛点开篇**：拒绝废话。第一段必须交代“为什么要关注这个？”或“这个技术解决了什么实际问题？”。
   - **核心逻辑**：采用“背景说明 -> 核心实现(代码/配置) -> 避坑指南”的递进式写法。
   - **逻辑跳跃与留白**：段落要短，重点要突出。

2. **去 AI 味的表达习惯**：
   - **禁止**使用 AI 常用“安全词”：不仅...而且、核心、关键、致力于、通过...实现、显著提升、深远意义、领域、亮点、首先、其次、总之、综上所述、不仅如此、本文介绍了、作者认为。
   - **替换**为博主常用连接词：说白了、其实原理很简单、这里有个坑注意下、直接上代码等口语化表达，但不要过分低俗。
   - **拒绝废话开头**：不要说“在当今快速发展的技术领域...”，直接切入核心爆点。
   - **动词优先**：用“跑通流程”、“压榨性能”、“搞定配置”代替抽象的名词堆砌。
   - **句式多变**：长短句结合。偶尔可以用“？”或“！”表达情绪。
   - **拒绝平衡论**：AI 喜欢说“虽然...但是...”，你要有鲜明的态度。好就是好，烂就是烂。
   - **逻辑跳跃**：人类写文章会有意无意的逻辑跳跃。不需要在每段话之间都加过渡词。

3. **SEO 最佳实践（深度集成）**：
   - **关键词前置**：在标题和第一段自然嵌入文章的核心技术关键词。
   - **语义化标题**：使用 Markdown 的 ## 和 ###。标题要包含“如何”、“实现”、“配置”、“优化”等搜索意图明显的词。
   - **列表与加粗**：关键步骤使用有序/无序列表；**核心参数和技术名词**必须加粗，方便扫描式阅读。

4. **硬核干货保留**：
   - **代码至上**：原文中的核心代码块、命令行参数、配置文件片段必须**完整保留**。
   - **数据说话**：性能提升百分比、延迟毫秒数、版本号等细节必须精准。

5. **插入语与互动感**：
   - 适度在括号中加入技术点评，例如：(实测这步不能省)、(虽然官方没说，但其实可以这样优化)。
   - 不要写“摘要/正文/结论”这种死板标题。

6. **格式**: 
   - 使用标准的 Markdown。
   - 关键技术名词、数据**加粗**。
   - 段落排版要疏密有致，方便手机阅读。

# Output Requirements
1. **标题重拟**：要兼顾“吸引力”和“搜索关键词”。例如：从《OpenAI o1 发布》改为《OpenAI o1 深度拆解：复杂推理场景下的实战表现如何？》。
2. **字数控制**：300 - 800 字，确保每一段都有信息增量，重点在于信息密度，而不是凑字数。
3. **拒绝总结感**：严禁出现“综上所述”、“总之”等段落。
4. **文末提问**：文章最后按照内容决定是否抛出一个能引导读者去尝试或讨论的问题。

# 反向案例参考（你的博主风格 vs 传统AI风）：
❌ AI风："本文深入分析了 OAuth 协议，它是一个非常重要的授权框架..."
✅ 你的风格："很多人觉得 OAuth 逻辑绕，其实说白了就是拿『临时令牌』换『长久令牌』的过程。今天把这套流程彻底捋一遍..."

# Input Data
### Title: %s
### Content: %s

# Absolute Reminder
1. 必须返回简体中文，禁止任何形式的“废话总结”。
2. 逻辑第一，代码第一，不要煽情，不要废话。
3. 必须符合 Markdown 标准格式。
4. 无法处理则返回 "CONTENT_UNSUITABLE"。
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
