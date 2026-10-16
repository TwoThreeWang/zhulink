package services

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "172.17.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "0.1.2.3", "::1", "fe80::1", "::ffff:127.0.0.1"} {
		if isPublicIP(net.ParseIP(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "96.44.165.177", "2606:4700::1111"} {
		if !isPublicIP(net.ParseIP(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
}

func TestSafeDialerBlocksLocalhost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	client := &http.Client{Transport: &http.Transport{DialContext: safeDialer.DialContext}}
	_, err := client.Get(srv.URL)
	if !errors.Is(err, errBlockedAddress) {
		t.Fatalf("want errBlockedAddress, got %v", err)
	}
}
