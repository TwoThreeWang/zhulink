package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"zhulink/internal/db"
	"zhulink/internal/models"
)

// 每日评论汇总：每天固定时间，把过去 24 小时内收到、发送时仍未读的评论通知合并成一封邮件。
// 窗口按发送时刻切成首尾相接的 24 小时，每条通知只会落进一个窗口，所以不需要"已发送"标记。
// 代价：发送时刻服务正好停机，那一天的汇总会漏发（站内通知不受影响）；多实例部署会重复发送。
const (
	digestDefaultHour = 20 // 默认每晚 8 点，可用 DIGEST_HOUR 修改（0～23，按服务器时区）
	digestMaxItems    = 20 // 单封邮件最多列出的条数
	digestExcerptLen  = 80 // 评论摘要的字数
)

var digestTypes = []models.NotificationType{models.NotificationTypeCommentPost, models.NotificationTypeReplyComment}

// DigestItem 汇总邮件里的一条
type DigestItem struct {
	Actor   string
	Reason  template.HTML // 通知原文（站内生成、已转义），链接已改成绝对地址
	Excerpt string
	Time    string
}

func digestHour() int {
	if h, err := strconv.Atoi(os.Getenv("DIGEST_HOUR")); err == nil && h >= 0 && h <= 23 {
		return h
	}
	return digestDefaultHour
}

// nextDigestTime 返回 now 之后最近的一个发送时刻
func nextDigestTime(now time.Time, hour int) time.Time {
	t := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !t.After(now) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

// StartCommentDigest 启动每日评论汇总定时任务
func StartCommentDigest(ctx context.Context) {
	hour := digestHour()
	go func() {
		for {
			next := nextDigestTime(time.Now(), hour)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				// 窗口用计划时刻而不是实际醒来的时刻，保证相邻两天的窗口首尾相接
				SendCommentDigests(next.Add(-24*time.Hour), next)
			}
		}
	}()
	log.Printf("评论汇总定时任务已启动: 每天 %02d:00 发送", hour)
}

// SendCommentDigests 汇总 [from, to) 内产生、当前仍未读的评论通知，每人一封
func SendCommentDigests(from, to time.Time) {
	var notes []models.Notification
	err := db.DB.Select("notifications.*").Preload("Actor").
		Joins("JOIN users ON users.id = notifications.user_id").
		Where("notifications.is_read = ? AND notifications.type IN ? AND notifications.created_at >= ? AND notifications.created_at < ?", false, digestTypes, from, to).
		Where("users.email_digest = ? AND users.is_activated = ? AND users.status <> ?", true, true, 2).
		Order("notifications.user_id, notifications.created_at DESC").
		Find(&notes).Error
	if err != nil {
		log.Printf("[Digest] 查询通知失败: %v", err)
		return
	}
	if len(notes) == 0 {
		log.Printf("[Digest] %s～%s 没有需要汇总的通知", from.Format("01-02 15:04"), to.Format("01-02 15:04"))
		return
	}

	excerpts := loadCommentExcerpts(notes)
	byUser := map[uint][]models.Notification{}
	var userIDs []uint
	for _, n := range notes {
		if _, ok := byUser[n.UserID]; !ok {
			userIDs = append(userIDs, n.UserID)
		}
		byUser[n.UserID] = append(byUser[n.UserID], n)
	}

	var users []models.User
	db.DB.Where("id IN ?", userIDs).Find(&users)
	mail := NewMailService()
	siteURL := strings.TrimRight(os.Getenv("SITE_URL"), "/")
	for _, u := range users {
		list := byUser[u.ID]
		items := make([]DigestItem, 0, digestMaxItems)
		for i, n := range list {
			if i == digestMaxItems {
				break
			}
			items = append(items, DigestItem{
				Actor:   n.Actor.Username,
				Reason:  template.HTML(strings.ReplaceAll(n.Reason, `href="/`, `href="`+siteURL+`/`)),
				Excerpt: excerpts[commentIDFromReason(n.Reason)],
				Time:    n.CreatedAt.Format("01-02 15:04"),
			})
		}
		mail.SendCommentDigest(u.Email, u.Username, items, len(list), siteURL+"/dashboard/notifications", DigestUnsubscribeURL(u.ID))
	}
	log.Printf("[Digest] 已为 %d 位用户发送评论汇总，共 %d 条通知", len(users), len(notes))
}

// 通知原文里的链接形如 /p/<pid>#comment-<id>
var commentIDRe = regexp.MustCompile(`#comment-(\d+)`)

func commentIDFromReason(reason string) uint {
	m := commentIDRe.FindStringSubmatch(reason)
	if m == nil {
		return 0
	}
	id, _ := strconv.ParseUint(m[1], 10, 64)
	return uint(id)
}

// loadCommentExcerpts 一次查出所有相关评论，返回评论 ID → 摘要
func loadCommentExcerpts(notes []models.Notification) map[uint]string {
	var ids []uint
	for _, n := range notes {
		if id := commentIDFromReason(n.Reason); id != 0 {
			ids = append(ids, id)
		}
	}
	out := map[uint]string{}
	if len(ids) == 0 {
		return out
	}
	var comments []models.Comment
	db.DB.Select("id", "content").Where("id IN ?", ids).Find(&comments)
	for _, c := range comments {
		out[c.ID] = commentExcerpt(c.Content)
	}
	return out
}

// commentExcerpt 去掉开头的"↳ 回复 #楼层"引用行，压成一行并截断
func commentExcerpt(content string) string {
	if strings.HasPrefix(content, "↳") {
		if i := strings.Index(content, "\n"); i >= 0 {
			content = content[i+1:]
		}
	}
	content = strings.Join(strings.Fields(content), " ")
	if utf8.RuneCountInString(content) > digestExcerptLen {
		content = string([]rune(content)[:digestExcerptLen]) + "…"
	}
	return content
}

// ==================== 退订 ====================

var (
	unsubscribeKey     []byte
	unsubscribeKeyOnce sync.Once
)

// 退订链接要长期有效，所以用固定的 SESSION_AUTH_KEY 派生签名；开发环境没配置时用临时密钥
func digestUnsubscribeKey() []byte {
	unsubscribeKeyOnce.Do(func() {
		if k := os.Getenv("SESSION_AUTH_KEY"); k != "" {
			unsubscribeKey = []byte(k)
			return
		}
		unsubscribeKey = make([]byte, 32)
		rand.Read(unsubscribeKey)
	})
	return unsubscribeKey
}

func signUnsubscribe(userID uint) string {
	mac := hmac.New(sha256.New, digestUnsubscribeKey())
	mac.Write([]byte("digest-unsubscribe:" + strconv.FormatUint(uint64(userID), 10)))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

// DigestUnsubscribeURL 邮件里的一键退订链接
func DigestUnsubscribeURL(userID uint) string {
	return strings.TrimRight(os.Getenv("SITE_URL"), "/") + "/unsubscribe?u=" + strconv.FormatUint(uint64(userID), 10) + "&s=" + signUnsubscribe(userID)
}

// VerifyUnsubscribe 校验退订链接，返回用户 ID
func VerifyUnsubscribe(uid, sig string) (uint, bool) {
	id, err := strconv.ParseUint(uid, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), hmac.Equal([]byte(sig), []byte(signUnsubscribe(uint(id))))
}
