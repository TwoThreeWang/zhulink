package router

import (
	"zhulink/internal/handlers"
	"zhulink/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine) {
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

	// 公共路由 (Public Routes)
	r.GET("/", storyHandler.ListTop)           // 首页 - 热门文章
	r.GET("/new", storyHandler.ListNew)        // 最新文章
	r.GET("/search", storyHandler.Search)      // 搜索页面
	r.GET("/p/:pid", storyHandler.Detail)      // 文章详情页
	r.GET("/t/:name", storyHandler.ListByNode) // 节点下的文章列表
	r.GET("/nodes", nodeHandler.ListNodes)     // 所有节点列表
	r.GET("/u/:id", userHandler.Profile)       // 用户主页

	r.GET("/signup", authHandler.ShowRegister) // 注册页面
	r.POST("/signup", authHandler.Register)    // 提交注册
	r.GET("/login", authHandler.ShowLogin)     // 登录页面
	r.POST("/login", authHandler.Login)        // 提交登录
	r.GET("/logout", authHandler.Logout)       // 退出登录

	// 受保护路由 (Protected Routes)
	authorized := r.Group("/")
	authorized.Use(middleware.AuthRequired())
	{
		authorized.GET("/submit", storyHandler.ShowCreate)             // 发布文章页面
		authorized.POST("/submit", storyHandler.Create)                // 提交发布文章
		authorized.POST("/p/:pid/comment", storyHandler.CreateComment) // 发表评论
		authorized.POST("/vote/:type/:id", voteHandler.Vote)           // 点赞/投票
		authorized.POST("/vote/:type/:id/down", voteHandler.Downvote)  // 踩/反对
		authorized.POST("/bookmark/:id", bookmarkHandler.Toggle)       // 收藏/取消收藏
		authorized.GET("/p/:pid/edit", storyHandler.ShowEdit)          // 编辑文章页面
		authorized.POST("/p/:pid/edit", storyHandler.Update)           // 提交文章更新

		authorized.DELETE("/p/:pid", storyHandler.Delete)              // 删除文章
		authorized.DELETE("/comment/:cid", storyHandler.DeleteComment) // 删除评论

		authorized.POST("/notifications/:id/read", notificationHandler.Read)    // 标记单条通知为已读
		authorized.DELETE("/notifications/:id", notificationHandler.Delete)     // 删除单条通知
		authorized.POST("/notifications/read-all", notificationHandler.ReadAll) // 全部通知标记为已读
	}

	// 仪表盘路由 (Dashboard Routes)
	dashboard := r.Group("/dashboard")
	dashboard.Use(middleware.AuthRequired())
	{
		dashboard.GET("", userHandler.Dashboard)                  // 仪表盘概览
		dashboard.GET("/notifications", notificationHandler.List) // 我的通知列表
		dashboard.GET("/points", userHandler.PointLogs)           // 积分记录
		dashboard.GET("/settings", userHandler.ShowSettings)      // 用户设置页面
		dashboard.POST("/settings", userHandler.UpdateSettings)   // 提交用户设置更新
		dashboard.POST("/checkin", userHandler.CheckIn)           // 每日签到
	}

	// RSS 阅读器路由 (RSS Reader Routes)
	rss := r.Group("/rss")
	rss.Use(middleware.AuthRequired())
	{
		rss.GET("", rssHandler.Index)                          // RSS 阅读器主页
		rss.GET("/feeds", rssHandler.GetFeeds)                 // 获取订阅源列表
		rss.GET("/items", rssHandler.GetItems)                 // 获取文章项列表
		rss.GET("/read/:id", rssHandler.ReadItem)              // 获取单篇文章内容
		rss.POST("/subscribe", rssHandler.Subscribe)           // 订阅新的 RSS 源
		rss.DELETE("/unsubscribe/:id", rssHandler.Unsubscribe) // 取消订阅

		rss.POST("/subscription/update", rssHandler.UpdateSubscription) // 更新订阅设置
		rss.POST("/refresh/:id", rssHandler.RefreshFeed)                // 手动刷新订阅源
		rss.POST("/anchor/:id", rssHandler.UpdateAnchor)                // 更新阅读进度

		rss.GET("/transplant/:id", transplantHandler.ShowTransplantModal) // 显示推荐到社区的弹窗
		rss.POST("/transplant/:id", transplantHandler.Transplant)         // 提交推荐到社区
	}
}
