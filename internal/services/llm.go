package services

import (
	"strings"
	"time"
)

type LLMService struct{}

var llmService *LLMService

func GetLLMService() *LLMService {
	if llmService == nil {
		llmService = &LLMService{}
	}
	return llmService
}

// GenerateSummary 模拟调用 LLM 生成摘要
func (s *LLMService) GenerateSummary(title, content string) (string, error) {
	// 模拟耗时 2 秒
	time.Sleep(2 * time.Second)

	// 特殊触发逻辑：如果标题包含 "违规" 字样，返回 CONTENT_UNSUITABLE
	if strings.Contains(title, "违规") || strings.Contains(content, "违规") {
		return "CONTENT_UNSUITABLE", nil
	}

	// 简单的 Mock 逻辑：返回一个固定的摘要前缀 + 标题
	return "[AI 摘要] 这篇文章主要讨论了：" + title + "。内容涵盖了该领域的关键点，并提供了深入的见解。", nil
}
