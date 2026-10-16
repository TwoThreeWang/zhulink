package utils

import (
	"math"
	"time"
)

type RankConfig struct {
	Gravity        float64 // 时间重力 (1.5)
	WeightCollect  float64 // 3.0
	WeightComment  float64 // 2.0
	WeightUpvote   float64 // 1.0
	WeightDownvote float64 // 1.5
	WeightView     float64 // 0.05 (浏览量权重)
	ScaleFactor    float64 // 放大系数 (100000)
	TimeBase       float64 // 时间基数 (24)
}

var DefaultConfig = RankConfig{
	Gravity:        1.5,
	WeightCollect:  3.0,
	WeightComment:  2.0,
	WeightUpvote:   1.0,
	WeightDownvote: 1.5,
	WeightView:     0.05,     // 浏览量权重,1000 次浏览 ≈ 50 分互动值
	ScaleFactor:    100000.0, // 分数要存成整数，放大后取整才不会大量并列
	TimeBase:       24.0,     // 时间基数,防止新帖分数虚高
}

func CalculateScore(t time.Time, up, down, collect, view, comment int) float64 {
	hours := time.Since(t).Hours()

	// 1. 计算加权互动值 (Weighted Sum)
	// 浏览量以极小权重参与计算,避免数量级过大扭曲结果
	weightedSum := (float64(up) * DefaultConfig.WeightUpvote) +
		(float64(comment) * DefaultConfig.WeightComment) +
		(float64(collect) * DefaultConfig.WeightCollect) +
		(float64(view) * DefaultConfig.WeightView) -
		(float64(down) * DefaultConfig.WeightDownvote)

	// 2. 基础修正
	if weightedSum < 0 {
		weightedSum = 0 // 踩多于赞时按 0 分处理
	}

	// 3. 放大系数；互动值线性计入（不取 log：log 会把互动差距压平，热门页退化成最新页）
	numerator := weightedSum * DefaultConfig.ScaleFactor

	// 4. 时间衰减 (分母)
	// 使用时间基数防止新帖子分数虚高
	decay := math.Pow(hours+DefaultConfig.TimeBase, DefaultConfig.Gravity)

	return numerator / decay
}
