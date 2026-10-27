package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//
// ===== Cloudflare Registrar API =====
//

// cloudflareRegistrarAccountID 获取 Cloudflare API Token 所属的账户 ID。
// 优先使用凭据里填写的 cf_account_id，否则通过 accounts 列表自动获取。
func cloudflareRegistrarAccountID(cred Credential) (string, error) {
	if accountID := strings.TrimSpace(credentialSecretValue(cred, "cf_account_id")); accountID != "" {
		return accountID, nil
	}
	token := strings.TrimSpace(credentialSecretValue(cred, "cf_token"))
	if token == "" {
		return "", fmt.Errorf("Cloudflare API Token 为空")
	}

	reqURL := strings.TrimRight(cloudflareAPIBaseURL, "/") + "/accounts"
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: domainMonitorAPITimeout}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("Cloudflare 返回 %s: %s", res.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Success bool `json:"success"`
		Result  []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
		Errors []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if !payload.Success {
		if len(payload.Errors) > 0 {
			return "", fmt.Errorf("Cloudflare 返回失败: %s", payload.Errors[0].Message)
		}
		return "", fmt.Errorf("Cloudflare 返回失败")
	}
	if len(payload.Result) == 0 {
		return "", fmt.Errorf("Cloudflare Token 没有关联任何账户")
	}
	return payload.Result[0].ID, nil
}

