package services

import (
	"context"
	"errors"
	"net"
	"testing"
)

func stubMX(t *testing.T, answers map[string]func() ([]*net.MX, error)) *int {
	t.Helper()
	calls := 0
	orig := lookupMX
	lookupMX = func(_ context.Context, domain string) ([]*net.MX, error) {
		calls++
		if f, ok := answers[domain]; ok {
			return f()
		}
		return nil, &net.DNSError{Err: "no such host", Name: domain, IsNotFound: true}
	}
	emailDomainCache.Purge()
	t.Cleanup(func() { lookupMX = orig; emailDomainCache.Purge() })
	return &calls
}

func TestEmailDomainAcceptsMail(t *testing.T) {
	stubMX(t, map[string]func() ([]*net.MX, error){
		"qq.com":      func() ([]*net.MX, error) { return []*net.MX{{Host: "mx1.qq.com.", Pref: 10}}, nil },
		"example.com": func() ([]*net.MX, error) { return []*net.MX{{Host: ".", Pref: 0}}, nil }, // Null MX
		"slow.com":    func() ([]*net.MX, error) { return nil, &net.DNSError{Err: "i/o timeout", IsTimeout: true} },
		"broken.com":  func() ([]*net.MX, error) { return nil, errors.New("network unreachable") },
	})
	cases := map[string]bool{
		"a@qq.com":         true,
		"A@QQ.COM":         true,  // 大小写不影响
		"a@asdfgh-xyz.com": false, // 域名不存在
		"a@gmail.con":      false, // 拼错的顶级域
		"a@example.com":    false, // 明确声明不收信
		"a@localhost":      false, // 单段域名
		"no-at-sign":       false,
		"a@":               false,
		"a@slow.com":       true, // DNS 超时放行
		"a@broken.com":     true, // 网络出错放行
	}
	for email, want := range cases {
		if got := EmailDomainAcceptsMail(email); got != want {
			t.Errorf("%s: got %v want %v", email, got, want)
		}
	}
}

func TestEmailDomainCache(t *testing.T) {
	calls := stubMX(t, map[string]func() ([]*net.MX, error){
		"qq.com":   func() ([]*net.MX, error) { return []*net.MX{{Host: "mx1.qq.com."}}, nil },
		"slow.com": func() ([]*net.MX, error) { return nil, &net.DNSError{Err: "i/o timeout", IsTimeout: true} },
	})
	EmailDomainAcceptsMail("a@qq.com")
	EmailDomainAcceptsMail("b@qq.com")
	EmailDomainAcceptsMail("c@nope-xyz.com")
	EmailDomainAcceptsMail("d@nope-xyz.com")
	if *calls != 2 {
		t.Fatalf("有结论的域名应缓存，got %d 次查询", *calls)
	}
	EmailDomainAcceptsMail("a@slow.com")
	EmailDomainAcceptsMail("b@slow.com")
	if *calls != 4 {
		t.Fatalf("临时故障不应缓存，got %d 次查询", *calls)
	}
}
