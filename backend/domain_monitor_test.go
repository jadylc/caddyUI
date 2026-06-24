package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDomainMonitorDomainsSplitsAndDedupes(t *testing.T) {
	got := domainMonitorDomains([]string{"example.com, api.example.com\nEXAMPLE.com", "blog.example.com"})
	want := []string{"example.com", "api.example.com", "blog.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("domainMonitorDomains() = %#v, want %#v", got, want)
	}
}

func TestParseDomainMonitorIntervalSupportsDays(t *testing.T) {
	got, err := parseDomainMonitorInterval("7d")
	if err != nil {
		t.Fatalf("parseDomainMonitorInterval() error = %v", err)
	}
	if got != 7*24*time.Hour {
		t.Fatalf("parseDomainMonitorInterval() = %v, want %v", got, 7*24*time.Hour)
	}
}

func TestDomainMonitorEndpointParsesURLAndPort(t *testing.T) {
	host, port := domainMonitorEndpoint("https://example.com:8443/path")
	if host != "example.com" || port != "8443" {
		t.Fatalf("domainMonitorEndpoint() = %q/%q, want example.com/8443", host, port)
	}
}

func TestDomainMonitorRegisteredDomainUsesHostedSuffix(t *testing.T) {
	cases := map[string]string{
		"wvw.dpdns.org.":   "wvw.dpdns.org",
		"a.b.dpdns.org":    "b.dpdns.org",
		"demo.us.kg":       "demo.us.kg",
		"a.demo.qzz.io":    "demo.qzz.io",
		"demo.xx.kg":       "demo.xx.kg",
		"edge.demo.qd.je":  "demo.qd.je",
		"oss.kdns.fr":      "oss.kdns.fr",
		"www.oss.kdns.fr":  "oss.kdns.fr",
		"api.example.com.": "example.com",
	}
	for host, want := range cases {
		got, _, err := domainMonitorRegisteredDomain(host)
		if err != nil {
			t.Fatalf("domainMonitorRegisteredDomain(%q) error = %v", host, err)
		}
		if got != want {
			t.Fatalf("domainMonitorRegisteredDomain(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestFetchDomainRegistrationExpirySkipsHostedSuffixRDAP(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	registeredDomain, expiresAt, err := fetchDomainRegistrationExpiry("wvw.dpdns.org.", "")
	if registeredDomain != "wvw.dpdns.org" {
		t.Fatalf("registeredDomain = %q, want wvw.dpdns.org", registeredDomain)
	}
	if !expiresAt.IsZero() {
		t.Fatalf("expiresAt = %v, want zero for hosted suffix", expiresAt)
	}
	if !errors.Is(err, errDomainRegistrationExpiryUnavailable) {
		t.Fatalf("error = %v, want errDomainRegistrationExpiryUnavailable", err)
	}
}

func TestFetchDomainRegistrationExpirySkipsUnmanagedHostedSuffixRDAP(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	registeredDomain, expiresAt, err := fetchDomainRegistrationExpiry("oss.kdns.fr", "")
	if registeredDomain != "oss.kdns.fr" {
		t.Fatalf("registeredDomain = %q, want oss.kdns.fr", registeredDomain)
	}
	if !expiresAt.IsZero() {
		t.Fatalf("expiresAt = %v, want zero for unmanaged hosted suffix", expiresAt)
	}
	if !errors.Is(err, errDomainRegistrationExpiryUnavailable) {
		t.Fatalf("error = %v, want errDomainRegistrationExpiryUnavailable", err)
	}
	if !strings.Contains(err.Error(), "没有可用的到期查询接口") {
		t.Fatalf("error = %q, want no API hint", err.Error())
	}
}

func TestStampDomainMonitorNormalizesLegacySSLIssuer(t *testing.T) {
	item := npmDomainMonitor{
		Name:          "issuer",
		DomainNames:   []string{"example.com"},
		CheckInterval: "24h",
		ThresholdDays: 30,
		Meta: map[string]any{
			"ssl_issuer": "R12",
			"results": []any{
				map[string]any{"domain": "a.example.com", "ssl_issuer": "YE2"},
				map[string]any{"domain": "b.example.com", "ssl_issuer": "WE1"},
			},
		},
	}

	stampNPMDomainMonitor(&item, "", true)

	if got := item.Meta["ssl_issuer"]; got != "Let's Encrypt（R12）" {
		t.Fatalf("ssl_issuer = %#v, want Let's Encrypt（R12）", got)
	}
	results := item.Meta["results"].([]any)
	first := results[0].(map[string]any)
	second := results[1].(map[string]any)
	if got := first["ssl_issuer"]; got != "Let's Encrypt（YE2）" {
		t.Fatalf("first ssl_issuer = %#v, want Let's Encrypt（YE2）", got)
	}
	if got := second["ssl_issuer"]; got != "Google Trust Services（WE1）" {
		t.Fatalf("second ssl_issuer = %#v, want Google Trust Services（WE1）", got)
	}
}

func TestFetchDomainRegistrationExpiryUsesDigitalPlatCredential(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	if err := saveCredentials([]Credential{{
		ID:                "dp",
		Name:              "DigitalPlat",
		Provider:          "digitalplat",
		DigitalPlatAPIKey: "dp_test_token",
	}}); err != nil {
		t.Fatalf("saveCredentials error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/domains" {
			t.Fatalf("path = %q, want /api/v1/domains", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer dp_test_token" {
			t.Fatalf("Authorization = %q, want Bearer dp_test_token", got)
		}
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, "Mozilla/5.0") {
			t.Fatalf("User-Agent = %q, want browser user agent", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": [
				{"name": "other.us.kg", "expiry_date": "2026-01-02"},
				{"domain": "wvw.dpdns.org", "status": "ok", "expires_at": "20270408"}
			],
			"meta": {}
		}`))
	}))
	defer server.Close()

	t.Setenv("DIGITALPLAT_API_BASE_URL", server.URL+"/api/v1")

	registeredDomain, expiresAt, err := fetchDomainRegistrationExpiry("wvw.dpdns.org.", "dp")
	if err != nil {
		t.Fatalf("fetchDomainRegistrationExpiry() error = %v", err)
	}
	if registeredDomain != "wvw.dpdns.org" {
		t.Fatalf("registeredDomain = %q, want wvw.dpdns.org", registeredDomain)
	}
	want := time.Date(2027, 4, 8, 23, 59, 59, 0, time.FixedZone("CST", 8*60*60))
	if !expiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", expiresAt, want)
	}
}

func TestParseDigitalPlatExpiryDateCompact(t *testing.T) {
	got, err := parseDigitalPlatExpiryDate("20270522")
	if err != nil {
		t.Fatalf("parseDigitalPlatExpiryDate() error = %v", err)
	}
	want := time.Date(2027, 5, 22, 23, 59, 59, 0, time.FixedZone("CST", 8*60*60))
	if !got.Equal(want) {
		t.Fatalf("time = %v, want %v", got, want)
	}
}

func TestRunDomainMonitorCheckMarksHostedExpiryUnavailable(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	item := npmDomainMonitor{
		Name:          "hosted",
		DomainNames:   []string{"wvw.dpdns.org"},
		CheckDomain:   true,
		CheckInterval: "24h",
		ThresholdDays: 30,
		Meta:          map[string]any{},
		Enabled:       true,
	}

	runDomainMonitorCheck(&item)

	if item.Meta["domain_name"] != "wvw.dpdns.org" {
		t.Fatalf("domain_name = %#v, want wvw.dpdns.org", item.Meta["domain_name"])
	}
	if item.Meta["domain_expiry_unavailable"] != true {
		t.Fatalf("domain_expiry_unavailable = %#v, want true", item.Meta["domain_expiry_unavailable"])
	}
	if item.Meta["status"] != "ok" {
		t.Fatalf("status = %#v, want ok", item.Meta["status"])
	}
}

func TestRunDomainMonitorCheckMarksDigitalPlatForbiddenAsUnavailable(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	if err := saveCredentials([]Credential{{
		ID:                "dp",
		Name:              "DigitalPlat",
		Provider:          "digitalplat",
		DigitalPlatAPIKey: "dp_forbidden",
	}}); err != nil {
		t.Fatalf("saveCredentials error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden domain", http.StatusForbidden)
	}))
	defer server.Close()
	t.Setenv("DIGITALPLAT_API_BASE_URL", server.URL+"/api/v1")

	item := npmDomainMonitor{
		Name:          "forbidden",
		DomainNames:   []string{"wvw.dpdns.org"},
		CheckDomain:   true,
		CredentialID:  "dp",
		CheckInterval: "24h",
		ThresholdDays: 30,
		Meta:          map[string]any{},
		Enabled:       true,
	}

	runDomainMonitorCheck(&item)

	if item.Meta["status"] != "ok" {
		t.Fatalf("status = %#v, want ok", item.Meta["status"])
	}
	if item.Meta["domain_expiry_unavailable"] != true {
		t.Fatalf("domain_expiry_unavailable = %#v, want true", item.Meta["domain_expiry_unavailable"])
	}
	reason, _ := item.Meta["domain_expiry_unavailable_reason"].(string)
	if !strings.Contains(reason, "403 Forbidden") || !strings.Contains(reason, "forbidden domain") {
		t.Fatalf("domain_expiry_unavailable_reason = %q, want 403 body", reason)
	}
	if lastError := item.Meta["last_error"]; lastError != "" {
		t.Fatalf("last_error = %#v, want empty", lastError)
	}
}

func TestFetchDigitalPlatDomainExpiryReportsChallengePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<!DOCTYPE html><title>Challenge Page</title>`))
	}))
	defer server.Close()
	t.Setenv("DIGITALPLAT_API_BASE_URL", server.URL+"/api/v1")

	_, err := fetchDigitalPlatDomainExpiry("wvw.dpdns.org", "dp_challenge")
	if !errors.Is(err, errDomainRegistrationExpiryUnavailable) {
		t.Fatalf("error = %v, want errDomainRegistrationExpiryUnavailable", err)
	}
	if !strings.Contains(err.Error(), "防护页拦截") {
		t.Fatalf("error = %q, want challenge page hint", err.Error())
	}
	if strings.Contains(err.Error(), "<!DOCTYPE") {
		t.Fatalf("error = %q, should not include HTML body", err.Error())
	}
}

