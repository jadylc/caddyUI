package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

//
// ===== RDAP 增强 =====
//

// rdapBootstrapResponse IANA RDAP bootstrap 响应。
type rdapBootstrapResponse struct {
	Services [][]string `json:"services"`
}

// rdapTLDServers 返回 TLD 对应的 RDAP 服务器列表。
// 优先使用缓存的 IANA bootstrap，未命中时回退到 rdap.org。
var rdapTLDServers = map[string][]string{}

// rdapBootstrapOnce 确保 bootstrap 只加载一次。
var rdapBootstrapOnce sync.Once

// fetchRDAPBootstrap 从 IANA 拉取 RDAP bootstrap 列表并缓存。
func fetchRDAPBootstrap() {
	rdapBootstrapOnce.Do(func() {
		client := &http.Client{Timeout: domainMonitorRDAPTimeout}
		res, err := client.Get("https://rdap.org/dns/" + url.PathEscape(""))
		if err != nil {
			return
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		if err != nil {
			return
		}
		var payload rdapBootstrapResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			return
		}
		for _, service := range payload.Services {
			if len(service) < 2 {
				continue
			}
			tlds := strings.Split(service[0], ",")
			servers := service[1]
			for _, tld := range tlds {
				tld = strings.TrimSpace(strings.ToLower(tld))
				if tld != "" {
					rdapTLDServers[tld] = strings.Split(servers, " ")
				}
			}
		}
	})
}

// rdapServersForTLD 返回 TLD 的 RDAP 服务器列表，附带 rdap.org 兜底。
func rdapServersForTLD(tld string) []string {
	fetchRDAPBootstrap()
	servers := append([]string{}, rdapTLDServers[tld]...)
	// rdap.org 作为兜底。
	if !containsString(servers, "https://rdap.org/domain/") {
		servers = append(servers, "https://rdap.org/domain/")
	}
	return servers
}

