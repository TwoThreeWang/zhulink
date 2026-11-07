package services

import (
	"context"
	"errors"
	"log"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// ErrEmailDomain 邮箱域名无法收信时给用户看的提示
var ErrEmailDomain = errors.New("该邮箱域名无法接收邮件，请检查是否拼写错误")

const emailDomainLookupTimeout = 3 * time.Second

// 查询结果缓存 1 小时：qq.com、gmail.com 这类常见域名不必每次都查 DNS
var emailDomainCache = expirable.NewLRU[string, bool](10000, nil, time.Hour)

// lookupMX 测试时替换成假的 DNS
var lookupMX = func(ctx context.Context, domain string) ([]*net.MX, error) {
	return net.DefaultResolver.LookupMX(ctx, domain)
}

// EmailDomainAcceptsMail 检查邮箱域名能不能收信：DNS 里必须有 MX 记录。
// 只在 DNS 明确答复"没有"时才返回 false；查询超时、网络出错时放行，不误伤正常用户。
// 只能拦住乱编或拼错的域名（abc@asdfgh.com、gmail.con），拦不住"域名真实、邮箱不存在"的地址。
func EmailDomainAcceptsMail(email string) bool {
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(email[at+1:])), ".")
	if !strings.Contains(domain, ".") {
		return false // localhost 之类的单段域名
	}
	if ok, cached := emailDomainCache.Get(domain); cached {
		return ok
	}

	ctx, cancel := context.WithTimeout(context.Background(), emailDomainLookupTimeout)
	defer cancel()
	records, err := lookupMX(ctx, domain)

	var ok bool
	var dnsErr *net.DNSError
	switch {
	case err == nil:
		// 只有一条 "." 的是 RFC 7505 的 Null MX，表示这个域名明确不收邮件（例如 example.com）
		ok = len(records) > 0 && !(len(records) == 1 && records[0].Host == ".")
	case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		ok = false // 域名不存在，或者存在但没有 MX 记录
	default:
		log.Printf("[EmailDomain] 查询 %s 的 MX 失败，放行: %v", domain, err)
		return true // 临时故障不缓存，下次再查
	}
	emailDomainCache.Add(domain, ok)
	return ok
}
