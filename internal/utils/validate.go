package utils

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

// 用户输入的长度上限（按字符数，URL 按字节）
const (
	MaxUsernameRunes = 20
	MaxAvatarRunes   = 16
	MaxBioRunes      = 200
	MaxTitleRunes    = 200
	MaxURLBytes      = 2048
	MaxPostRunes     = 100000
	MaxCommentRunes  = 10000
	MaxReasonRunes   = 200
)

// IsHTTPURL 是否为 http/https 的绝对地址。
// 用户填的外链会进 href 和 window.open，javascript: 之类的协议必须挡在外面
func IsHTTPURL(raw string) bool {
	if raw == "" || len(raw) > MaxURLBytes {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// NormalizeHTTPURL 校验用户提交的外链：去掉首尾空白，空串视为"没填"，否则必须是 http/https 地址
func NormalizeHTTPURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	return raw, IsHTTPURL(raw)
}

func isUsernameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '_' || r == '-' || r == '.'
}

// ValidUsername 用户名规则：1～20 个字符，只含各语言的字母、数字和 _ - .，中间可以有单个空格。
// 用户名会拼进回复引用的 Markdown 和邮件标题，不能放任意符号
func ValidUsername(s string) bool {
	n := utf8.RuneCountInString(s)
	if n < 1 || n > MaxUsernameRunes || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return false
	}
	prevSpace := false
	for _, r := range s {
		switch {
		case isUsernameRune(r):
			prevSpace = false
		case r == ' ' && !prevSpace:
			prevSpace = true
		default:
			return false
		}
	}
	return true
}

// SanitizeUsername 把邮箱前缀、Google 昵称整理成合法用户名：丢掉不允许的字符并截断，整理后为空时返回"竹友"
func SanitizeUsername(s string) string {
	var b strings.Builder
	n, pendingSpace := 0, false
	for _, r := range s {
		switch {
		case isUsernameRune(r):
			if pendingSpace && n > 0 && n < MaxUsernameRunes-1 {
				b.WriteByte(' ')
				n++
			}
			pendingSpace = false
			b.WriteRune(r)
			n++
		case unicode.IsSpace(r):
			pendingSpace = true
		}
		if n >= MaxUsernameRunes {
			break
		}
	}
	if b.Len() == 0 {
		return "竹友"
	}
	return b.String()
}

// ValidAvatar 头像只能是 emoji：最多 16 个码点（够放带肤色、ZWJ 的组合 emoji），
// 不能含字母、数字、空白和控制字符，否则可以拿"管理员"之类的文字当头像冒充身份
func ValidAvatar(s string) bool {
	n := utf8.RuneCountInString(s)
	if n < 1 || n > MaxAvatarRunes || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 0x80 || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// TruncateRunes 按字符数截断
func TruncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