// cloudflareRegistrarDomains 获取账户下全部已注册域名。
func cloudflareRegistrarDomains(cred Credential) ([]cloudflareRegistrarDomain, error) {
	accountID, err := cloudflareRegistrarAccountID(cred)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(credentialSecretValue(cred, "cf_token"))

	reqURL := strings.TrimRight(cloudflareAPIBaseURL, "/") + "/accounts/" + accountID + "/registrar/domains?per_page=200"
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: domainMonitorAPITimeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("Cloudflare 返回 %s: %s", res.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Success bool                        `json:"success"`
		Result  []cloudflareRegistrarDomain `json:"result"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if !payload.Success {
		if len(payload.Errors) > 0 {
			return nil, fmt.Errorf("Cloudflare 返回失败: %s", payload.Errors[0].Message)
		}
		return nil, fmt.Errorf("Cloudflare 返回失败")
	}
	return payload.Result, nil
}

type cloudflareRegistrarDomain struct {
	Name      string `json:"name"`
	ExpiresOn int64  `json:"expires_on"`
	Status    string `json:"status,omitempty"`
}

// fetchCloudflareRegistrarExpiry 查询 Cloudflare Registrar 域名到期时间。
func fetchCloudflareRegistrarExpiry(domain string, cred Credential) (time.Time, error) {
	domain = normalizedDomainMonitorHost(domain)
	if domain == "" {
		return time.Time{}, fmt.Errorf("域名为空")
	}
	domains, err := cloudflareRegistrarDomains(cred)
	if err != nil {
		return time.Time{}, err
	}
	for _, item := range domains {
		if normalizedDomainMonitorHost(item.Name) == domain {
			if item.ExpiresOn <= 0 {
				return time.Time{}, fmt.Errorf("Cloudflare 未返回 %s 的到期时间", domain)
			}
			return time.Unix(item.ExpiresOn, 0), nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: Cloudflare 未返回域名 %s", errDomainRegistrationExpiryUnavailable, domain)
}

// renewCloudflareRegistrarDomain 通过 Cloudflare Registrar API 续期域名。
func renewCloudflareRegistrarDomain(domain string, cred Credential) (domainRenewResult, error) {
	domain = normalizedDomainMonitorHost(domain)
	accountID, err := cloudflareRegistrarAccountID(cred)
	if err != nil {
		return domainRenewResult{Domain: domain, Provider: "cloudflare", Status: "error", Error: err.Error()}, err
	}
	token := strings.TrimSpace(credentialSecretValue(cred, "cf_token"))

	reqURL := strings.TrimRight(cloudflareAPIBaseURL, "/") + "/accounts/" + accountID + "/registrar/domains/" + domain + "/renew"
	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		return domainRenewResult{Domain: domain, Provider: "cloudflare", Status: "error", Error: err.Error()}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: domainMonitorAPITimeout}
	res, err := client.Do(req)
	if err != nil {
		return domainRenewResult{Domain: domain, Provider: "cloudflare", Status: "error", Error: err.Error()}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return domainRenewResult{Domain: domain, Provider: "cloudflare", Status: "error", Error: err.Error()}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return domainRenewResult{Domain: domain, Provider: "cloudflare", Status: "error", Error: fmt.Sprintf("Cloudflare 返回 %s: %s", res.Status, strings.TrimSpace(string(body)))}, fmt.Errorf("Cloudflare 续期失败")
	}

	var payload struct {
		Success bool `json:"success"`
		Result  struct {
			Name      string `json:"name"`
			ExpiresOn int64  `json:"expires_on"`
		} `json:"result"`
		Errors []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return domainRenewResult{Domain: domain, Provider: "cloudflare", Status: "error", Error: err.Error()}, err
	}
	if !payload.Success {
		message := "Cloudflare 续期失败"
		if len(payload.Errors) > 0 {
			message = payload.Errors[0].Message
		}
		return domainRenewResult{Domain: domain, Provider: "cloudflare", Status: "error", Error: message}, fmt.Errorf("%s", message)
	}

	result := domainRenewResult{
		Domain:   domain,
		Provider: "cloudflare",
		Status:   "ok",
	}
	if payload.Result.ExpiresOn > 0 {
		result.NewExpiresOn = time.Unix(payload.Result.ExpiresOn, 0).Format(time.RFC3339)
	}
	return result, nil
}

//
// ===== Porkbun Registrar API =====
//

type porkbunDomainListResponse struct {
	Status   string                   `json:"status"`
	Response []porkbunRegistrarDomain `json:"response"`
	Message  string                   `json:"message,omitempty"`
	Code     string                   `json:"code,omitempty"`
}

type porkbunRegistrarDomain struct {
	Domain  string `json:"domain"`
	Expires string `json:"expires"`
	Status  string `json:"status,omitempty"`
}

// porkbunAPIRequest 发送 Porkbun API 请求，认证走 X-API-Key / X-Secret-API-Key。
func porkbunAPIRequest(endpoint string, cred Credential, body any, target any) error {
	apiKey := strings.TrimSpace(credentialSecretValue(cred, "porkbun_api_key"))
	secretKey := strings.TrimSpace(credentialSecretValue(cred, "porkbun_api_secret_key"))
	if apiKey == "" || secretKey == "" {
		return fmt.Errorf("Porkbun API Key 或 Secret Key 为空")
	}

	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(porkbunAPIBaseURL, "/")+"/"+endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("X-Secret-API-Key", secretKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: domainMonitorAPITimeout}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Porkbun 返回 %s: %s", res.Status, strings.TrimSpace(string(data)))
	}
	if target != nil {
		if err := json.Unmarshal(data, target); err != nil {
			return err
		}
	}
	return nil
}

// fetchPorkbunDomainExpiry 查询 Porkbun 域名到期时间。
func fetchPorkbunDomainExpiry(domain string, cred Credential) (time.Time, error) {
	domain = normalizedDomainMonitorHost(domain)
	if domain == "" {
		return time.Time{}, fmt.Errorf("域名为空")
	}

	var payload porkbunDomainListResponse
	if err := porkbunAPIRequest("domain/list", cred, map[string]any{}, &payload); err != nil {
		return time.Time{}, err
	}
	if strings.ToUpper(payload.Status) != "SUCCESS" {
		return time.Time{}, fmt.Errorf("Porkbun 返回失败: %s", firstNonEmpty(payload.Message, payload.Code, "请求失败"))
	}

	for _, item := range payload.Response {
		if normalizedDomainMonitorHost(item.Domain) == domain {
			expiresAt, err := parsePorkbunExpiryDate(item.Expires)
			if err != nil {
				return time.Time{}, err
			}
			return expiresAt, nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: Porkbun 未返回域名 %s", errDomainRegistrationExpiryUnavailable, domain)
}

// parsePorkbunExpiryDate 解析 Porkbun 到期时间，支持多种格式。
func parsePorkbunExpiryDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("Porkbun 未返回到期时间")
	}
	// 优先按 RFC3339 解析。
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	// 再按 Porkbun 常见的 "2024-01-01" 格式解析（UTC）。
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t.UTC(), nil
	}
	// 最后尝试 Unix 时间戳。
	var ts int64
	if _, err := fmt.Sscanf(value, "%d", &ts); err == nil && ts > 0 {
		return time.Unix(ts, 0), nil
	}
	return time.Time{}, fmt.Errorf("Porkbun 到期时间解析失败: %s", value)
}

type porkbunRenewResponse struct {
	Status   string `json:"status"`
	Response struct {
		Domain  string `json:"domain"`
		Expires string `json:"expires"`
	} `json:"response"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

// renewPorkbunDomain 通过 Porkbun API 续期域名。
func renewPorkbunDomain(domain string, cred Credential) (domainRenewResult, error) {
	domain = normalizedDomainMonitorHost(domain)

	var payload porkbunRenewResponse
	if err := porkbunAPIRequest("domain/renew/"+domain, cred, map[string]any{}, &payload); err != nil {
		return domainRenewResult{Domain: domain, Provider: "porkbun", Status: "error", Error: err.Error()}, err
	}
	if strings.ToUpper(payload.Status) != "SUCCESS" {
		message := firstNonEmpty(payload.Message, payload.Code, "续期失败")
		return domainRenewResult{Domain: domain, Provider: "porkbun", Status: "error", Error: message}, fmt.Errorf("%s", message)
	}

	result := domainRenewResult{
		Domain:   domain,
		Provider: "porkbun",
		Status:   "ok",
	}
	if payload.Response.Expires != "" {
		if expiresAt, err := parsePorkbunExpiryDate(payload.Response.Expires); err == nil {
			result.NewExpiresOn = expiresAt.Format(time.RFC3339)
		}
	}
	return result, nil
}

// ensure net/url is used (kept for future URL construction).
var _ = url.PathEscape
