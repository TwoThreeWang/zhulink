package main

import (
	"fmt"
	"html/template"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/handlers"
	"zhulink/internal/middleware"
	"zhulink/internal/services"

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

	// Initialize Database
	db.Init()

	// 初始化异步排名服务
	services.GetRankingService()

	// Initialize Gin
	r := gin.Default()

	// Setup Sessions
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		secret = "secret_key_change_me"
	}
	store := cookie.NewStore([]byte(secret))
	r.Use(sessions.Sessions("zhulink_session", store))

	// Load Templates using Multitemplate to avoid collision and allow handler names
	r.HTMLRender = loadTemplates("./web/templates")

	// Static Assets
	r.Static("/static", "./web/static")

	// Middleware
	r.Use(middleware.LoadUser())

	// Handlers
	authHandler := handlers.NewAuthHandler()
	storyHandler := handlers.NewStoryHandler()
	voteHandler := handlers.NewVoteHandler()
	userHandler := handlers.NewUserHandler()
	nodeHandler := handlers.NewNodeHandler()
	bookmarkHandler := handlers.NewBookmarkHandler()
	notificationHandler := handlers.NewNotificationHandler()
	rssHandler := handlers.NewRSSHandler()
	transplantHandler := handlers.NewTransplantHandler()

	// Public Routes
	r.GET("/", storyHandler.ListTop)
	r.GET("/new", storyHandler.ListNew)
	r.GET("/search", storyHandler.Search)
	r.GET("/p/:pid", storyHandler.Detail)
	r.GET("/t/:name", storyHandler.ListByNode)
	r.GET("/nodes", nodeHandler.ListNodes)
	r.GET("/u/:id", userHandler.Profile) // 用户主页

	r.GET("/signup", authHandler.ShowRegister)
	r.POST("/signup", authHandler.Register)
	r.GET("/login", authHandler.ShowLogin)
	r.POST("/login", authHandler.Login)
	r.GET("/logout", authHandler.Logout)

	// Protected Routes
	authorized := r.Group("/")
	authorized.Use(middleware.AuthRequired())
	{
		authorized.GET("/submit", storyHandler.ShowCreate)
		authorized.POST("/submit", storyHandler.Create)
		authorized.POST("/p/:pid/comment", storyHandler.CreateComment)
		authorized.POST("/vote/:type/:id", voteHandler.Vote)
		authorized.POST("/vote/:type/:id/down", voteHandler.Downvote)
		authorized.POST("/bookmark/:id", bookmarkHandler.Toggle)
		authorized.GET("/p/:pid/edit", storyHandler.ShowEdit)
		authorized.POST("/p/:pid/edit", storyHandler.Update)

		authorized.DELETE("/p/:pid", storyHandler.Delete)
		authorized.DELETE("/comment/:cid", storyHandler.DeleteComment)

		authorized.POST("/notifications/:id/read", notificationHandler.Read)
		authorized.DELETE("/notifications/:id", notificationHandler.Delete)
		authorized.POST("/notifications/read-all", notificationHandler.ReadAll)
	}

	// Dashboard Routes
	dashboard := r.Group("/dashboard")
	dashboard.Use(middleware.AuthRequired())
	{
		dashboard.GET("", userHandler.Dashboard)
		dashboard.GET("/notifications", notificationHandler.List)
		dashboard.GET("/points", userHandler.PointLogs)
		dashboard.GET("/settings", userHandler.ShowSettings)
		dashboard.POST("/settings", userHandler.UpdateSettings)
		dashboard.POST("/checkin", userHandler.CheckIn)
	}

	// RSS 阅读器路由（苗圃）
	rss := r.Group("/rss")
	rss.Use(middleware.AuthRequired())
	{
		rss.GET("", rssHandler.Index)
		rss.GET("/feeds", rssHandler.GetFeeds)
		rss.GET("/items", rssHandler.GetItems)
		rss.GET("/read/:id", rssHandler.ReadItem)
		rss.POST("/subscribe", rssHandler.Subscribe)
		rss.DELETE("/unsubscribe/:id", rssHandler.Unsubscribe)

		rss.POST("/subscription/update", rssHandler.UpdateSubscription)
		rss.POST("/refresh/:id", rssHandler.RefreshFeed)
		rss.POST("/anchor/:id", rssHandler.UpdateAnchor)

		// 推荐到社区 (Transplant)
		rss.GET("/transplant/:id", transplantHandler.ShowTransplantModal)
		rss.POST("/transplant/:id", transplantHandler.Transplant)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("ZhuLink server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
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

	// Generate our templates map from our views directories
	// We want to map: "story/list.html" -> [base, list, components...]

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
		"timeAgo": func(t interface{}) string {
			// 尝试将输入转换为 time.Time
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
			// 简单的 HTML 标签移除
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
			// 移除多余的空白
			text := string(result)
			// 替换 &nbsp; 等 HTML 实体
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

	// Scan views
	// auth/login.html, auth/register.html
	// story/list.html, story/detail.html, story/create.html
	// error.html

	// Manual registration to ensure keys match handler expectation
	// Auth
	r.AddFromFilesFuncs("auth/login.html", funcMap, assemble(templatesDir+"/views/auth/login.html")...)
	r.AddFromFilesFuncs("auth/register.html", funcMap, assemble(templatesDir+"/views/auth/register.html")...)

	// Story
	// list.html handles both "/" and "/new", handler calls "story/list.html"
	r.AddFromFilesFuncs("story/list.html", funcMap, assemble(templatesDir+"/views/story/list.html")...)
	r.AddFromFilesFuncs("story/detail.html", funcMap, assemble(templatesDir+"/views/story/detail.html")...)
	r.AddFromFilesFuncs("story/create.html", funcMap, assemble(templatesDir+"/views/story/create.html")...)
	r.AddFromFilesFuncs("story/edit.html", funcMap, assemble(templatesDir+"/views/story/edit.html")...)

	// User
	r.AddFromFilesFuncs("user/public.html", funcMap, assemble(templatesDir+"/views/user/public.html")...)

	// Dashboard
	r.AddFromFilesFuncs("dashboard/overview.html", funcMap, assemble(templatesDir+"/views/dashboard/overview.html")...)
	r.AddFromFilesFuncs("notification/list.html", funcMap, assemble(templatesDir+"/views/notification/list.html")...)
	r.AddFromFilesFuncs("dashboard/points.html", funcMap, assemble(templatesDir+"/views/dashboard/points.html")...)
	r.AddFromFilesFuncs("dashboard/settings.html", funcMap, assemble(templatesDir+"/views/dashboard/settings.html")...)

	// Node
	r.AddFromFilesFuncs("node/list.html", funcMap, assemble(templatesDir+"/views/node/list.html")...)

	// Search
	r.AddFromFilesFuncs("search.html", funcMap, assemble(templatesDir+"/views/search.html")...)

	// Error
	r.AddFromFilesFuncs("error.html", funcMap, assemble(templatesDir+"/views/error.html")...)

	// RSS 阅读器（苗圃）
	r.AddFromFilesFuncs("rss/index.html", funcMap, assemble(templatesDir+"/views/rss/index.html")...)
	r.AddFromFilesFuncs("rss/feed_list.html", funcMap, templatesDir+"/views/rss/feed_list.html")
	r.AddFromFilesFuncs("rss/item_list.html", funcMap, templatesDir+"/views/rss/item_list.html")
	r.AddFromFilesFuncs("rss/reader_content.html", funcMap, templatesDir+"/views/rss/reader_content.html")
	r.AddFromFilesFuncs("rss/transplant_modal.html", funcMap, templatesDir+"/views/rss/transplant_modal.html")
	r.AddFromFilesFuncs("rss/transplant_result.html", funcMap, templatesDir+"/views/rss/transplant_result.html")

	return r
}
