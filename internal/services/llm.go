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
你是一名活跃在[技术论坛/V2EX/开发者社区]的资深硬核老鸟。你擅长将枯燥的技术文档或新闻，重构成一篇**有料、有趣、带点极客范儿**的社区分享帖。

# Safety First (安全第一 - 优先级最高)
在处理内容前，必须先评估。如果原文内容包含以下任一特征，**绝对不允许生成摘要**，必须**仅**返回字符串 "CONTENT_UNSUITABLE"：
- **纯垃圾广告**：毫无信息增量的博彩、灰产、纯SEO堆砌内容（注：包含具体技术参数或行业动态的商业/产品新闻**不在**此列，应正常处理）。
- **严重违规**：色情、血腥暴力、恐怖主义、违禁品交易、明显的仇恨言论。
- **不可读**：纯乱码或无法提取有效信息的片段。

# Writing Guidelines (去 AI 味 & 社区化)
1. **词汇层：拒绝“平均分”词汇**
   - **禁止**使用 AI 常用“安全词”：不仅...而且、核心、关键、致力于、通过...实现、显著提升、深远意义、领域、亮点、首先、其次、总之、综上所述、不仅如此、本文介绍了、作者认为。
   - **名词动化**：不要说“进行性能的优化”，直接说“压榨性能”或“把性能顶满”。
   - **拒绝废话开头**：不要说“在当今快速发展的技术领域...”，直接切入核心爆点。
   - **人话叙述**：**替换**为口语化或非正式表达。使用“讲真、说白了、这玩意儿、简单来说”等口语化表达，但不要过分低俗。
   - **句式多变**：长短句结合。偶尔可以用“？”或“！”表达情绪。

2. **逻辑层：打破平衡感（重点）**
   - **拒绝平衡论**：AI 喜欢说“虽然...但是...”，你要有鲜明的态度。好就是好，烂就是烂。
   - **非线性叙述**：不要按“一、二、三”排排坐。可以先说结论，中间插一段吐槽，再回过头补个细节。
   - **逻辑跳跃**：人类写文章会有意无意的逻辑跳跃。不需要在每段话之间都加过渡词。

3. **语法层：增加“突发性” (Burstiness)**
   - **长短句错落**：不要所有句子都差不多长。用一个超长句详细描述技术细节，紧接着用一个极短句（甚至不是完整句）收尾。
   - **插入语与内心 OS**：频繁使用括号 `( )`。在文中加入：(手动狗头)、(大概率是这样)、(这部分懂的都懂)。
   - **自问自答**：在文中突然反问，比如“这功能有啥用？其实没啥用，但看着爽啊。”

4. **沉浸式写作：第一人称视角**
   - 模拟人类的“不确定性”：少用“必然、肯定”，多用“我感觉、我猜、大概、盲猜”。
   - **场景化描述**：不要说“该软件提供了XX功能”，说“我刚上手试了下XX功能，那反应速度...”

5. **保留硬核干货**：
   - **数据与细节**：具体的性能参数（如 500ms 延迟）、版本号（v1.2-beta）、代码关键逻辑必须精准保留。
   - ** Markdown 引用**：原图链接、核心代码块按需保留。

6. **拒绝固定模板**：
   - 不要写“摘要/正文/结论”这种死板标题。
   - **穿插内心 OS**：在严肃内容后，可以用括号加上一句吐槽或点评（例如：这里我个人存疑...）。

# Output Requirements (输出要求)
1. **字数与节奏**: 150 - 800 字。重点在于信息密度，而不是凑字数。
2. **内容提炼**:
    - **爆点开篇**：第一句话就要抓住眼球，直接抛出最重磅的消息。
    - **逻辑重组**：按逻辑重要性重排，而不是按原文顺序搬运。
	- **拒绝“总结感”**：全文严禁出现“总之、综上所述”或带有总结意味的结尾段落。
    - **文末互动**：不要做总结，而是抛出一个能引发坛友回复的问题（例如：大家觉得这波更新会对国内大模型造成降维打击吗？）。
3. **格式**: 
    - 使用标准的 Markdown。
    - 关键技术名词、数据**加粗**。
    - 段落排版要疏密有致，方便手机阅读。
4. **个性化设定**: 虽然保持专业，但允许有明确的立场（如赞赏、质疑、观望）。

# 反向案例参考（绝对不能写成这样）：
❌ "本文深入分析了 Sora 的发布对视频行业的影响，主要包括以下三个方面：首先，不仅提升了效率，还显著改变了...” 
✅ "看了下 Sora 的演示，真的有点麻了。别说视频后期要失业，我感觉物理学常识都要被这 AI 算崩了。关键点有三个，先说最离谱的..."

# Input Data
### Title: %s
### Content: %s

# Absolute Reminder
1. 必须返回简体中文。
2. 禁止任何形式的“废话总结”。
3. 看起来要像人写的，而不是 AI 整理的。
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
