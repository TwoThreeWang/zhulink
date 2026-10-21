package db

import (
	"log"
	"os"
	"time"
	"zhulink/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Fallback for local dev if not set
		dsn = "host=localhost user=postgres password=postgres dbname=zhulink port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 启用 pgvector 扩展
	if err := DB.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		log.Printf("Failed to enable pgvector extension: %v", err)
	}

	log.Println("Database connection established")

	// 配置连接池
	sqlDB, err := DB.DB()
	if err != nil {
		log.Printf("Failed to get underlying sql.DB: %v", err)
	} else {
		sqlDB.SetMaxOpenConns(100)          // 最大打开连接数
		sqlDB.SetMaxIdleConns(10)           // 最大空闲连接数
		sqlDB.SetConnMaxLifetime(time.Hour) // 连接最大存活时间
		log.Println("Database connection pool configured: MaxOpen=100, MaxIdle=10, MaxLifetime=1h")
	}

	// Auto Migrate
	err = DB.AutoMigrate(
		&models.User{},
		&models.Node{},
		&models.Post{},
		&models.Comment{},
		&models.Vote{},
		&models.PointLog{},
		&models.Bookmark{},
		&models.Notification{},
		// RSS 相关模型
		&models.Feed{},
		&models.UserSubscription{},
		&models.FeedItem{},
		&models.Report{},
	)
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("Database migration completed")

	migrateUniqueConstraints()

	// Seed initial nodes
	seedNodes()
}

func seedNodes() {
	// 检查是否已有节点数据
	var count int64
	DB.Model(&models.Node{}).Count(&count)
	if count > 0 {
		log.Println("Nodes already seeded, skipping")
		return
	}

	// 创建预设节点
	nodes := []models.Node{
		{Name: "技术", Description: "技术相关的讨论和分享"},
		{Name: "生活", Description: "生活日常、经验分享"},
		{Name: "展示", Description: "作品展示、项目分享"},
		{Name: "闲聊", Description: "随便聊聊"},
	}

	for _, node := range nodes {
		if err := DB.Create(&node).Error; err != nil {
			log.Printf("Failed to create node %s: %v", node.Name, err)
		}
	}
	log.Println("Initial nodes created successfully")
}

// migrateUniqueConstraints 补上 AutoMigrate 处理不了的唯一约束
func migrateUniqueConstraints() {
	stmts := []string{
		// 每人对同一帖子/评论只能投一票：先删掉并发请求留下的重复票（保留最早一张），再建部分唯一索引
		`DELETE FROM votes a USING votes b WHERE a.id > b.id AND a.user_id = b.user_id AND a.post_id = b.post_id`,
		`DELETE FROM votes a USING votes b WHERE a.id > b.id AND a.user_id = b.user_id AND a.comment_id = b.comment_id`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_vote_user_post ON votes (user_id, post_id) WHERE post_id IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_vote_user_comment ON votes (user_id, comment_id) WHERE comment_id IS NOT NULL`,
		// 登录、注册按 LOWER(email) 查用户；不建唯一索引，历史数据里可能有只差大小写的重复邮箱
		`CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users (LOWER(email))`,
		// RSS 条目改为"同一订阅源内 GUID 唯一"，删掉旧的全局唯一索引
		`DROP INDEX IF EXISTS idx_feed_items_guid`,
	}
	for _, stmt := range stmts {
		if err := DB.Exec(stmt).Error; err != nil {
			log.Fatalf("Failed to migrate unique constraints (%s): %v", stmt, err)
		}
	}
}
