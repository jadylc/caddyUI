package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 模拟一行 coraza v3 JSON 审计日志（结构与 coraza JSON 审计日志实际输出一致：
// messages 位于根级，transaction.timestamp，headers key 小写且值为数组，
// 命中详情在 messages[].error_message 富文本中）
const sampleWafAuditLine = `{"transaction":{"timestamp":"2026/10/13 10:00:00","id":"tx-001","client_ip":"1.2.3.4","server_id":"example.com","request":{"method":"GET","uri":"/etc/passwd","headers":{"host":["attack.example.com"]}},"producer":{"rule_engine":"DetectionOnly"},"is_interrupted":false},"messages":[{"actionset":"","message":"","error_message":"[client \"1.2.3.4\"] Coraza: Warning. Inbound Anomaly Score Exceeded (Total Score: 20) [file \"@owasp_crs/REQUEST-949-BLOCKING-EVALUATION.conf\"] [id \"949110\"] [rev \"\"] [msg \"Inbound Anomaly Score Exceeded (Total Score: 20)\"] [severity \"critical\"] [ver \"OWASP_CRS/4.25.0\"] [tag \"OWASP_CRS\"]","data":null}]}`

// 拦截场景：is_interrupted=true，error_message 含 Access denied
const sampleWafAuditDeniedLine = `{"transaction":{"timestamp":"2026/10/13 10:05:00","id":"tx-002","client_ip":"1.2.3.5","server_id":"example.com","request":{"method":"POST","uri":"/login","headers":{"host":["attack.example.com"]}},"producer":{"rule_engine":"coraza"},"is_interrupted":true},"messages":[{"actionset":"deny","message":"","error_message":"[client \"1.2.3.5\"] Coraza: Access denied with code 403 (phase 2). Match of \"pm\" against \"REQUEST_HEADERS:User-Agent\" [file \"@owasp_crs/REQUEST-913-SCANNER-DETECTION.conf\"] [id \"913100\"] [rev \"\"] [msg \"Found User-Agent associated with security scanner\"] [severity \"warning\"] [ver \"OWASP_CRS/4.25.0\"]","data":null}]}`

func writeSampleWafAudit(t *testing.T, dir string, lines []string) {
	t.Helper()
	path := filepath.Join(dir, "audit.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	old := wafAuditLogPath
	wafAuditLogPath = path
	t.Cleanup(func() { wafAuditLogPath = old })
}

func TestWafAuditHandlerParsing(t *testing.T) {
	tmp := t.TempDir()
	withTempRuntimePaths(t, tmp)
	writeSampleWafAudit(t, tmp, []string{sampleWafAuditLine, sampleWafAuditDeniedLine})

	req := httptest.NewRequest(http.MethodGet, "/caddy/waf/audit", nil)
	rr := httptest.NewRecorder()
	wafAuditHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp struct {
		Total int             `json:"total"`
		Items []wafAuditEntry `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error = %v, body = %q", err, rr.Body.String())
	}
	if resp.Total != 2 || len(resp.Items) != 2 {
		t.Fatalf("total=%d items=%d, want 2/2", resp.Total, len(resp.Items))
	}
	// 按 transaction_id 索引，避免依赖返回顺序
	byID := map[string]wafAuditEntry{}
	for _, it := range resp.Items {
		byID[it.TransactionID] = it
	}
	// 检测模式（未拦截）
	wantFields := map[string]map[string]string{
		"tx-001": {
			"Timestamp":  "2026/10/13 10:00:00",
			"Host":       "attack.example.com",
			"URI":        "/etc/passwd",
			"Method":     "GET",
			"RuleID":     "949110",
			"Message":    "Inbound Anomaly Score Exceeded (Total Score: 20)",
			"Severity":   "critical",
			"Action":     "observe",
			"RemoteAddr": "1.2.3.4",
		},
		// 拦截模式
		"tx-002": {
			"Timestamp":  "2026/10/13 10:05:00",
			"Host":       "attack.example.com",
			"URI":        "/login",
			"Method":     "POST",
			"RuleID":     "913100",
			"Message":    "Found User-Agent associated with security scanner",
			"Severity":   "warning",
			"Action":     "deny",
			"RemoteAddr": "1.2.3.5",
		},
	}
	for txID, expects := range wantFields {
		item, ok := byID[txID]
		if !ok {
			t.Fatalf("missing transaction %s in items %+v", txID, resp.Items)
		}
		for field, want := range expects {
			var got string
			switch field {
			case "TransactionID":
				got = item.TransactionID
			case "Timestamp":
				got = item.Timestamp
			case "Host":
				got = item.Host
			case "URI":
				got = item.URI
			case "Method":
				got = item.Method
			case "RuleID":
				got = item.RuleID
			case "Severity":
				got = item.Severity
			case "Action":
				got = item.Action
			case "RemoteAddr":
				got = item.RemoteAddr
			case "Message":
				got = item.Message
			}
			if got != want {
				t.Fatalf("%s[%s] = %q, want %q", txID, field, got, want)
			}
		}
	}
}

func TestWafAuditHandlerEmptyAndFilter(t *testing.T) {
	tmp := t.TempDir()
	withTempRuntimePaths(t, tmp)
	writeSampleWafAudit(t, tmp, []string{sampleWafAuditLine, sampleWafAuditLine})

	// 无匹配 host -> 空列表
	req := httptest.NewRequest(http.MethodGet, "/caddy/waf/audit?host=nomatch", nil)
	rr := httptest.NewRecorder()
	wafAuditHandler(rr, req)
	var resp struct {
		Total int             `json:"total"`
		Items []wafAuditEntry `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if resp.Total != 0 || len(resp.Items) != 0 {
		t.Fatalf("filter total=%d items=%d, want 0/0", resp.Total, len(resp.Items))
	}

	// limit=1 只返回 1 条
	req = httptest.NewRequest(http.MethodGet, "/caddy/waf/audit?limit=1", nil)
	rr = httptest.NewRecorder()
	wafAuditHandler(rr, req)
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if resp.Total != 2 || len(resp.Items) != 1 {
		t.Fatalf("limit total=%d items=%d, want 2/1", resp.Total, len(resp.Items))
	}
}

func TestWafAuditHandlerMethodNotAllowed(t *testing.T) {
	tmp := t.TempDir()
	withTempRuntimePaths(t, tmp)
	req := httptest.NewRequest(http.MethodPost, "/caddy/waf/audit", nil)
	rr := httptest.NewRecorder()
	wafAuditHandler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}
