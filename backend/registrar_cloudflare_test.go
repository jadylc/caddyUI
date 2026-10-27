package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestCloudflareRegistrarAccountIDAutoDiscover 未填 account_id 时应通过 accounts 列表自动获取。
func TestCloudflareRegistrarAccountIDAutoDiscover(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cf-token" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "result": [{"id": "acc-1", "name": "test"}]}`))
	}))
	defer server.Close()

	oldBase := cloudflareAPIBaseURL
	cloudflareAPIBaseURL = server.URL
	defer func() { cloudflareAPIBaseURL = oldBase }()

	cred := Credential{Provider: "cloudflare", CFToken: "cf-token"}
	accountID, err := cloudflareRegistrarAccountID(cred)
	if err != nil {
		t.Fatalf("cloudflareRegistrarAccountID error = %v", err)
	}
	if accountID != "acc-1" {
		t.Fatalf("accountID = %q, want acc-1", accountID)
	}
}

// TestCloudflareRegistrarExpiry 到期查询应匹配域名并返回到期时间。
func TestCloudflareRegistrarExpiry(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	expiresOn := time.Date(2027, 5, 6, 12, 0, 0, 0, time.UTC).Unix()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/accounts":
			_, _ = w.Write([]byte(`{"success": true, "result": [{"id": "acc-1"}]}`))
		case strings.Contains(r.URL.Path, "/registrar/domains"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{"success": true, "result": [{"name": "example.com", "expires_on": %d}]}`, expiresOn)))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	oldBase := cloudflareAPIBaseURL
	cloudflareAPIBaseURL = server.URL
	defer func() { cloudflareAPIBaseURL = oldBase }()

	cred := Credential{Provider: "cloudflare", CFToken: "cf-token"}
	expiresAt, err := fetchCloudflareRegistrarExpiry("example.com", cred)
	if err != nil {
		t.Fatalf("fetchCloudflareRegistrarExpiry error = %v", err)
	}
	want := time.Unix(expiresOn, 0)
	if !expiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", expiresAt, want)
	}
}

// TestCloudflareRegistrarRenew 续期应返回新的到期时间。
func TestCloudflareRegistrarRenew(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	newExpiresOn := time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC).Unix()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/accounts":
			_, _ = w.Write([]byte(`{"success": true, "result": [{"id": "acc-1"}]}`))
		case strings.Contains(r.URL.Path, "/registrar/domains/example.com/renew"):
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{"success": true, "result": {"name": "example.com", "expires_on": %d}}`, newExpiresOn)))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	oldBase := cloudflareAPIBaseURL
	cloudflareAPIBaseURL = server.URL
	defer func() { cloudflareAPIBaseURL = oldBase }()

	cred := Credential{Provider: "cloudflare", CFToken: "cf-token"}
	result, err := renewCloudflareRegistrarDomain("example.com", cred)
	if err != nil {
		t.Fatalf("renewCloudflareRegistrarDomain error = %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	wantNewExpires := time.Unix(newExpiresOn, 0).Format(time.RFC3339)
	if result.NewExpiresOn != wantNewExpires {
		t.Fatalf("NewExpiresOn = %q, want %q", result.NewExpiresOn, wantNewExpires)
	}
}

// TestPorkbunDomainExpiry Porkbun 到期查询。
func TestPorkbunDomainExpiry(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/domain/list") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "pb-key" {
			t.Fatalf("X-API-Key = %q", r.Header.Get("X-API-Key"))
		}
		if r.Header.Get("X-Secret-API-Key") != "pb-secret" {
			t.Fatalf("X-Secret-API-Key = %q", r.Header.Get("X-Secret-API-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status": "SUCCESS", "response": [{"domain": "example.com", "expires": "2027-05-06"}]}`))
	}))
	defer server.Close()

	oldBase := porkbunAPIBaseURL
	porkbunAPIBaseURL = server.URL + "/api/json/v3"
	defer func() { porkbunAPIBaseURL = oldBase }()

	cred := Credential{Provider: "porkbun", Secrets: map[string]string{
		"porkbun_api_key":        "pb-key",
		"porkbun_api_secret_key": "pb-secret",
	}}
	expiresAt, err := fetchPorkbunDomainExpiry("example.com", cred)
	if err != nil {
		t.Fatalf("fetchPorkbunDomainExpiry error = %v", err)
	}
	want := time.Date(2027, 5, 6, 0, 0, 0, 0, time.UTC)
	if !expiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", expiresAt, want)
	}
}

// TestPorkbunDomainRenew Porkbun 续期。
func TestPorkbunDomainRenew(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/domain/renew/example.com") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status": "SUCCESS", "response": {"domain": "example.com", "expires": "2028-01-01"}}`))
	}))
	defer server.Close()

	oldBase := porkbunAPIBaseURL
	porkbunAPIBaseURL = server.URL + "/api/json/v3"
	defer func() { porkbunAPIBaseURL = oldBase }()

	cred := Credential{Provider: "porkbun", Secrets: map[string]string{
		"porkbun_api_key":        "pb-key",
		"porkbun_api_secret_key": "pb-secret",
	}}
	result, err := renewPorkbunDomain("example.com", cred)
	if err != nil {
		t.Fatalf("renewPorkbunDomain error = %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	wantNewExpires := time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	if result.NewExpiresOn != wantNewExpires {
		t.Fatalf("NewExpiresOn = %q, want %q", result.NewExpiresOn, wantNewExpires)
	}
}

// TestParsePorkbunExpiryDate 覆盖多种到期时间格式。
func TestParsePorkbunExpiryDate(t *testing.T) {
	cases := []struct {
		input string
		want  time.Time
	}{
		{"2027-05-06", time.Date(2027, 5, 6, 0, 0, 0, 0, time.UTC)},
		{"2027-05-06T12:00:00Z", time.Date(2027, 5, 6, 12, 0, 0, 0, time.UTC)},
	}
	for _, item := range cases {
		got, err := parsePorkbunExpiryDate(item.input)
		if err != nil {
			t.Fatalf("parsePorkbunExpiryDate(%q) error = %v", item.input, err)
		}
		if !got.Equal(item.want) {
			t.Fatalf("parsePorkbunExpiryDate(%q) = %v, want %v", item.input, got, item.want)
		}
	}
}

// TestRegistrarSpecsHaveImplementations 注册表里标记 Registrar 的 provider 必须有 fetchExpiry。
func TestRegistrarSpecsHaveImplementations(t *testing.T) {
	for _, spec := range providerSpecs {
		if !spec.Registrar {
			continue
		}
		if spec.fetchExpiry == nil {
			t.Fatalf("provider %s 标记为 registrar 但缺少 fetchExpiry", spec.ID)
		}
	}
}
