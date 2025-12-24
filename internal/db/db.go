package db

import (
	"log"
	"os"
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

	log.Println("Database connection established")

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
	)
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("Database migration completed")

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
