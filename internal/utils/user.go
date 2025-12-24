package utils

import (
	"math/rand"
	"time"
)

// GetUserLevel 根据积分数量返回用户等级
func GetUserLevel(points int) (name string, icon string) {
	switch {
	case points >= 1000:
		return "成林", "🎋"
	case points >= 201:
		return "翠竹", "🎍"
	case points >= 51:
		return "新竹", "🌿"
	case points >= 11:
		return "破土", "🌾"
	default:
		return "萌芽", "🌱"
	}
}

// GetDaysSinceJoined 计算入林天数
func GetDaysSinceJoined(createdAt time.Time) int {
	return int(time.Since(createdAt).Hours() / 24)
}

// GetRandomEmoji 返回一个随机 emoji 用于默认头像
func GetRandomEmoji() string {
	rand.Seed(time.Now().UnixNano())
	emojis := []string{"🌱", "🌿", "🍃", "🌾", "🎋", "🎍", "🌲", "🌳", "🐼", "🦊", "🐨", "🐸"}
	return emojis[rand.Intn(len(emojis))]
}

// GetCommonEmojis 返回常用 emoji 列表供用户选择
func GetCommonEmojis() []string {
	return []string{
		"🌱", "🌿", "🍃", "🌾", "🎋", "🎍", "🌲", "🌳",
		"🐼", "🦊", "🐨", "🐸", "🦉", "🐯", "🐱", "🐶",
		"😀", "😃", "😄", "😁", "😊", "😎", "🤓", "🧐",
		"👨‍💻", "👩‍💻", "👨‍🎨", "👩‍🎨", "🧑‍🚀", "👨‍🔬", "👩‍🔬", "🧙",
		"⭐", "✨", "🔥", "💡", "🚀", "🎯", "💎", "🏆",
	}
}
