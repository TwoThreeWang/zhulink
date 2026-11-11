package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"zhulink/internal/middleware"
	"zhulink/internal/models"
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

const (
	maxUploadBytes = 10 << 20 // 上传单张图片的上限
	maxProxyBytes  = 25 << 20 // 代理单张图片最多转发的字节数
)

// 上传会消耗 Imgur 的 API 配额：每个用户 10 分钟 20 张、每天 100 张
var (
	uploadShortLimiter = middleware.NewLimiter(20, 10*time.Minute)
	uploadDayLimiter   = middleware.NewLimiter(100, 24*time.Hour)
)

// 允许上传和代理的图片类型（不含 SVG：SVG 里可以带脚本）
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

// imgurIDPattern Imgur 图片 ID 只有字母数字；不校验的话 id 里的 ../ 会被拼进上游地址
var imgurIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{5,12}$`)

var proxyImageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}

// imgurClient 不跟随跳转：Imgur 对已删除的图片会 302 到别的地址，只转发 i.imgur.com 直接返回的内容
var imgurClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// Upload 处理图片上传请求 (POST /api/upload)
// 需要用户已登录
func (h *ImageHandler) Upload(c *gin.Context) {
	// 检查用户是否登录（由 middleware.AuthRequired() 保证）
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	fail := func(status int, msg string) {
		c.JSON(status, gin.H{"success": false, "error": msg})
	}

	if user.Status != 0 {
		fail(http.StatusForbidden, "您处于禁言状态，暂时无法上传图片")
		return
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		fail(http.StatusBadRequest, "请选择要上传的图片（不超过 10MB）")
		return
	}
	defer file.Close()

	// 验证文件大小（限制 10MB）
	if header.Size > maxUploadBytes {
		fail(http.StatusBadRequest, "图片大小不能超过 10MB")
		return
	}

	// 验证文件类型：看文件内容的开头，不信任客户端自己报的 Content-Type
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	if !allowedImageTypes[http.DetectContentType(head[:n])] {
		fail(http.StatusBadRequest, "只支持 JPG、PNG、GIF、WebP 图片")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		fail(http.StatusInternalServerError, "读取图片失败，请重试")
		return
	}

	// 放在校验之后计数，选错文件不占次数
	uid := strconv.FormatUint(uint64(user.ID), 10)
	if !uploadShortLimiter.Allow(uid) || !uploadDayLimiter.Allow(uid) {
		fail(http.StatusTooManyRequests, "上传太频繁，请稍后再试")
		return
	}

	// 上传到 Imgur
	result, err := services.UploadImage(file, header)
	if err != nil {
		// 详细原因只写日志，不返回给用户（里面可能有上游地址、配置信息）
		log.Printf("[Upload] 图片上传失败 (user=%d): %v", user.ID, err)
		fail(http.StatusInternalServerError, "上传失败，请稍后重试")
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

	// 解析图片 ID 和扩展名
	ext := strings.ToLower(filepath.Ext(imageID))
	id := strings.TrimSuffix(imageID, filepath.Ext(imageID))
	if ext == "" {
		ext = ".jpg" // 默认扩展名
	}
	if !imgurIDPattern.MatchString(id) || !proxyImageExts[ext] {
		c.String(http.StatusNotFound, "图片不存在")
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

	// 构建 Imgur URL
	imgurURL := fmt.Sprintf("https://i.imgur.com/%s%s", id, ext)

	// 请求 Imgur 图片
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, imgurURL, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "请求创建失败")
		return
	}

	// 设置请求头，模拟浏览器
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")

	resp, err := imgurClient.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "获取图片失败")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.String(http.StatusNotFound, "图片不存在")
		return
	}

	// 只转发图片：上游返回别的类型（HTML 错误页等）时不能原样以本站的名义输出
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if !allowedImageTypes[contentType] {
		c.String(http.StatusBadGateway, "获取图片失败")
		return
	}
	c.Header("Content-Type", contentType)

	// 缓存控制：缓存 7 天
	c.Header("Cache-Control", "public, max-age=604800")
	c.Header("Vary", "Sec-Fetch-Site, Sec-Fetch-Mode")

	// 流式复制响应，限制最大字节数
	c.Status(http.StatusOK)
	io.Copy(c.Writer, io.LimitReader(resp.Body, maxProxyBytes))
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
