package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/handlers"
	"zhulink/internal/middleware"
	"zhulink/internal/router"
	"zhulink/internal/services"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, finding env vars from system")
	}

	// 显式设置模式
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	} else if os.Getenv("PORT") != "" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize Database
	db.Init()

	// 初始化 Google OAuth
	handlers.InitGoogleOAuth()

	// 初始化异步排名服务
	services.GetRankingService()

	// 启动 RSS 定时任务
	rssFetcher := services.GetRSSFetcher()
	rssFetcher.StartScheduledFetch()   // 每 30 分钟拉取新文章
	rssFetcher.StartScheduledCleanup() // 每天凌晨 2 点清除过期文章
	log.Println("RSS 定时任务已启动: 拉取间隔 30 分钟, 保留最近 30 天文章")

	// 启动文章分数定时更新任务
	services.GetRankingService().StartScheduledScoreUpdate() // 每天凌晨 3 点更新
	log.Println("文章分数定时任务已启动: 每天凌晨 3 点更新")

	// Initialize Gin
	r := gin.Default()

	// Setup Sessions
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		secret = "secret_key_change_me"
	}
	store := cookie.NewStore([]byte(secret))

	// 配置cookie选项以支持iOS等移动设备
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,                          // 7天
		HttpOnly: true,                               // 防止XSS攻击
		Secure:   os.Getenv("GIN_MODE") == "release", // 生产环境使用HTTPS
		SameSite: http.SameSiteLaxMode,               // Lax模式兼容性最好
	})

	r.Use(sessions.Sessions("zhulink_session", store))

	// Gzip 压缩中间件（对文本类型响应启用压缩）
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	// Load Templates using Multitemplate to avoid collision and allow handler names
	r.HTMLRender = loadTemplates("./web/templates")

	// Static Assets with Cache Headers (1 year for versioned files)
	r.Use(staticCacheMiddleware())
	r.Static("/static", "./web/static")

	// Middleware
	r.Use(middleware.LoadUser())

	// Register Routes
	router.RegisterRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "32919"
	}

	// 配置 http.Server
	srv := &http.Server{
		Addr:           ":" + port,
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   20 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// 在 goroutine 中启动服务器
	go func() {
		log.Printf("ZhuLink server starting on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 实现平滑停机
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// 给予 5 秒的缓冲时间来处理现有请求
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}

func loadTemplates(templatesDir string) multitemplate.Renderer {
	r := multitemplate.NewRenderer()

	layouts, err := filepath.Glob(templatesDir + "/layouts/*.html")
	if err != nil {
		panic(err)
	}

	includes, err := filepath.Glob(templatesDir + "/includes/*.html")
	if err != nil {
		panic(err)
	}

	components, err := filepath.Glob(templatesDir + "/components/*.html")
	if err != nil {
		panic(err)
	}

	// Helper to assemble files
	assemble := func(view string) []string {
		files := make([]string, 0)
		files = append(files, layouts...)
		files = append(files, includes...)
		files = append(files, components...)
		files = append(files, view)
		return files
	}

	// FuncMap
	funcMap := template.FuncMap{
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"iterate": func(count int) []int {
			var items []int
			for i := 1; i <= count; i++ {
				items = append(items, i)
			}
			return items
		},
		"lt": func(a, b int) bool {
			return a < b
		},
		"timeAgo": func(t interface{}) string {
			var timeVal time.Time
			switch v := t.(type) {
			case time.Time:
				timeVal = v
			default:
				return ""
			}

			duration := time.Since(timeVal)
			seconds := int(duration.Seconds())

			if seconds < 60 {
				return fmt.Sprintf("%d秒前", seconds)
			} else if seconds < 3600 {
				return fmt.Sprintf("%d分钟前", seconds/60)
			} else if seconds < 86400 {
				return fmt.Sprintf("%d小时前", seconds/3600)
			} else if seconds < 2592000 {
				return fmt.Sprintf("%d天前", seconds/86400)
			} else if seconds < 31536000 {
				return fmt.Sprintf("%d个月前", seconds/2592000)
			}
			return fmt.Sprintf("%d年前", seconds/31536000)
		},
		"eq": func(a, b interface{}) bool {
			return a == b
		},
		"gt": func(a, b int) bool {
			return a > b
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"stripHTML": func(s string) string {
			var result []rune
			inTag := false
			for _, r := range s {
				if r == '<' {
					inTag = true
				} else if r == '>' {
					inTag = false
				} else if !inTag {
					result = append(result, r)
				}
			}
			text := string(result)
			text = strings.ReplaceAll(text, "&nbsp;", " ")
			text = strings.ReplaceAll(text, "&amp;", "&")
			text = strings.ReplaceAll(text, "&lt;", "<")
			text = strings.ReplaceAll(text, "&gt;", ">")
			text = strings.ReplaceAll(text, "&quot;", "\"")
			text = strings.ReplaceAll(text, "&#39;", "'")
			return strings.TrimSpace(text)
		},
		"urlquery": func(s string) string {
			return url.QueryEscape(s)
		},
	}

	// Manual registration to ensure keys match handler expectation
	r.AddFromFilesFuncs("auth/login.html", funcMap, assemble(templatesDir+"/views/auth/login.html")...)
	r.AddFromFilesFuncs("auth/register.html", funcMap, assemble(templatesDir+"/views/auth/register.html")...)
	r.AddFromFilesFuncs("auth/activate.html", funcMap, assemble(templatesDir+"/views/auth/activate.html")...)
	r.AddFromFilesFuncs("auth/forgot_password.html", funcMap, assemble(templatesDir+"/views/auth/forgot_password.html")...)
	r.AddFromFilesFuncs("auth/reset_password.html", funcMap, assemble(templatesDir+"/views/auth/reset_password.html")...)
	r.AddFromFilesFuncs("story/list.html", funcMap, assemble(templatesDir+"/views/story/list.html")...)
	r.AddFromFilesFuncs("story/detail.html", funcMap, assemble(templatesDir+"/views/story/detail.html")...)
	r.AddFromFilesFuncs("story/create.html", funcMap, assemble(templatesDir+"/views/story/create.html")...)
	r.AddFromFilesFuncs("story/edit.html", funcMap, assemble(templatesDir+"/views/story/edit.html")...)
	r.AddFromFilesFuncs("user/public.html", funcMap, assemble(templatesDir+"/views/user/public.html")...)
	r.AddFromFilesFuncs("dashboard/overview.html", funcMap, assemble(templatesDir+"/views/dashboard/overview.html")...)
	r.AddFromFilesFuncs("notification/list.html", funcMap, assemble(templatesDir+"/views/notification/list.html")...)
	r.AddFromFilesFuncs("dashboard/points.html", funcMap, assemble(templatesDir+"/views/dashboard/points.html")...)
	r.AddFromFilesFuncs("dashboard/settings.html", funcMap, assemble(templatesDir+"/views/dashboard/settings.html")...)
	r.AddFromFilesFuncs("node/list.html", funcMap, assemble(templatesDir+"/views/node/list.html")...)
	r.AddFromFilesFuncs("search.html", funcMap, assemble(templatesDir+"/views/search.html")...)
	r.AddFromFilesFuncs("error.html", funcMap, assemble(templatesDir+"/views/error.html")...)
	r.AddFromFilesFuncs("rss/index.html", funcMap, assemble(templatesDir+"/views/rss/index.html")...)
	r.AddFromFilesFuncs("rss/feed_list.html", funcMap, templatesDir+"/views/rss/feed_list.html")
	r.AddFromFilesFuncs("rss/item_list.html", funcMap, templatesDir+"/views/rss/item_list.html")
	r.AddFromFilesFuncs("rss/item_list_items.html", funcMap, templatesDir+"/views/rss/item_list_items.html")
	r.AddFromFilesFuncs("rss/reader_content.html", funcMap, templatesDir+"/views/rss/reader_content.html")
	r.AddFromFilesFuncs("rss/transplant_modal.html", funcMap, templatesDir+"/views/rss/transplant_modal.html")
	r.AddFromFilesFuncs("rss/transplant_result.html", funcMap, templatesDir+"/views/rss/transplant_result.html")
	r.AddFromFilesFuncs("rss/popular.html", funcMap, assemble(templatesDir+"/views/rss/popular.html")...)
	r.AddFromFilesFuncs("admin/reports.html", funcMap, assemble(templatesDir+"/views/admin/reports.html")...)
	r.AddFromFilesFuncs("admin/users.html", funcMap, assemble(templatesDir+"/views/admin/users.html")...)

	return r
}

// staticCacheMiddleware 为静态资源添加缓存头
func staticCacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 只处理静态资源路径
		if strings.HasPrefix(path, "/static/") {
			// 检查是否带版本号（如 style.css?v=4）
			if c.Query("v") != "" || strings.Contains(path, ".") {
				// 带版本号的资源：缓存 1 年
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				// 无版本号：缓存 1 天
				c.Header("Cache-Control", "public, max-age=86400")
			}
		}

		c.Next()
	}
}
