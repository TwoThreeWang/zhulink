// LLM 连通性测试脚本
//
// 使用方法:
//
//	go run cmd/test_llm/main.go
//
// 测试内容:
//   - 检查 CF Gateway 配置是否正确
//   - 测试 GenerateSummary 接口（摘要生成）
//   - 测试 GenerateSEOMetadata 接口（SEO 元数据生成）
//
// 未配置 CF Gateway 时会自动回退到原 LLM 接口（LLM_BASE_URL / LLM_TOKEN）
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"zhulink/internal/services"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	fmt.Println("=== LLM 配置 ===")
	cfURL := os.Getenv("CF_GATEWAY_URL")
	cfModel := os.Getenv("CF_GATEWAY_MODEL")
	cfToken := os.Getenv("CF_API_TOKEN")

	if cfURL != "" && cfToken != "" {
		fmt.Printf("模式: Cloudflare AI Gateway\n")
		fmt.Printf("CF_GATEWAY_URL:   %s\n", cfURL)
		fmt.Printf("CF_GATEWAY_MODEL: %s\n", cfModel)
		fmt.Printf("CF_API_TOKEN:     %s\n", maskToken(cfToken))
	} else {
		fmt.Printf("模式: 原始 LLM 接口（Legacy）\n")
		fmt.Printf("LLM_BASE_URL: %s\n", os.Getenv("LLM_BASE_URL"))
		fmt.Printf("LLM_MODEL:    %s\n", os.Getenv("LLM_MODEL"))
		fmt.Printf("LLM_TOKEN:    %s\n", maskToken(os.Getenv("LLM_TOKEN")))
	}
	fmt.Println()

	llm := services.GetLLMService()

	// 测试 GenerateSummary
	fmt.Println("=== 测试 GenerateSummary ===")
	start := time.Now()
	summary, err := llm.GenerateSummary("Go 1.22 发布", "Go 1.22 带来了 range over integers 和新的 for 循环语义。")
	elapsed := time.Since(start)
	if err != nil {
		fmt.Printf("失败: %v (耗时 %v)\n", err, elapsed)
	} else {
		fmt.Printf("成功 (耗时 %v)\n", elapsed)
		fmt.Printf("结果: %s\n\n", truncate(summary, 200))
	}

	// 测试 GenerateSEOMetadata
	fmt.Println("=== 测试 GenerateSEOMetadata ===")
	start = time.Now()
	seo, err := llm.GenerateSEOMetadata("Go 1.22 发布", "Go 1.22 带来了 range over integers 和新的 for 循环语义。")
	elapsed = time.Since(start)
	if err != nil {
		fmt.Printf("失败: %v (耗时 %v)\n", err, elapsed)
	} else {
		fmt.Printf("成功 (耗时 %v)\n", elapsed)
		fmt.Printf("Keywords:    %s\n", seo.Keywords)
		fmt.Printf("Description: %s\n", seo.Description)
	}
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
