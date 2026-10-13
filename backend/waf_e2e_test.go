package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestWAFProxyHostHTTPEndToEnd 走真实前端保存入口的集成链路：
// POST /caddy/proxy-hosts（启用 WAF）→ 校验渲染 conf 含 coraza_waf、meta 落盘、
// global.d/waf.conf 生成 → PUT /caddy/proxy-hosts/{id} 关闭 WAF → waf.conf 被删除 →
// GET /caddy/waf/audit 无日志时返回空列表。
func TestWAFProxyHostHTTPEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	withTempRuntimePaths(t, tmp)

	oldWAFConf := wafConfPath
	wafConfPath = filepath.Join(tmp, "global.d", "waf.conf")
	defer func() { wafConfPath = oldWAFConf }()
	if err := os.MkdirAll(filepath.Dir(wafConfPath), 0755); err != nil {
		t.Fatalf("MkdirAll wafConf dir error = %v", err)
	}

	oldAuditLog := wafAuditLogPath
	wafAuditLogPath = filepath.Join(tmp, "waf-audit.json")
	defer func() { wafAuditLogPath = oldAuditLog }()

	stubCaddyValidate(t)

	reloadCalled := false
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/load" {
			t.Fatalf("admin path = %q, want /load", r.URL.Path)
		}
		reloadCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer adminServer.Close()
	caddyAdmin = adminServer.URL

	domain := "waf-e2e.local"
	base := npmProxyHost{
		DomainNames:   []string{domain},
		ListenPort:    80,
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		SSLForced:     false,
		Enabled:       true,
		Meta:          map[string]any{"challenge_pref": "http"},
	}

	// 1) 创建启用 WAF 的 proxy host
	create := base
	create.WAF = WAFConfig{Enabled: true, Mode: "detection_only"}
	body, err := json.Marshal(create)
	if err != nil {
		t.Fatalf("json.Marshal create error = %v", err)
	}
	rr := httptest.NewRecorder()
	npmProxyHostsHandler(rr, httptest.NewRequest(http.MethodPost, "/caddy/proxy-hosts", bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, body = %q", rr.Code, rr.Body.String())
	}
	if !reloadCalled {
		t.Fatalf("caddy reload was not called")
	}

	var created npmProxyHost
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal created error = %v", err)
	}
	if !created.WAF.Enabled || created.WAF.Mode != "detection_only" {
		t.Fatalf("created waf = %#v", created.WAF)
	}

	// 站点 conf 渲染必须包含 coraza_waf
	confBytes, err := os.ReadFile(filepath.Join(sitesDir, domain+".conf"))
	if err != nil {
		t.Fatalf("read rendered conf error = %v", err)
	}
	conf := string(confBytes)
	for _, want := range []string{
		"coraza_waf {",
		"load_owasp_crs",
		"directives `",
		"Include @coraza.conf-recommended",
		"Include @crs-setup.conf.example",
		"Include @owasp_crs/*.conf",
		"SecRuleEngine DetectionOnly",
		"SecAuditLog " + wafAuditLogPath,
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("rendered conf = %q, want %q", conf, want)
		}
	}
	assertSecInsideDirectives(t, conf, "SecRuleEngine DetectionOnly", "SecAuditLogType JSON")

	// meta 落盘必须带 waf 字段
	var saved Site
	if err := loadJSONFile(filepath.Join(sitesDir, domain+metaSuffix), &saved); err != nil {
		t.Fatalf("load saved site error = %v", err)
	}
	if !saved.WAF.Enabled || saved.WAF.Mode != "detection_only" {
		t.Fatalf("saved site waf = %#v", saved.WAF)
	}

	// global.d/waf.conf 必须生成
	wafConfBytes, err := os.ReadFile(wafConfPath)
	if err != nil {
		t.Fatalf("read waf.conf error = %v", err)
	}
	if got := string(wafConfBytes); got != "order coraza_waf first\n" {
		t.Fatalf("waf.conf = %q, want %q", got, "order coraza_waf first\n")
	}

	// 2) PUT 关闭 WAF → waf.conf 删除
	disable := create
	disable.WAF = WAFConfig{Enabled: false, Mode: "on"}
	body2, err := json.Marshal(disable)
	if err != nil {
		t.Fatalf("json.Marshal disable error = %v", err)
	}
	rr2 := httptest.NewRecorder()
	npmProxyHostItemHandler(rr2, httptest.NewRequest(http.MethodPut, "/caddy/proxy-hosts/"+strconv.Itoa(created.ID), bytes.NewReader(body2)))
	if rr2.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %q", rr2.Code, rr2.Body.String())
	}
	if _, err := os.Stat(wafConfPath); !os.IsNotExist(err) {
		t.Fatalf("waf.conf should be removed after disabling WAF, stat err = %v", err)
	}

	// 3) 审计接口无日志 → 200 空列表
	auditRR := httptest.NewRecorder()
	wafAuditHandler(auditRR, httptest.NewRequest(http.MethodGet, "/caddy/waf/audit", nil))
	if auditRR.Code != http.StatusOK {
		t.Fatalf("audit status = %d, body = %q", auditRR.Code, auditRR.Body.String())
	}
	var auditResp struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(auditRR.Body.Bytes(), &auditResp); err != nil {
		t.Fatalf("json.Unmarshal audit error = %v", err)
	}
	if auditResp.Total != 0 {
		t.Fatalf("audit total = %d, want 0", auditResp.Total)
	}
}
