package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"zhulink/internal/services"

	"github.com/gin-gonic/gin"
)

// 盗链提醒 SVG 图片
const hotlinkSVG = `<svg width="200" height="200" xmlns="http://www.w3.org/2000/svg">
  <rect width="100%" height="100%" fill="#f8f9fa"/>
  <text x="50%" y="50%" font-family="Arial" font-size="14" fill="#6c757d" text-anchor="middle">
    仅限 ZhuLink 论坛内部使用
  </text>
  <text x="50%" y="70%" font-family="Arial" font-size="12" fill="#adb5bd" text-anchor="middle">
    zhulink.vip
  </text>
</svg>`

// ImageHandler 图片处理 Handler
type ImageHandler struct{}

// NewImageHandler 创建 ImageHandler 实例
func NewImageHandler() *ImageHandler {
	return &ImageHandler{}
}

// Upload 处理图片上传请求 (POST /api/upload)
// 需要用户已登录
func (h *ImageHandler) Upload(c *gin.Context) {
	// 检查用户是否登录（由 middleware.AuthRequired() 保证）

	// 获取上传的文件
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请选择要上传的图片",
		})
		return
	}
	defer file.Close()

	// 验证文件类型
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "只允许上传图片文件",
		})
		return
	}

	// 验证文件大小（限制 10MB）
	if header.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "图片大小不能超过 10MB",
		})
		return
	}

	// 上传到 Imgur
	result, err := services.UploadImage(file, header)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("上传失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"url":     result.URL,
		"id":      result.ID,
	})
}

// Proxy 反代 Imgur 图片 (GET /img/:id)
// 使用 Sec-Fetch-* 头部检测盗链
func (h *ImageHandler) Proxy(c *gin.Context) {
	imageID := c.Param("id")
	if imageID == "" {
		c.String(http.StatusBadRequest, "缺少图片 ID")
		return
	}

	// 防盗链检测：使用 Sec-Fetch-* 头部
	if !isAllowedRequest(c) {
		// 返回盗链提醒 SVG
		c.Header("Content-Type", "image/svg+xml")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.String(http.StatusOK, hotlinkSVG)
		return
	}

	// 解析图片 ID 和扩展名
	ext := filepath.Ext(imageID)
	id := strings.TrimSuffix(imageID, ext)
	if ext == "" {
		ext = ".jpg" // 默认扩展名
	}

	// 构建 Imgur URL
	imgurURL := fmt.Sprintf("https://i.imgur.com/%s%s", id, ext)

	// 请求 Imgur 图片
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", imgurURL, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "请求创建失败")
		return
	}

	// 设置请求头，模拟浏览器
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "获取图片失败")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.String(resp.StatusCode, "图片不存在")
		return
	}

	// 设置响应头
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}

	// 缓存控制：缓存 7 天
	c.Header("Cache-Control", "public, max-age=604800")
	c.Header("Vary", "Sec-Fetch-Site, Sec-Fetch-Mode")

	// 流式复制响应
	c.Status(http.StatusOK)
	io.Copy(c.Writer, resp.Body)
}

// isAllowedRequest 使用 Sec-Fetch-* 头部检测是否为合法请求
// 现代浏览器会自动发送这些头部，无法伪造
func isAllowedRequest(c *gin.Context) bool {
	secFetchSite := c.GetHeader("Sec-Fetch-Site")
	secFetchMode := c.GetHeader("Sec-Fetch-Mode")

	// 允许的情况：
	// 1. 没有 Sec-Fetch-* 头部（旧浏览器或直接访问）
	if secFetchSite == "" {
		return true
	}

	// 2. same-origin: 同源请求
	if secFetchSite == "same-origin" {
		return true
	}

	// 3. same-site: 同站请求
	if secFetchSite == "same-site" {
		return true
	}

	// 4. none: 用户直接在地址栏输入或从书签访问
	if secFetchSite == "none" {
		return true
	}

	// 5. navigate 模式: 用户导航到页面（允许在新标签页打开图片）
	if secFetchMode == "navigate" {
		return true
	}

	// 其他情况（cross-site 且非导航）视为盗链
	return false
}
