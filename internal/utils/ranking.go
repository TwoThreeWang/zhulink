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
	ScaleFactor    float64 // 放大系数 (100)
}

var DefaultConfig = RankConfig{
	Gravity:        1.5,
	WeightCollect:  3.0,
	WeightComment:  2.0,
	WeightUpvote:   1.0,
	WeightDownvote: 1.5,
	ScaleFactor:    100.0, // 让分数落在 0-100 区间，像"温度"
}

func CalculateScore(t time.Time, up, down, collect, view, comment int) float64 {
	hours := time.Since(t).Hours()

	// 1. 计算加权互动值 (Weighted Sum)
	// 注意：这里去掉了 View，因为 View 数量级太大，不适合放在 Log 里的权重计算，
	// 或者 View 的权重需要给得极小 (e.g., 0.01)
	weightedSum := (float64(up) * DefaultConfig.WeightUpvote) +
		(float64(comment) * DefaultConfig.WeightComment) +
		(float64(collect) * DefaultConfig.WeightCollect) -
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
	decay := math.Pow(hours+2, DefaultConfig.Gravity)

	return numerator / decay
}
