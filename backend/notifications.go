package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	notificationEventDomainExpiring         = "domain_expiring"
	notificationEventDomainExpired          = "domain_expired"
	notificationEventDomainRenewSuccess     = "domain_renew_success"
	notificationEventDomainRenewFailed      = "domain_renew_failed"
	notificationEventDomainCheckFailed      = "domain_check_failed"
	notificationEventSSLExpiring            = "ssl_expiring"
	notificationEventSSLExpired             = "ssl_expired"
	notificationEventSSLCheckFailed         = "ssl_check_failed"
	notificationEventDNSCheckFailed         = "dns_check_failed"
	notificationEventMonitorFailed          = "monitor_failed"
	notificationEventMonitorRecovered       = "monitor_recovered"
	notificationEventCertificateRenewed     = "certificate_renewed"
	notificationEventCertificateRenewFailed = "certificate_renew_failed"
	notificationEventTest                   = "test"
)

type notificationChannel struct {
	ID           int            `json:"id"`
	CreatedOn    string         `json:"created_on"`
	ModifiedOn   string         `json:"modified_on"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	URL          string         `json:"url,omitempty"`
	Method       string         `json:"method,omitempty"`
	Headers      string         `json:"headers,omitempty"`
	BodyTemplate string         `json:"body_template,omitempty"`
	ProxyURL     string         `json:"proxy_url,omitempty"`
	Token        string         `json:"token,omitempty"`
	Secret       string         `json:"secret,omitempty"`
	ChatID       string         `json:"chat_id,omitempty"`
	Events       []string       `json:"events,omitempty"`
	Enabled      bool           `json:"enabled"`
	Meta         map[string]any `json:"meta,omitempty"`
	LastError    string         `json:"last_error,omitempty"`
	LastSentAt   string         `json:"last_sent_at,omitempty"`
}

type notificationEventSpec struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

var notificationEventSpecs = []notificationEventSpec{
	{notificationEventDomainExpiring, "域名即将到期", "域名到达配置的提前提醒天数"},
	{notificationEventDomainExpired, "域名已过期", "域名注册到期时间已经过去"},
	{notificationEventDomainRenewSuccess, "域名续期成功", "DNSHE 手动或自动续期成功"},
	{notificationEventDomainRenewFailed, "域名续期失败", "DNSHE 手动或自动续期失败"},
	{notificationEventDomainCheckFailed, "域名到期检查失败", "RDAP 或注册商 API 检查失败"},
	{notificationEventSSLExpiring, "证书即将到期", "证书到达监控告警阈值"},
	{notificationEventSSLExpired, "证书已过期", "证书有效期已经过去"},
	{notificationEventSSLCheckFailed, "证书检查失败", "TLS 探测或证书校验失败"},
	{notificationEventDNSCheckFailed, "DNS 检查失败", "DNS 解析失败"},
	{notificationEventMonitorFailed, "监控异常", "域名监控整体状态从非错误变为错误"},
	{notificationEventMonitorRecovered, "监控恢复", "域名监控整体状态从错误恢复"},
	{notificationEventCertificateRenewed, "证书签发/续期成功", "Caddy 获得证书成功"},
	{notificationEventCertificateRenewFailed, "证书签发/续期失败", "Caddy 证书申请或续期失败"},
}

func loadNotificationChannels() ([]notificationChannel, error) {
	items := []notificationChannel{}
	if err := loadJSONFile(notificationChannelPath, &items); err != nil {
		return nil, err
	}
	fallback := timestampFromFile(notificationChannelPath)
	for i := range items {
		stampNotificationChannel(&items[i], fallback, false)
	}
	return items, nil
}

func stampNotificationChannel(item *notificationChannel, fallback string, touchModified bool) {
	stampCreatedModified(&item.CreatedOn, &item.ModifiedOn, fallback, touchModified)
	if item.Events == nil {
		item.Events = []string{}
	}
	if item.Meta == nil {
		item.Meta = map[string]any{}
	}
}

func validateNotificationChannel(item notificationChannel) error {
	if strings.TrimSpace(item.Name) == "" {
		return fmt.Errorf("推送渠道名称不能为空")
	}
	if _, err := parseNotificationProxyURL(item.ProxyURL); err != nil {
		return err
	}
	switch item.Type {
	case "webhook":
		if strings.TrimSpace(item.URL) == "" {
			return fmt.Errorf("Webhook 地址不能为空")
		}
	case "bark":
		if strings.TrimSpace(item.URL) == "" && strings.TrimSpace(item.Token) == "" {
			return fmt.Errorf("Bark 需要填写推送地址或 Key")
		}
	case "serverchan":
		if strings.TrimSpace(item.URL) == "" && strings.TrimSpace(item.Token) == "" {
			return fmt.Errorf("Server酱 需要填写 SendKey 或完整地址")
		}
	case "gotify":
		if strings.TrimSpace(item.URL) == "" || strings.TrimSpace(item.Token) == "" {
			return fmt.Errorf("Gotify 需要填写地址和 Token")
		}
	case "telegram":
		if strings.TrimSpace(item.Token) == "" || strings.TrimSpace(item.ChatID) == "" {
			return fmt.Errorf("Telegram 需要填写 Bot Token 和 Chat ID")
		}
	case "wecom", "dingtalk", "feishu":
		if strings.TrimSpace(item.URL) == "" {
			return fmt.Errorf("群机器人 Webhook 地址不能为空")
		}
	default:
		return fmt.Errorf("未知推送渠道：%s", item.Type)
	}
	return nil
}

func notificationChannelsHandler(w http.ResponseWriter, r *http.Request) {
	items, err := loadNotificationChannels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"channels": items, "events": notificationEventSpecs})
	case http.MethodPost:
		var item notificationChannel
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		item.ID = stableID(fmt.Sprintf("notification:%s:%d", item.Name, time.Now().UnixNano()))
		stampNotificationChannel(&item, "", true)
		if err := validateNotificationChannel(item); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items = append(items, item)
		if err := saveJSONFile(notificationChannelPath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		appendAuditLog("notification-channel", item.ID, "created", map[string]any{"name": item.Name, "type": item.Type})
		writeJSON(w, http.StatusCreated, item)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func notificationChannelItemHandler(w http.ResponseWriter, r *http.Request) {
	prefix := trimAnyPrefix(r.URL.Path, "/notifications/channels/", "/caddy/notifications/channels/", "/nginx/notifications/channels/")
	parts := strings.Split(strings.Trim(prefix, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if len(parts) == 2 && parts[1] == "test" {
		testNotificationChannel(w, r, id)
		return
	}
	items, err := loadNotificationChannels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idx := -1
	for i := range items {
		if items[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		http.Error(w, "channel not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, items[idx])
	case http.MethodPut:
		var item notificationChannel
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		item.ID = id
		item.CreatedOn = items[idx].CreatedOn
		stampNotificationChannel(&item, "", true)
		if err := validateNotificationChannel(item); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items[idx] = item
		if err := saveJSONFile(notificationChannelPath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		appendAuditLog("notification-channel", id, "updated", map[string]any{"name": item.Name, "type": item.Type})
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		deleted := items[idx]
		items = append(items[:idx], items[idx+1:]...)
		if err := saveJSONFile(notificationChannelPath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		appendAuditLog("notification-channel", id, "deleted", map[string]any{"name": deleted.Name, "type": deleted.Type})
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func testNotificationChannel(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := loadNotificationChannels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range items {
		if items[i].ID != id {
			continue
		}
		err := deliverNotification(items[i], notificationEventTest, "Caddy UI 推送测试", "这是一条测试推送。", map[string]any{"time": localTimestamp(time.Now())})
		if err != nil {
			items[i].LastError = err.Error()
			_ = saveJSONFile(notificationChannelPath, items)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items[i].LastError = ""
		items[i].LastSentAt = localTimestamp(time.Now())
		_ = saveJSONFile(notificationChannelPath, items)
		writeJSON(w, http.StatusOK, items[i])
		return
	}
	http.Error(w, "channel not found", http.StatusNotFound)
}

func channelAllowsEvent(channel notificationChannel, event string) bool {
	if len(channel.Events) == 0 {
		return true
	}
	for _, value := range channel.Events {
		value = strings.TrimSpace(value)
		if value == "*" || value == "all" || value == event {
			return true
		}
	}
	return false
}

func sendNotification(event string, title string, message string, data map[string]any, dedupeKey string, interval time.Duration) {
	if dedupeKey != "" && !notificationDedupeAllowed(event+":"+dedupeKey, interval) {
		return
	}
	items, err := loadNotificationChannels()
	if err != nil {
		logNotificationError("load channels", err)
		return
	}
	changed := false
	for i := range items {
		if !items[i].Enabled || !channelAllowsEvent(items[i], event) {
			continue
		}
		if err := deliverNotification(items[i], event, title, message, data); err != nil {
			items[i].LastError = err.Error()
			logNotificationError(items[i].Name, err)
		} else {
			items[i].LastError = ""
			items[i].LastSentAt = localTimestamp(time.Now())
		}
		changed = true
	}
	if changed {
		_ = saveJSONFile(notificationChannelPath, items)
	}
}

func logNotificationError(name string, err error) {
	if err != nil {
		log.Printf("notification %s: %v", name, err)
	}
}

func notificationDedupeAllowed(key string, interval time.Duration) bool {
	if interval <= 0 {
		return true
	}
	state := map[string]string{}
	_ = loadJSONFile(notificationStatePath, &state)
	if lastText := state[key]; lastText != "" {
		if last, err := time.Parse(time.RFC3339, lastText); err == nil && time.Since(last) < interval {
			return false
		}
	}
	state[key] = time.Now().Format(time.RFC3339)
	_ = saveJSONFile(notificationStatePath, state)
	return true
}

func deliverNotification(channel notificationChannel, event string, title string, message string, data map[string]any) error {
	text := strings.TrimSpace(title + "\n" + message)
	if strings.TrimSpace(channel.BodyTemplate) != "" {
		text = renderNotificationTemplate(channel.BodyTemplate, event, title, message, data)
	}
	switch channel.Type {
	case "webhook":
		return sendWebhookNotification(channel, event, title, message, text, data)
	case "bark":
		return sendJSONNotification(channel, barkURL(channel), map[string]any{"title": title, "body": message, "group": "Caddy UI"})
	case "serverchan":
		return sendFormNotification(channel, serverChanURL(channel), url.Values{"title": {title}, "desp": {message}})
	case "gotify":
		return sendJSONNotification(channel, gotifyURL(channel), map[string]any{"title": title, "message": message, "priority": 5})
	case "telegram":
		return sendJSONNotification(channel, "https://api.telegram.org/bot"+strings.TrimSpace(channel.Token)+"/sendMessage", map[string]any{"chat_id": channel.ChatID, "text": text})
	case "wecom":
		return sendJSONNotification(channel, channel.URL, map[string]any{"msgtype": "text", "text": map[string]any{"content": text}})
	case "dingtalk":
		return sendJSONNotification(channel, channel.URL, map[string]any{"msgtype": "text", "text": map[string]any{"content": text}})
	case "feishu":
		return sendJSONNotification(channel, channel.URL, map[string]any{"msg_type": "text", "content": map[string]any{"text": text}})
	default:
		return fmt.Errorf("未知推送渠道：%s", channel.Type)
	}
}

func sendWebhookNotification(channel notificationChannel, event string, title string, message string, text string, data map[string]any) error {
	method := strings.ToUpper(strings.TrimSpace(channel.Method))
	if method == "" {
		method = http.MethodPost
	}
	body := any(map[string]any{"event": event, "title": title, "message": message, "text": text, "data": data})
	if strings.TrimSpace(channel.BodyTemplate) != "" {
		return sendRawNotification(method, channel.URL, "application/json", []byte(text), channel.Headers, channel.ProxyURL)
	}
	payload, _ := json.Marshal(body)
	return sendRawNotification(method, channel.URL, "application/json", payload, channel.Headers, channel.ProxyURL)
}

func sendJSONNotification(channel notificationChannel, targetURL string, body any) error {
	payload, _ := json.Marshal(body)
	return sendRawNotification(http.MethodPost, targetURL, "application/json", payload, "", channel.ProxyURL)
}

func sendFormNotification(channel notificationChannel, targetURL string, values url.Values) error {
	return sendRawNotification(http.MethodPost, targetURL, "application/x-www-form-urlencoded", []byte(values.Encode()), "", channel.ProxyURL)
}

func sendRawNotification(method string, targetURL string, contentType string, payload []byte, headersRaw string, proxyURL string) error {
	req, err := http.NewRequest(method, targetURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range parseNotificationHeaders(headersRaw) {
		req.Header.Set(key, value)
	}
	client, err := notificationHTTPClient(proxyURL)
	if err != nil {
		return err
	}
	if transport, ok := client.Transport.(*http.Transport); ok {
		defer transport.CloseIdleConnections()
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 256*1024))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("HTTP %s: %s", res.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func notificationHTTPClient(proxyURL string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	parsedProxy, err := parseNotificationProxyURL(proxyURL)
	if err != nil {
		return nil, err
	}
	if parsedProxy != nil {
		transport.Proxy = http.ProxyURL(parsedProxy)
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: transport}, nil
}

func parseNotificationProxyURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "//") {
		raw = "http:" + raw
	} else if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("代理地址格式不正确")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
		return parsed, nil
	default:
		return nil, fmt.Errorf("代理地址仅支持 http、https、socks5")
	}
}

func parseNotificationHeaders(raw string) map[string]string {
	out := map[string]string{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out
	}
	var asJSON map[string]string
	if json.Unmarshal([]byte(raw), &asJSON) == nil {
		return asJSON
	}
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return out
}

func barkURL(channel notificationChannel) string {
	if strings.TrimSpace(channel.URL) != "" {
		return channel.URL
	}
	return "https://api.day.app/" + strings.TrimSpace(channel.Token)
}

func serverChanURL(channel notificationChannel) string {
	if strings.TrimSpace(channel.URL) != "" {
		return channel.URL
	}
	return "https://sctapi.ftqq.com/" + strings.TrimSpace(channel.Token) + ".send"
}

func gotifyURL(channel notificationChannel) string {
	base := strings.TrimRight(strings.TrimSpace(channel.URL), "/")
	return base + "/message?token=" + url.QueryEscape(strings.TrimSpace(channel.Token))
}

func renderNotificationTemplate(template string, event string, title string, message string, data map[string]any) string {
	replacements := map[string]string{
		"event":   event,
		"title":   title,
		"message": message,
	}
	for key, value := range data {
		replacements[key] = fmt.Sprint(value)
	}
	out := template
	for key, value := range replacements {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
	}
	return out
}

func notifyDomainRenewSuccess(domain string, expiresOn string) {
	sendNotification(
		notificationEventDomainRenewSuccess,
		"域名续期成功",
		fmt.Sprintf("%s 已续期，新到期时间：%s", domain, firstNonEmpty(expiresOn, "未知")),
		map[string]any{"domain": domain, "expires_on": expiresOn},
		domain,
		time.Hour,
	)
}

func notifyDomainRenewFailed(domain string, reason string) {
	sendNotification(
		notificationEventDomainRenewFailed,
		"域名续期失败",
		fmt.Sprintf("%s 续期失败：%s", domain, reason),
		map[string]any{"domain": domain, "error": reason},
		domain+":"+reason,
		6*time.Hour,
	)
}

func notifyCertificateRenewed(domain string) {
	sendNotification(notificationEventCertificateRenewed, "证书签发/续期成功", domain+" 的证书已由 Caddy 成功获取。", map[string]any{"domain": domain}, domain, time.Hour)
}

func notifyCertificateRenewFailed(domain string, reason string) {
	sendNotification(notificationEventCertificateRenewFailed, "证书签发/续期失败", fmt.Sprintf("%s 证书失败：%s", domain, reason), map[string]any{"domain": domain, "error": reason}, domain+":"+reason, 6*time.Hour)
}

func effectiveReminderDays(item npmDomainMonitor) []int {
	if len(item.ReminderDays) > 0 {
		days := append([]int{}, item.ReminderDays...)
		sort.Ints(days)
		return days
	}
	return []int{1, 3, 7, 15, 30}
}

func effectiveRenewBeforeDays(item *npmDomainMonitor) int {
	if item.RenewBefore > 0 {
		return item.RenewBefore
	}
	return 30
}

func notifyDomainExpiryIfNeeded(domain string, daysLeft int, expiresAt time.Time, item npmDomainMonitor) {
	if domain == "" {
		return
	}
	if daysLeft < 0 {
		sendNotification(
			notificationEventDomainExpired,
			"域名已过期",
			fmt.Sprintf("%s 已于 %s 过期", domain, localTimestamp(expiresAt)),
			map[string]any{"domain": domain, "days_left": daysLeft, "expires_on": localTimestamp(expiresAt)},
			domain+":expired:"+expiresAt.Format("2006-01-02"),
			24*time.Hour,
		)
		return
	}
	for _, days := range effectiveReminderDays(item) {
		if daysLeft > days {
			continue
		}
		sendNotification(
			notificationEventDomainExpiring,
			"域名即将到期",
			fmt.Sprintf("%s 将在 %d 天后到期，到期时间：%s", domain, daysLeft, localTimestamp(expiresAt)),
			map[string]any{"domain": domain, "days_left": daysLeft, "expires_on": localTimestamp(expiresAt), "reminder_days": days},
			fmt.Sprintf("%s:reminder:%d:%s", domain, days, expiresAt.Format("2006-01-02")),
			24*time.Hour,
		)
		return
	}
}

func notifySSLExpiryIfNeeded(domain string, daysLeft int, expiresAt time.Time, threshold int) {
	if domain == "" {
		return
	}
	if daysLeft < 0 {
		sendNotification(
			notificationEventSSLExpired,
			"证书已过期",
			fmt.Sprintf("%s 的证书已于 %s 过期", domain, localTimestamp(expiresAt)),
			map[string]any{"domain": domain, "days_left": daysLeft, "expires_on": localTimestamp(expiresAt)},
			domain+":ssl-expired:"+expiresAt.Format("2006-01-02"),
			24*time.Hour,
		)
		return
	}
	if daysLeft <= threshold {
		sendNotification(
			notificationEventSSLExpiring,
			"证书即将到期",
			fmt.Sprintf("%s 的证书将在 %d 天后到期，到期时间：%s", domain, daysLeft, localTimestamp(expiresAt)),
			map[string]any{"domain": domain, "days_left": daysLeft, "expires_on": localTimestamp(expiresAt), "threshold": threshold},
			fmt.Sprintf("%s:ssl-expiring:%s", domain, expiresAt.Format("2006-01-02")),
			24*time.Hour,
		)
	}
}

func notifyDomainMonitorCheckFailed(event string, domain string, title string, reason string) {
	sendNotification(
		event,
		title,
		fmt.Sprintf("%s：%s", firstNonEmpty(domain, "未知域名"), reason),
		map[string]any{"domain": domain, "error": reason},
		firstNonEmpty(domain, "unknown")+":"+reason,
		6*time.Hour,
	)
}

func notifyMonitorStatusChange(name string, previousStatus string, currentStatus string, reason string) {
	if currentStatus == "error" && previousStatus != "error" {
		sendNotification(
			notificationEventMonitorFailed,
			"域名监控异常",
			fmt.Sprintf("%s 检查异常：%s", name, firstNonEmpty(reason, "请查看监控详情")),
			map[string]any{"name": name, "status": currentStatus, "error": reason},
			name+":failed",
			time.Hour,
		)
	}
	if previousStatus == "error" && currentStatus != "error" {
		sendNotification(
			notificationEventMonitorRecovered,
			"域名监控恢复",
			fmt.Sprintf("%s 已恢复，当前状态：%s", name, currentStatus),
			map[string]any{"name": name, "status": currentStatus},
			name+":recovered",
			time.Hour,
		)
	}
}
