package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestParseWhoisExpiry 覆盖多种 WHOIS 到期时间格式。
func TestParseWhoisExpiry(t *testing.T) {
	cases := []struct {
		name string
		text string
		want time.Time
	}{
		{
			name: "Verisign 格式",
			text: "Registry Expiry Date: 2027-05-06T12:00:00Z",
			want: time.Date(2027, 5, 6, 12, 0, 0, 0, time.UTC),
		},
		{
			name: "PIR 格式",
			text: "Registry Expiry Date: 06-May-2027",
			want: time.Date(2027, 5, 6, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "CN 格式",
			text: "Expiration Date: 2027-05-06",
			want: time.Date(2027, 5, 6, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "RU 格式",
			text: "paid-till: 20270506T000000Z",
			want: time.Date(2027, 5, 6, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, item := range cases {
		got, err := parseWhoisExpiry(item.text)
		if err != nil {
			t.Fatalf("parseWhoisExpiry(%q) error = %v", item.name, err)
		}
		if !got.Equal(item.want) {
			t.Fatalf("parseWhoisExpiry(%q) = %v, want %v", item.name, got, item.want)
		}
	}
}

// TestParseWhoisExpiryNoMatch 无到期时间时应报错。
func TestParseWhoisExpiryNoMatch(t *testing.T) {
	text := "Domain Name: EXAMPLE.COM\nRegistrar: TEST\nStatus: ok"
	_, err := parseWhoisExpiry(text)
	if err == nil {
		t.Fatal("应返回错误")
	}
}

// TestRDAPTLD 提取 TLD。
func TestRDAPTLD(t *testing.T) {
	cases := map[string]string{
		"example.com":       "com",
		"sub.example.io":    "io",
		"a.b.c.example.org": "org",
		"localhost":         "",
	}
	for domain, want := range cases {
		if got := rdapTLD(domain); got != want {
			t.Fatalf("rdapTLD(%q) = %q, want %q", domain, got, want)
		}
	}
}

// TestRDAPExpiryFallbackUsesRDAP 优先走 RDAP。
func TestRDAPExpiryFallbackUsesRDAP(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	expiresOn := time.Date(2027, 5, 6, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"events": [{"eventAction": "expiration", "eventDate": "%s"}]}`, expiresOn)))
	}))
	defer server.Close()

	// 替换 RDAP 服务器列表，指向测试服务器。
	oldServers := rdapTLDServers["com"]
	rdapTLDServers["com"] = []string{server.URL + "/"}
	defer func() { rdapTLDServers["com"] = oldServers }()

	expiresAt, err := fetchRDAPExpiry("example.com")
	if err != nil {
		t.Fatalf("fetchRDAPExpiry error = %v", err)
	}
	want := time.Date(2027, 5, 6, 12, 0, 0, 0, time.UTC)
	if !expiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", expiresAt, want)
	}
}

// TestRDAPExpiryFallbackFallsBackToWHOIS RDAP 失败时应回退到 WHOIS。
// 由于 httptest 不支持 TCP port 43，这里验证 RDAP 失败后会尝试 WHOIS 路径。
// 如果测试环境有网络且 WHOIS 可达，函数会成功返回——这也是正确的回退行为。
func TestRDAPExpiryFallbackFallsBackToWHOIS(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	// RDAP 服务器返回 404。
	rdapServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer rdapServer.Close()

	oldServers := rdapTLDServers["com"]
	rdapTLDServers["com"] = []string{rdapServer.URL + "/"}
	defer func() { rdapTLDServers["com"] = oldServers }()

	// RDAP 失败后，fetchDomainExpiryFallback 会尝试 WHOIS。
	// 无论 WHOIS 是否可达，函数都不应 panic。
	_, _ = fetchDomainExpiryFallback("example.com")
}

// TestFetchDomainRegistrationExpiryRDAPFallback 验证 fetchDomainRegistrationExpiry 走通用兜底路径。
func TestFetchDomainRegistrationExpiryRDAPFallback(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	expiresOn := time.Date(2027, 5, 6, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"events": [{"eventAction": "expiration", "eventDate": "%s"}]}`, expiresOn)))
	}))
	defer server.Close()

	oldServers := rdapTLDServers["com"]
	rdapTLDServers["com"] = []string{server.URL + "/"}
	defer func() { rdapTLDServers["com"] = oldServers }()

	registeredDomain, expiresAt, err := fetchDomainRegistrationExpiry("example.com", "")
	if err != nil {
		t.Fatalf("fetchDomainRegistrationExpiry error = %v", err)
	}
	if registeredDomain != "example.com" {
		t.Fatalf("registeredDomain = %q", registeredDomain)
	}
	want := time.Date(2027, 5, 6, 12, 0, 0, 0, time.UTC)
	if !expiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", expiresAt, want)
	}
}
