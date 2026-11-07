package services

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// solve 暴力找 nonce，测试用低难度
func solve(c PowChallenge) string {
	for i := 0; ; i++ {
		n := strconv.Itoa(i)
		if leadingZeroBits(c.Salt, n) >= c.Bits {
			return n
		}
	}
}

func newTestPow(now *time.Time) *PowService {
	s := NewPowService()
	s.now = func() time.Time { return *now }
	return s
}

func TestPowVerify(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := newTestPow(&now)
	c := s.Issue("register", 8)
	nonce := solve(c)

	now = now.Add(powMinFillTime - time.Second)
	if err := s.Verify(c.Token, nonce, "register", 8); err != ErrPowTooFast {
		t.Fatalf("提交过快应被拒绝, got %v", err)
	}

	now = now.Add(5 * time.Second)
	if err := s.Verify(c.Token, nonce, "reset", 8); err != ErrPowInvalid {
		t.Fatalf("跨表单使用应被拒绝, got %v", err)
	}
	if err := s.Verify(c.Token, nonce, "register", 12); err != ErrPowTooWeak {
		t.Fatalf("难度低于当前要求应被拒绝, got %v", err)
	}
	if err := s.Verify(c.Token, nonce, "register", 8); err != nil {
		t.Fatalf("正确的 nonce 应通过, got %v", err)
	}
	if err := s.Verify(c.Token, nonce, "register", 8); err != ErrPowReplayed {
		t.Fatalf("重放应被拒绝, got %v", err)
	}
}

func TestPowRejectsTampering(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := newTestPow(&now)
	c := s.Issue("register", 16)
	now = now.Add(5 * time.Second)

	// 把难度改低后签名对不上
	weak := strings.Replace(c.Token, ".16.", ".1.", 1)
	if err := s.Verify(weak, "0", "register", 1); err != ErrPowInvalid {
		t.Fatalf("篡改难度应被拒绝, got %v", err)
	}
	if err := s.Verify(c.Token, "", "register", 16); err != ErrPowMissing {
		t.Fatalf("缺少 nonce 应提示, got %v", err)
	}
	// 另一个进程（相当于重启后）签发的挑战不认
	other := NewPowService().Issue("register", 8)
	if err := s.Verify(other.Token, solve(other), "register", 8); err != ErrPowInvalid {
		t.Fatalf("其他密钥签发的挑战应被拒绝, got %v", err)
	}
}

func TestPowExpires(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := newTestPow(&now)
	c := s.Issue("reset", 8)
	now = now.Add(powTTL + time.Second)
	if err := s.Verify(c.Token, solve(c), "reset", 8); err != ErrPowExpired {
		t.Fatalf("过期挑战应被拒绝, got %v", err)
	}
}

// 固定向量（用 Python hashlib 独立算出），web/static/js/pow.js 的测试用同一组数据，保证前后端算法一致
func TestPowKnownVectors(t *testing.T) {
	cases := []struct {
		salt, nonce string
		want        int
	}{
		{"abc", "0", 1},        // sha256 = 5f36efce…
		{"zhulink", "212", 11}, // sha256 = 001553f9…，8 位难度下第一个解
		{"zhulink", "13058", 16},
	}
	for _, c := range cases {
		if got := leadingZeroBits(c.salt, c.nonce); got != c.want {
			t.Errorf("%s:%s got %d want %d", c.salt, c.nonce, got, c.want)
		}
	}
	if n := solve(PowChallenge{Salt: "zhulink", Bits: 8}); n != "212" {
		t.Errorf("8 位第一个解 got %s want 212", n)
	}
	if n := solve(PowChallenge{Salt: "zhulink", Bits: 16}); n != "13058" {
		t.Errorf("16 位第一个解 got %s want 13058", n)
	}
}