// containsString 检查字符串是否在切片中。
func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// fetchRDAPExpiry 从 RDAP 服务器查询域名到期时间。
// 依次尝试多个 RDAP 服务器，任一成功即返回。
func fetchRDAPExpiry(domain string) (time.Time, error) {
	tld := rdapTLD(domain)
	servers := rdapServersForTLD(tld)

	lastErr := errors.New("没有可用的 RDAP 服务器")
	for _, base := range servers {
		rdapURL := strings.TrimRight(base, "/") + "/" + url.PathEscape(domain)
		expiresAt, err := fetchRDAPExpiryFromURL(rdapURL)
		if err == nil {
			return expiresAt, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

// fetchRDAPExpiryFromURL 从指定 RDAP URL 查询到期时间。
func fetchRDAPExpiryFromURL(rdapURL string) (time.Time, error) {
	client := &http.Client{Timeout: domainMonitorRDAPTimeout}
	res, err := client.Get(rdapURL)
	if err != nil {
		return time.Time{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return time.Time{}, fmt.Errorf("RDAP 返回 %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 1024*1024))
	if err != nil {
		return time.Time{}, err
	}
	var payload rdapDomainResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return time.Time{}, err
	}
	for _, event := range payload.Events {
		action := strings.ToLower(strings.TrimSpace(event.EventAction))
		if !strings.Contains(action, "expiration") && !strings.Contains(action, "expiry") {
			continue
		}
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(event.EventDate))
		if err != nil {
			return time.Time{}, err
		}
		return t, nil
	}
	return time.Time{}, errors.New("RDAP 未返回域名到期时间")
}

// rdapTLD 提取域名的 TLD（最后一段）。
func rdapTLD(domain string) string {
	domain = normalizedDomainMonitorHost(domain)
	if domain == "" {
		return ""
	}
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

//
// ===== WHOIS 回退 =====
//

// whoisTLDServers 常见 TLD 的 WHOIS 服务器映射。
var whoisTLDServers = map[string]string{
	"com":    "whois.verisign-grs.com",
	"net":    "whois.verisign-grs.com",
	"org":    "whois.pir.org",
	"info":   "whois.afilias.net",
	"io":     "whois.nic.io",
	"ai":     "whois.nic.ai",
	"app":    "whois.nic.google",
	"dev":    "whois.nic.google",
	"xyz":    "whois.nic.xyz",
	"top":    "whois.nic.top",
	"site":   "whois.nic.site",
	"store":  "whois.nic.store",
	"online": "whois.nic.online",
	"tech":   "whois.nic.tech",
	"cloud":  "whois.nic.cloud",
	"club":   "whois.nic.club",
	"live":   "whois.nic.live",
	"pro":    "whois.afilias.net",
	"me":     "whois.nic.me",
	"co":     "whois.nic.co",
	"uk":     "whois.nic.uk",
	"cn":     "whois.cnnic.cn",
	"de":     "whois.denic.de",
	"fr":     "whois.nic.fr",
	"nl":     "whois.domain-registry.nl",
	"jp":     "whois.jprs.jp",
	"ru":     "whois.tcinet.ru",
	"au":     "whois.auda.org.au",
	"ca":     "whois.cira.ca",
	"in":     "whois.registry.in",
}

// whoisIANAServer IANA WHOIS 服务器，用于查询 TLD 的 WHOIS 服务器。
const whoisIANAServer = "whois.iana.org"

// whoisServerForTLD 返回 TLD 的 WHOIS 服务器。
// 优先查缓存，其次查常见 TLD 映射，最后通过 IANA 查询。
func whoisServerForTLD(tld string) string {
	tld = strings.ToLower(strings.TrimSpace(tld))
	if server, ok := whoisTLDServers[tld]; ok {
		return server
	}
	// 通过 IANA 查询。
	server, err := whoisQueryServer(whoisIANAServer, tld)
	if err != nil {
		return ""
	}
	return server
}

// whoisQueryServer 向 WHOIS 服务器查询指定 TLD 的 WHOIS 服务器。
func whoisQueryServer(server, query string) (string, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(server, "43"), domainMonitorTimeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(domainMonitorTimeout))

	if _, err := conn.Write([]byte(query + "\r\n")); err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lower := strings.ToLower(line)
		// IANA 响应格式: "refer: whois.example.com" 或 "whois: whois.example.com"
		if strings.HasPrefix(lower, "refer:") || strings.HasPrefix(lower, "whois:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}
	return "", errors.New("WHOIS 未返回服务器")
}

// whoisQueryDomain 向 WHOIS 服务器查询域名信息。
func whoisQueryDomain(server, domain string) (string, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(server, "43"), domainMonitorTimeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(domainMonitorTimeout))

	if _, err := conn.Write([]byte(domain + "\r\n")); err != nil {
		return "", err
	}
	var sb strings.Builder
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// whoisExpiryPatterns WHOIS 响应中到期时间的字段名。
var whoisExpiryPatterns = []string{
	"Registry Expiry Date",
	"Expiry Date",
	"Expiration Date",
	"Registrar Registration Expiration Date",
	"paid-till",
	"paid-until",
	"Domain Expiration Date",
	"Expiration Time",
}

// parseWhoisExpiry 从 WHOIS 文本中解析到期时间。
func parseWhoisExpiry(text string) (time.Time, error) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, pattern := range whoisExpiryPatterns {
			if !strings.HasPrefix(line, pattern+":") {
				continue
			}
			value := strings.TrimSpace(strings.TrimPrefix(line, pattern+":"))
			// 尝试多种日期格式。
			formats := []string{
				"2006-01-02T15:04:05Z",
				"2006-01-02T15:04:05.000Z",
				"2006-01-02 15:04:05",
				"2006-01-02",
				"2006.01.02",
				"02-Jan-2006",
				"20060102",
				"20060102T150405Z",
				"20060102T150405.000Z",
			}
			for _, format := range formats {
				if t, err := time.Parse(format, value); err == nil {
					return t, nil
				}
			}
		}
	}
	return time.Time{}, errors.New("WHOIS 未返回域名到期时间")
}

// fetchWHOISExpiry 通过 WHOIS 查询域名到期时间。
func fetchWHOISExpiry(domain string) (time.Time, error) {
	tld := rdapTLD(domain)
	server := whoisServerForTLD(tld)
	if server == "" {
		return time.Time{}, fmt.Errorf("WHOIS: 无法确定 %s 的 WHOIS 服务器", domain)
	}
	text, err := whoisQueryDomain(server, domain)
	if err != nil {
		return time.Time{}, err
	}
	return parseWhoisExpiry(text)
}

// fetchDomainExpiryFallback 依次尝试 RDAP 和 WHOIS，返回第一个成功的到期时间。
// 这是通用兜底路径，不需要域名商凭据。
func fetchDomainExpiryFallback(domain string) (time.Time, error) {
	// 先试 RDAP（支持多服务器 + rdap.org 兜底）。
	if expiresAt, err := fetchRDAPExpiry(domain); err == nil {
		return expiresAt, nil
	}
	// 再试 WHOIS。
	if expiresAt, err := fetchWHOISExpiry(domain); err == nil {
		return expiresAt, nil
	}
	return time.Time{}, fmt.Errorf("RDAP 和 WHOIS 均未返回 %s 的到期时间", domain)
}