func TestFetchDomainRegistrationExpiryUsesDNSHECredential(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	if err := saveCredentials([]Credential{{
		ID:             "dnshe",
		Name:           "DNSHE",
		Provider:       "dnshe",
		DNSHEAPIKey:    "cfsd_key",
		DNSHEAPISecret: "secret",
	}}); err != nil {
		t.Fatalf("saveCredentials error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("endpoint") != "subdomains" || r.URL.Query().Get("action") != "list" {
			t.Fatalf("query = %q, want subdomains/list", r.URL.RawQuery)
		}
		if got := r.Header.Get("X-API-Key"); got != "cfsd_key" {
			t.Fatalf("X-API-Key = %q, want cfsd_key", got)
		}
		if got := r.Header.Get("X-API-Secret"); got != "secret" {
			t.Fatalf("X-API-Secret = %q, want secret", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"subdomains": [
				{"id": 7, "subdomain": "wvw", "rootdomain": "cc.cd", "full_domain": "wvw.cc.cd", "status": "active", "expires_at": "2027-05-06 12:00:00"}
			]
		}`))
	}))
	defer server.Close()

	t.Setenv("DNSHE_API_BASE_URL", server.URL+"/index.php?m=domain_hub")

	registeredDomain, expiresAt, err := fetchDomainRegistrationExpiry("wvw.cc.cd.", "dnshe")
	if err != nil {
		t.Fatalf("fetchDomainRegistrationExpiry() error = %v", err)
	}
	if registeredDomain != "wvw.cc.cd" {
		t.Fatalf("registeredDomain = %q, want wvw.cc.cd", registeredDomain)
	}
	want := time.Date(2027, 5, 6, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if !expiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", expiresAt, want)
	}
}

func TestRenewDNSHEDomain(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	if err := saveCredentials([]Credential{{
		ID:             "dnshe",
		Name:           "DNSHE",
		Provider:       "dnshe",
		DNSHEAPIKey:    "cfsd_key",
		DNSHEAPISecret: "secret",
	}}); err != nil {
		t.Fatalf("saveCredentials error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("action") {
		case "list":
			_, _ = w.Write([]byte(`{"success": true, "subdomains": [{"id": 9, "subdomain": "wvw", "rootdomain": "cc.cd", "full_domain": "wvw.cc.cd", "expires_at": "2026-07-01"}]}`))
		case "renew":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("renew body decode error = %v", err)
			}
			if body["subdomain_id"].(float64) != 9 {
				t.Fatalf("subdomain_id = %#v, want 9", body["subdomain_id"])
			}
			_, _ = w.Write([]byte(`{"success": true, "message": "ok", "old_expires_at": "2026-07-01", "new_expires_at": "2027-07-01", "charged_amount": 0}`))
		default:
			t.Fatalf("unexpected action %q", r.URL.Query().Get("action"))
		}
	}))
	defer server.Close()

	t.Setenv("DNSHE_API_BASE_URL", server.URL+"/index.php?m=domain_hub")

	result, err := renewDNSHEDomain("wvw.cc.cd", "dnshe")
	if err != nil {
		t.Fatalf("renewDNSHEDomain() error = %v", err)
	}
	if result.Status != "ok" || result.NewExpiresOn != "2027-07-01" {
		t.Fatalf("result = %#v, want ok new expiry", result)
	}
}
