package services

import (
	"fmt"
	"math/rand"
	"time"
)

type CaptchaService struct {
	rnd *rand.Rand
}

func NewCaptchaService() *CaptchaService {
	return &CaptchaService{
		rnd: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateMathProblem returns a display string (e.g. "3 + 5") and the integer answer.
// Usage: Store answer in session, display question to user.
func (s *CaptchaService) GenerateMathProblem() (string, int) {
	a := s.rnd.Intn(10) // 0-9
	b := s.rnd.Intn(10) // 0-9
	op := s.rnd.Intn(2) // 0: +, 1: -

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
