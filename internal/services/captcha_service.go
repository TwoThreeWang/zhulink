package services

import (
	"fmt"
	"math/rand"
)

// CaptchaService 用包级 rand 函数：它们并发安全，共享的 *rand.Rand 不是
type CaptchaService struct{}

func NewCaptchaService() *CaptchaService {
	return &CaptchaService{}
}

// GenerateMathProblem returns a display string (e.g. "3 + 5") and the integer answer.
// Usage: Store answer in session, display question to user.
func (s *CaptchaService) GenerateMathProblem() (string, int) {
	a := rand.Intn(10) // 0-9
	b := rand.Intn(10) // 0-9
	op := rand.Intn(2) // 0: +, 1: -

	if op == 0 {
		return fmt.Sprintf("%d + %d", a, b), a + b
	} else {
		// Ensure positive result for simplicity if subtraction
		if a < b {
			a, b = b, a
		}
		return fmt.Sprintf("%d - %d", a, b), a - b
	}
}
