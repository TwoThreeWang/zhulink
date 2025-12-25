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
	WeightView     float64 // 0.005 (浏览量权重极小)
	ScaleFactor    float64 // 放大系数 (1000)
	TimeBase       float64 // 时间基数 (24)
}

var DefaultConfig = RankConfig{
	Gravity:        1.5,
	WeightCollect:  3.0,
	WeightComment:  2.0,
	WeightUpvote:   1.0,
	WeightDownvote: 1.5,
	WeightView:     0.005,  // 浏览量权重极小,1000 次浏览 ≈ 5 分互动值
	ScaleFactor:    1000.0, // 让分数落在 0-999 区间
	TimeBase:       24.0,   // 时间基数,防止新帖分数虚高
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
		weightedSum = 0 // 防止负数无法取对数
	}

	// 3. 对数平滑 (Log Smoothing)
	// log10(sum + 1) -> 确保 sum=0 时结果为 0
	logScore := math.Log10(weightedSum + 1)

	// 4. 放大系数 (0.x -> 几十)
	numerator := logScore * DefaultConfig.ScaleFactor

	// 5. 时间衰减 (分母)
	// 使用时间基数防止新帖子分数虚高
	decay := math.Pow(hours+DefaultConfig.TimeBase, DefaultConfig.Gravity)

	return numerator / decay
}
