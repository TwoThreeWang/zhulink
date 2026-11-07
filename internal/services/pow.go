package services

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// 工作量证明（PoW）人机校验：服务端签发挑战，浏览器找一个 nonce，
// 使 SHA-256(salt + ":" + nonce) 的前 bits 位全为 0。
// 正常用户在后台算 1 秒左右，批量刷接口的脚本每次都要付出同样的算力。
const (
	// 平均要试 2^bits 次。浏览器里实测约 400 万次/秒（M 系列芯片），手机慢 3～5 倍
	PowBitsNormal = 20 // 电脑约 0.25 秒，手机约 1 秒
	PowBitsHigh   = 23 // 熔断时使用，约为平时的 8 倍

	powTTL         = 30 * time.Minute // 挑战有效期：覆盖用户慢慢填表的时间
	powMinFillTime = 2 * time.Second  // 从打开页面到提交不足这个时间，视为脚本
	powMaxBits     = 32
	powUsedCap     = 100000
)

var (
	ErrPowMissing  = errors.New("缺少人机校验，请确认浏览器已启用 JavaScript 后刷新页面")
	ErrPowInvalid  = errors.New("人机校验失败，请刷新页面后重试")
	ErrPowExpired  = errors.New("页面已过期，请重新提交")
	ErrPowTooFast  = errors.New("提交太快了，请稍后再试")
	ErrPowTooWeak  = errors.New("当前访问量较大，需要重新校验，请再次提交")
	ErrPowReplayed = errors.New("该校验已使用过，请刷新页面后重试")
)

// PowChallenge 渲染到表单里的挑战
type PowChallenge struct {
	Token string // 完整挑战，原样随表单提交
	Salt  string // 参与哈希计算的部分
	Bits  int
}

type PowService struct {
	key  []byte
	mu   sync.Mutex
	used *expirable.LRU[string, struct{}] // 用过的 salt，保留到挑战过期，防重放
	now  func() time.Time
}

// NewPowService 每次启动随机生成签名密钥：重启后旧挑战全部失效，不会出现
// "内存里的已用记录清空了，旧挑战又能用"的情况。代价是多实例部署时要换成共享密钥和共享存储。
func NewPowService() *PowService {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(fmt.Sprintf("pow: generate key: %v", err))
	}
	return &PowService{
		key:  key,
		used: expirable.NewLRU[string, struct{}](powUsedCap, nil, powTTL),
		now:  time.Now,
	}
}

var (
	powService     *PowService
	powServiceOnce sync.Once
)

func GetPowService() *PowService {
	powServiceOnce.Do(func() { powService = NewPowService() })
	return powService
}

// Issue 为指定表单签发挑战。scope 区分表单，注册页的挑战不能拿去找回密码用。
func (s *PowService) Issue(scope string, difficulty int) PowChallenge {
	if difficulty < 1 || difficulty > powMaxBits {
		difficulty = PowBitsNormal
	}
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		panic(fmt.Sprintf("pow: generate salt: %v", err))
	}
	salt := hex.EncodeToString(raw)
	payload := fmt.Sprintf("%s.%d.%d.%s", scope, s.now().Unix(), difficulty, salt)
	return PowChallenge{Token: payload + "." + s.sign(payload), Salt: salt, Bits: difficulty}
}

// Verify 校验挑战和 nonce；minBits 是当前要求的最低难度，用来拒绝熔断前囤下的低难度挑战。
// 通过后挑战即作废，同一个挑战只能用一次。
func (s *PowService) Verify(token, nonce, scope string, minBits int) error {
	if token == "" || nonce == "" {
		return ErrPowMissing
	}
	parts := strings.Split(token, ".")
	if len(parts) != 5 {
		return ErrPowInvalid
	}
	payload := strings.Join(parts[:4], ".")
	if !hmac.Equal([]byte(parts[4]), []byte(s.sign(payload))) || parts[0] != scope {
		return ErrPowInvalid
	}
	issued, err1 := strconv.ParseInt(parts[1], 10, 64)
	difficulty, err2 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || len(nonce) > 20 {
		return ErrPowInvalid
	}

	age := s.now().Sub(time.Unix(issued, 0))
	switch {
	case age > powTTL:
		return ErrPowExpired
	case age < powMinFillTime:
		return ErrPowTooFast
	case difficulty < minBits:
		return ErrPowTooWeak
	case leadingZeroBits(parts[3], nonce) < difficulty:
		return ErrPowInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used.Contains(parts[3]) {
		return ErrPowReplayed
	}
	s.used.Add(parts[3], struct{}{})
	return nil
}

func (s *PowService) sign(payload string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

// leadingZeroBits 计算 SHA-256(salt:nonce) 开头连续 0 的位数（最多看前 32 位）
func leadingZeroBits(salt, nonce string) int {
	sum := sha256.Sum256([]byte(salt + ":" + nonce))
	return bits.LeadingZeros32(binary.BigEndian.Uint32(sum[:4]))
}
