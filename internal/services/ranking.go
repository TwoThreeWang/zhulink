package services

import (
	"log"
	"sync"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/models"
	"zhulink/internal/utils"
)

// RankingService 提供异步计算和更新帖子 Score 的服务
type RankingService struct {
	queue   chan uint // 待更新的帖子 ID 队列
	pending map[uint]bool
	mu      sync.Mutex
}

var (
	rankingService *RankingService
	once           sync.Once
)

// GetRankingService 获取单例排名服务
func GetRankingService() *RankingService {
	once.Do(func() {
		rankingService = &RankingService{
			queue:   make(chan uint, 1000), // 缓冲队列，防止阻塞
			pending: make(map[uint]bool),
		}
		// 启动后台 worker
		go rankingService.worker()
	})
	return rankingService
}

// ScheduleUpdate 将帖子加入更新队列（异步）
// 使用去重机制避免短时间内重复计算同一帖子
func (s *RankingService) ScheduleUpdate(postID uint) {
	s.mu.Lock()
	if s.pending[postID] {
		// 已在队列中，跳过
		s.mu.Unlock()
		return
	}
	s.pending[postID] = true
	s.mu.Unlock()

	// 非阻塞发送到队列
	select {
	case s.queue <- postID:
		// 成功加入队列
	default:
		// 队列满了，移除 pending 标记
		s.mu.Lock()
		delete(s.pending, postID)
		s.mu.Unlock()
		log.Printf("排名更新队列已满，跳过帖子 %d", postID)
	}
}

// worker 后台处理队列中的更新请求
func (s *RankingService) worker() {
	// 批量处理：收集一批请求后统一处理
	batch := make([]uint, 0, 50)
	ticker := time.NewTicker(500 * time.Millisecond) // 每 500ms 处理一批
	defer ticker.Stop()

	for {
		select {
		case postID := <-s.queue:
			batch = append(batch, postID)
			// 如果达到批量大小，立即处理
			if len(batch) >= 50 {
				s.processBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			// 定时处理剩余的
			if len(batch) > 0 {
				s.processBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// processBatch 批量处理帖子 Score 更新
func (s *RankingService) processBatch(postIDs []uint) {
	for _, postID := range postIDs {
		s.updatePostScore(postID)

		// 清除 pending 状态
		s.mu.Lock()
		delete(s.pending, postID)
		s.mu.Unlock()
	}
}

// updatePostScore 计算并更新单个帖子的 Score
func (s *RankingService) updatePostScore(postID uint) {
	// 获取帖子信息
	var post models.Post
	if err := db.DB.First(&post, postID).Error; err != nil {
		log.Printf("更新 Score 失败：帖子 %d 不存在", postID)
		return
	}

	// 统计点赞数
	var upvotes int64
	db.DB.Model(&models.Vote{}).Where("post_id = ? AND value = 1", postID).Count(&upvotes)

	// 统计点踩数
	var downvotes int64
	db.DB.Model(&models.Vote{}).Where("post_id = ? AND value = -1", postID).Count(&downvotes)

	// 统计收藏数
	var collects int64
	db.DB.Model(&models.Bookmark{}).Where("post_id = ?", postID).Count(&collects)

	// 统计评论数
	var comments int64
	db.DB.Model(&models.Comment{}).Where("post_id = ?", postID).Count(&comments)

	// 计算新 Score
	newScore := utils.CalculateScore(
		post.CreatedAt,
		int(upvotes),
		int(downvotes),
		int(collects),
		post.Views,
		int(comments),
	)

	// 更新数据库（Score 现在是 0-100 区间的整数）
	scoreInt := int(newScore)

	if err := db.DB.Model(&post).UpdateColumn("score", scoreInt).Error; err != nil {
		log.Printf("更新帖子 %d Score 失败: %v", postID, err)
	}
}

// UpdatePostScoreSync 同步更新帖子 Score（用于需要立即生效的场景）
func UpdatePostScoreSync(postID uint) {
	GetRankingService().updatePostScore(postID)
}
