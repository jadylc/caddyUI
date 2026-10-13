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

// 模拟一行 coraza 审计 JSON（结构与 coraza JSON 审计日志格式一致）
const sampleWafAuditLine = `{"transaction":{"id":"tx-001","time_stamp":"2026-10-13T10:00:00Z","client_ip":"1.2.3.4","request":{"method":"GET","uri":"/etc/passwd","headers":{"Host":"attack.example.com"}}},"audit_data":{"messages":[{"message":"Inbound Anomaly Score Exceeded","details":{"ruleId":"949110","severity":"2","action":"deny"}}]}}`

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
	writeSampleWafAudit(t, tmp, []string{sampleWafAuditLine})

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
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("total=%d items=%d, want 1/1", resp.Total, len(resp.Items))
	}
	item := resp.Items[0]
	for field, want := range map[string]string{
		"TransactionID": "tx-001",
		"Timestamp":     "2026-10-13T10:00:00Z",
		"Host":          "attack.example.com",
		"URI":           "/etc/passwd",
		"Method":        "GET",
		"RuleID":        "949110",
		"Severity":      "2",
		"Action":        "deny",
		"RemoteAddr":    "1.2.3.4",
	} {
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
		}
		if got != want {
			t.Fatalf("%s = %q, want %q", field, got, want)
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
