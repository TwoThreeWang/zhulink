package utils

import (
	crand "crypto/rand"
	"math/big"
	"math/rand"
	"time"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index (0-63)
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// RandStringBytesMaskImpr generates a random string of length n using letters only (a-zA-Z)
// This implementation uses bit masking for improved performance
func RandStringBytesMaskImpr(n int) string {
	b := make([]byte, n)
	// 使用 rand.Int63() 生成63位随机数,足够 letterIdxMax 个字母的索引
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return string(b)
}

// GenerateRandomCode generates a random numeric string of length n
// 用于邮件验证码，必须用 crypto/rand，math/rand 的输出可被预测
func GenerateRandomCode(n int) string {
	b := make([]byte, n)
	for i := range b {
		d, err := crand.Int(crand.Reader, big.NewInt(10))
		if err != nil {
			panic(err) // 系统随机源不可用时不能退化成可预测的验证码
		}
		b[i] = byte(d.Int64()) + '0'
	}
	return string(b)
}
