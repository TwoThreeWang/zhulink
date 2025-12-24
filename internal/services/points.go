package services

import (
	"math/rand"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/models"

	"gorm.io/gorm"
)

// 积分动作常量
const (
	ActionPostCreate       = "发布帖子"
	ActionPostLiked        = "帖子获赞"
	ActionPostBookmarked   = "帖子被收藏"
	ActionPostUnbookmark   = "帖子取消收藏"
	ActionPostDownvoted    = "帖子被踩"
	ActionPostDeleted      = "删除帖子"
	ActionCommentCreate    = "发布评论"
	ActionCommentLiked     = "评论获赞"
	ActionCommentDownvoted = "评论被踩"
	ActionCommentDeleted   = "删除评论"
	ActionDownvoteOther    = "踩了别人"
	ActionCheckIn          = "每日签到"
	ActionCheckInBonus     = "签到额外奖励"
)

// 积分值常量
const (
	PointsPostCreate       = 1
	PointsPostLiked        = 1
	PointsPostBookmarked   = 3
	PointsPostUnbookmark   = -3
	PointsPostDownvoted    = -3
	PointsPostDeleted      = -10
	PointsCommentCreate    = 1
	PointsCommentLiked     = 1
	PointsCommentDownvoted = -3
	PointsCommentDeleted   = -3
	PointsDownvoteOther    = -1
	PointsCheckIn          = 1
)

// 每日限制
const (
	DailyPostLimit    = 3 // 每天前3篇帖子有积分
	DailyCommentLimit = 3 // 每天前3条评论有积分
)

// AddPoints 使用事务添加积分并记录明细
// 传入用户ID、积分变动值（正数增加，负数扣除）、动作描述
func AddPoints(userID uint, amount int, action string) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 创建积分明细记录
		log := models.PointLog{
			UserID: userID,
			Amount: amount,
			Action: action,
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}

		// 2. 更新用户积分余额
		if err := tx.Model(&models.User{}).
			Where("id = ?", userID).
			UpdateColumn("points", gorm.Expr("points + ?", amount)).
			Error; err != nil {
			return err
		}

		return nil
	})
}

// AddPointsAsync 异步添加积分（在 goroutine 中调用）
func AddPointsAsync(userID uint, amount int, action string) {
	go func() {
		_ = AddPoints(userID, amount, action)
	}()
}

// getTodayRange 获取今日的开始和结束时间
func getTodayRange() (time.Time, time.Time) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)
	return startOfDay, endOfDay
}

// countTodayPointLogs 统计今日指定动作的积分记录数
func countTodayPointLogs(userID uint, action string) int64 {
	startOfDay, endOfDay := getTodayRange()
	var count int64
	db.DB.Model(&models.PointLog{}).
		Where("user_id = ? AND action = ? AND created_at >= ? AND created_at < ?", userID, action, startOfDay, endOfDay).
		Count(&count)
	return count
}

// CanEarnPostPoints 检查用户今日是否还能通过发帖获取积分
func CanEarnPostPoints(userID uint) bool {
	count := countTodayPointLogs(userID, ActionPostCreate)
	return count < DailyPostLimit
}

// CanEarnCommentPoints 检查用户今日是否还能通过评论获取积分
func CanEarnCommentPoints(userID uint) bool {
	count := countTodayPointLogs(userID, ActionCommentCreate)
	return count < DailyCommentLimit
}

// HasCheckedInToday 检查用户今日是否已签到
func HasCheckedInToday(userID uint) bool {
	count := countTodayPointLogs(userID, ActionCheckIn)
	return count > 0
}

// CheckIn 每日签到，返回获得的积分和是否为首次签到
func CheckIn(userID uint) (points int, bonus int, alreadyCheckedIn bool, err error) {
	// 检查是否已签到
	if HasCheckedInToday(userID) {
		return 0, 0, true, nil
	}

	// 基础积分
	points = PointsCheckIn

	// 使用事务处理签到逻辑
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		// 记录基础签到积分
		baseLog := models.PointLog{
			UserID: userID,
			Amount: points,
			Action: ActionCheckIn,
		}
		if err := tx.Create(&baseLog).Error; err != nil {
			return err
		}

		// 中等概率（约30%）获得额外奖励
		if rand.Intn(100) < 30 {
			bonus = rand.Intn(3) + 1 // 1-3 额外竹笋
			bonusLog := models.PointLog{
				UserID: userID,
				Amount: bonus,
				Action: ActionCheckInBonus,
			}
			if err := tx.Create(&bonusLog).Error; err != nil {
				return err
			}
		}

		// 更新用户积分
		totalPoints := points + bonus
		if err := tx.Model(&models.User{}).
			Where("id = ?", userID).
			UpdateColumn("points", gorm.Expr("points + ?", totalPoints)).
			Error; err != nil {
			return err
		}

		return nil
	})

	return points, bonus, false, err
}
