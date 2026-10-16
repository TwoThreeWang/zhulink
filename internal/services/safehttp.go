package services

import (
	"errors"
	"io"
	"net"
	"syscall"
	"time"
)

// maxFetchBody 抓取用户提供的外部地址（RSS、文章正文）时，单个响应最多读取的字节数
const maxFetchBody = 5 << 20

var errBlockedAddress = errors.New("不允许访问内网或本机地址")

// safeDialer 只允许连接公网地址。检查发生在 DNS 解析之后的拨号阶段，
// 所以域名解析到内网（含 DNS 重绑定）也会被拦下。用于所有访问用户提供 URL 的 HTTP 客户端，防止 SSRF
var safeDialer = &net.Dialer{
	Timeout: 10 * time.Second,
	Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		if ip := net.ParseIP(host); ip == nil || !isPublicIP(ip) {
			return errBlockedAddress
		}
		return nil
	},
}

var cgnatRange = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

func isPublicIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 0 { // 0.0.0.0/8 在 Linux 上会连到本机
		return false
	}
	return !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsUnspecified() &&
		!ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() &&
		!ip.IsInterfaceLocalMulticast() && !ip.IsMulticast() && !cgnatRange.Contains(ip)
}

// limitedBody 限制响应体大小，防止超大响应把内存撑爆
type limitedBody struct {
	io.Reader
	io.Closer
}

func limitBody(body io.ReadCloser) io.ReadCloser {
	return limitedBody{io.LimitReader(body, maxFetchBody), body}
}
