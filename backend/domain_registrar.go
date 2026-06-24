package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type dnsheSubdomainsResponse struct {
	Success    bool             `json:"success"`
	Message    string           `json:"message,omitempty"`
	Error      string           `json:"error,omitempty"`
	ErrorCode  string           `json:"error_code,omitempty"`
	Subdomains []dnsheSubdomain `json:"subdomains"`
}

type dnsheSubdomain struct {
	ID           int    `json:"id"`
	Subdomain    string `json:"subdomain"`
	RootDomain   string `json:"rootdomain"`
	FullDomain   string `json:"full_domain"`
	Status       string `json:"status"`
	ExpiresAt    string `json:"expires_at"`
	NeverExpires any    `json:"never_expires"`
}

type dnsheRenewResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message,omitempty"`
	Error         string `json:"error,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
	OldExpiresAt  string `json:"old_expires_at,omitempty"`
	NewExpiresAt  string `json:"new_expires_at,omitempty"`
	ChargedAmount any    `json:"charged_amount,omitempty"`
}

type domainRenewResult struct {
	Domain        string `json:"domain"`
	Provider      string `json:"provider"`
	Status        string `json:"status"`
	OldExpiresOn  string `json:"old_expires_on,omitempty"`
	NewExpiresOn  string `json:"new_expires_on,omitempty"`
	ChargedAmount any    `json:"charged_amount,omitempty"`
	Error         string `json:"error,omitempty"`
}

func domainMonitorRegistrarHostedZones() []string {
	out := append([]string{}, digitalPlatHostedZones...)
	out = append(out, dnsheHostedZones...)
	out = append(out, unmanagedHostedZones...)
	for _, suffix := range strings.FieldsFunc(os.Getenv("DOMAIN_MONITOR_HOSTED_ZONES"), func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == ';'
	}) {
		suffix = strings.Trim(strings.ToLower(strings.TrimSpace(suffix)), ".")
		if suffix != "" {
			out = append(out, suffix)
		}
	}
	return out
}

func domainMonitorRegistrarProvider(domain string) string {
	domain = normalizedDomainMonitorHost(domain)
	for _, suffix := range digitalPlatHostedZones {
		suffix = strings.Trim(strings.ToLower(strings.TrimSpace(suffix)), ".")
		if domain == suffix || strings.HasSuffix(domain, "."+suffix) {
			return "digitalplat"
		}
	}
	for _, suffix := range dnsheHostedZones {
		suffix = strings.Trim(strings.ToLower(strings.TrimSpace(suffix)), ".")
		if domain == suffix || strings.HasSuffix(domain, "."+suffix) {
			return "dnshe"
		}
	}
	return ""
}

func normalizeDomainMonitorRegistrarProvider(provider string) string {
	raw := strings.TrimSpace(provider)
	key := strings.ToLower(strings.NewReplacer(" ", "", "-", "", "_", "").Replace(raw))
	switch key {
	case "":
		return ""
	case "aliyun", "alidns", "alibabacloud", "aliyundns", "阿里云", "阿里云dns":
		return "alidns"
	case "digitalplat", "digitalplatform":
		return "digitalplat"
	case "dnshe":
		return "dnshe"
	case "cloudflare":
		return "cloudflare"
	case "dnspod":
		return "dnspod"
	default:
		return raw
	}
}

func inferDomainMonitorRegistrarProvider(domains []string) string {
	for _, domain := range domainMonitorDomains(domains) {
		host, _ := domainMonitorEndpoint(domain)
		if host == "" {
			continue
		}
		if registeredDomain, _, err := domainMonitorRegisteredDomain(host); err == nil {
			if provider := domainMonitorRegistrarProvider(registeredDomain); provider != "" {
				return provider
			}
		}
		if provider := domainMonitorRegistrarProvider(host); provider != "" {
			return provider
		}
	}
	return ""
}

func effectiveDomainMonitorRegistrarProvider(item npmDomainMonitor) string {
	if provider := normalizeDomainMonitorRegistrarProvider(item.RegistrarProvider); provider != "" {
		return provider
	}
	return inferDomainMonitorRegistrarProvider(item.DomainNames)
}

func registrarCredentialValid(provider string, cred Credential) bool {
	switch provider {
	case "digitalplat":
		return cred.Provider == "digitalplat" && strings.TrimSpace(cred.DigitalPlatAPIKey) != ""
	case "dnshe":
		return cred.Provider == "dnshe" && strings.TrimSpace(cred.DNSHEAPIKey) != "" && strings.TrimSpace(cred.DNSHEAPISecret) != ""
	default:
		return false
	}
}

func domainRegistrarCredentialForDomainMonitor(provider string, credentialID string) (Credential, bool, error) {
	if provider == "" {
		return Credential{}, false, nil
	}
	creds, err := loadCredentials()
	if err != nil {
		return Credential{}, false, err
	}
	credentialID = strings.TrimSpace(credentialID)
	if credentialID != "" {
		cred, ok := findCredential(credentialID, creds)
		if !ok {
			return Credential{}, false, errors.New("选择的域名商凭据不存在")
		}
		if !registrarCredentialValid(provider, cred) {
			return Credential{}, false, fmt.Errorf("所选凭据不是 %s 凭据或缺少密钥", domainRegistrarProviderName(provider))
		}
		return cred, true, nil
	}

	matches := []Credential{}
	for _, cred := range creds {
		if registrarCredentialValid(provider, cred) {
			matches = append(matches, cred)
		}
	}
	if len(matches) == 0 {
		return Credential{}, false, nil
	}
	if len(matches) > 1 {
		return Credential{}, false, fmt.Errorf("存在多个 %s 凭据，请在域名监控中选择一个", domainRegistrarProviderName(provider))
	}
	return matches[0], true, nil
}

func domainRegistrarProviderName(provider string) string {
	switch provider {
	case "digitalplat":
		return "DigitalPlat"
	case "dnshe":
		return "DNSHE"
	default:
		return provider
	}
}

func dnsheBaseURL() string {
	if base := strings.TrimSpace(os.Getenv("DNSHE_API_BASE_URL")); base != "" {
		return base
	}
	return dnsheAPIBaseURL
}

func dnsheRequestURL(endpoint string, action string, params map[string]string) string {
	base := dnsheBaseURL()
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	if q.Get("m") == "" {
		q.Set("m", "domain_hub")
	}
	q.Set("endpoint", endpoint)
	q.Set("action", action)
	for key, value := range params {
		if strings.TrimSpace(value) != "" {
			q.Set(key, value)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func dnsheAPIRequest(method string, requestURL string, cred Credential, body any, target any) error {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, requestURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", strings.TrimSpace(cred.DNSHEAPIKey))
	req.Header.Set("X-API-Secret", strings.TrimSpace(cred.DNSHEAPISecret))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: domainMonitorAPITimeout}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1024*1024))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("DNSHE 返回 %s: %s", res.Status, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	return nil
}

func fetchDNSHEDomainRecord(domain string, cred Credential) (dnsheSubdomain, error) {
	domain = normalizedDomainMonitorHost(domain)
	if domain == "" {
		return dnsheSubdomain{}, errors.New("域名为空")
	}
	rootDomain := ""
	for _, suffix := range dnsheHostedZones {
		suffix = strings.Trim(strings.ToLower(strings.TrimSpace(suffix)), ".")
		if domain == suffix || strings.HasSuffix(domain, "."+suffix) {
			rootDomain = suffix
			break
		}
	}
	requestURL := dnsheRequestURL("subdomains", "list", map[string]string{
		"search":     domain,
		"rootdomain": rootDomain,
		"per_page":   "500",
		"fields":     "id,subdomain,rootdomain,full_domain,status,expires_at,never_expires",
	})
	var payload dnsheSubdomainsResponse
	if err := dnsheAPIRequest(http.MethodGet, requestURL, cred, nil, &payload); err != nil {
		return dnsheSubdomain{}, err
	}
	if !payload.Success {
		return dnsheSubdomain{}, fmt.Errorf("DNSHE 返回失败: %s", firstNonEmpty(payload.Message, payload.Error, payload.ErrorCode, "请求失败"))
	}
	for _, item := range payload.Subdomains {
		fullDomain := normalizedDomainMonitorHost(firstNonEmpty(item.FullDomain, joinDNSHEDomain(item.Subdomain, item.RootDomain)))
		if fullDomain == domain {
			return item, nil
		}
	}
	return dnsheSubdomain{}, fmt.Errorf("DNSHE 未返回域名 %s", domain)
}

func joinDNSHEDomain(subdomain string, rootDomain string) string {
	subdomain = strings.Trim(strings.TrimSpace(subdomain), ".")
	rootDomain = strings.Trim(strings.TrimSpace(rootDomain), ".")
	if subdomain == "" {
		return rootDomain
	}
	if rootDomain == "" {
		return subdomain
	}
	return subdomain + "." + rootDomain
}

func fetchDNSHEDomainExpiry(domain string, cred Credential) (time.Time, error) {
	item, err := fetchDNSHEDomainRecord(domain, cred)
	if err != nil {
		return time.Time{}, err
	}
	if truthy(item.NeverExpires) && strings.TrimSpace(item.ExpiresAt) == "" {
		return time.Date(9999, 12, 31, 23, 59, 59, 0, time.FixedZone("CST", 8*60*60)), nil
	}
	if strings.TrimSpace(item.ExpiresAt) == "" {
		return time.Time{}, fmt.Errorf("DNSHE 未返回 %s 的到期时间", domain)
	}
	expiresAt, err := parseDigitalPlatExpiryDate(item.ExpiresAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("DNSHE 到期时间解析失败: %w", err)
	}
	return expiresAt, nil
}

func renewDNSHEDomain(domain string, credentialID string) (domainRenewResult, error) {
	domain = normalizedDomainMonitorHost(domain)
	registeredDomain, hostedZone, err := domainMonitorRegisteredDomain(domain)
	if err != nil {
		return domainRenewResult{Domain: domain, Status: "error", Error: err.Error()}, err
	}
	if !hostedZone || domainMonitorRegistrarProvider(registeredDomain) != "dnshe" {
		err := errors.New("该域名不支持 DNSHE 续期")
		return domainRenewResult{Domain: registeredDomain, Provider: domainMonitorRegistrarProvider(registeredDomain), Status: "error", Error: err.Error()}, err
	}
	cred, ok, err := domainRegistrarCredentialForDomainMonitor("dnshe", credentialID)
	if err != nil {
		return domainRenewResult{Domain: registeredDomain, Provider: "dnshe", Status: "error", Error: err.Error()}, err
	}
	if !ok {
		err := errDomainRegistrationExpiryUnavailable
		return domainRenewResult{Domain: registeredDomain, Provider: "dnshe", Status: "error", Error: err.Error()}, err
	}
	record, err := fetchDNSHEDomainRecord(registeredDomain, cred)
	if err != nil {
		return domainRenewResult{Domain: registeredDomain, Provider: "dnshe", Status: "error", Error: err.Error()}, err
	}
	requestURL := dnsheRequestURL("subdomains", "renew", nil)
	var payload dnsheRenewResponse
	if err := dnsheAPIRequest(http.MethodPost, requestURL, cred, map[string]any{"subdomain_id": record.ID}, &payload); err != nil {
		return domainRenewResult{Domain: registeredDomain, Provider: "dnshe", Status: "error", Error: err.Error()}, err
	}
	result := domainRenewResult{
		Domain:        registeredDomain,
		Provider:      "dnshe",
		Status:        "ok",
		OldExpiresOn:  payload.OldExpiresAt,
		NewExpiresOn:  payload.NewExpiresAt,
		ChargedAmount: payload.ChargedAmount,
	}
	if !payload.Success {
		result.Status = "error"
		result.Error = firstNonEmpty(payload.Message, payload.Error, payload.ErrorCode, "续期失败")
		return result, errors.New(result.Error)
	}
	return result, nil
}

func truthy(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case string:
		v = strings.TrimSpace(strings.ToLower(v))
		return v == "1" || v == "true" || v == "yes"
	default:
		return false
	}
}

func renewDomainMonitorItem(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Domain string `json:"domain"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	items, err := loadNPMDomainMonitors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range items {
		if items[i].ID != id {
			continue
		}
		if err := validateDomainMonitor(items[i]); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		domains := []string{}
		if strings.TrimSpace(req.Domain) != "" {
			domains = append(domains, req.Domain)
		} else {
			domains = domainMonitorDomains(items[i].DomainNames)
		}
		results := []domainRenewResult{}
		hadError := false
		for _, domain := range domains {
			host, _ := domainMonitorEndpoint(domain)
			result, renewErr := renewDNSHEDomain(host, items[i].CredentialID)
			if renewErr != nil {
				hadError = true
				notifyDomainRenewFailed(result.Domain, renewErr.Error())
			} else {
				notifyDomainRenewSuccess(result.Domain, result.NewExpiresOn)
			}
			results = append(results, result)
		}
		if items[i].Meta == nil {
			items[i].Meta = map[string]any{}
		}
		items[i].Meta["last_renewed"] = localTimestamp(time.Now())
		items[i].Meta["renew_results"] = results
		runDomainMonitorCheck(&items[i])
		if err := saveJSONFile(domainMonitorPath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		appendAuditLog("domain-monitor", id, "renewed", auditMetaForJSONItem(items[i]))
		if hadError {
			writeJSON(w, http.StatusBadRequest, items[i])
			return
		}
		writeJSON(w, http.StatusOK, items[i])
		return
	}
	http.Error(w, "item not found", http.StatusNotFound)
}

func parseReminderDays(value string) []int {
	out := []int{}
	seen := map[int]bool{}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == ' ' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		days, err := strconv.Atoi(part)
		if err != nil || days < 0 || seen[days] {
			continue
		}
		seen[days] = true
		out = append(out, days)
	}
	sort.Ints(out)
	return out
}
