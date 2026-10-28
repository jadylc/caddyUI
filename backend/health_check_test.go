package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestEffectiveHealthCheckURI 未填路径时必须对齐 Caddy 的默认值 "/"，
// 不能返回空串，否则渲染与检测会不一致。
func TestEffectiveHealthCheckURI(t *testing.T) {
	cases := []struct {
		name string
		cfg  ReverseProxyHealthCheckConfig
		want string
	}{
		{"未启用", ReverseProxyHealthCheckConfig{Enabled: false, URI: "/health"}, ""},
		{"启用且留空走默认", ReverseProxyHealthCheckConfig{Enabled: true, URI: ""}, "/"},
		{"启用且留空空格", ReverseProxyHealthCheckConfig{Enabled: true, URI: "   "}, "/"},
		{"启用且指定路径", ReverseProxyHealthCheckConfig{Enabled: true, URI: "/healthz"}, "/healthz"},
	}
	for _, item := range cases {
		if got := effectiveHealthCheckURI(item.cfg); got != item.want {
			t.Fatalf("%s: effectiveHealthCheckURI = %q, want %q", item.name, got, item.want)
		}
	}
}

// TestActiveHealthCheckEnabledMatchesRenderer 渲染与检测必须一致：
// 未填路径的历史配置同样算「已启用主动健康检查」。
func TestActiveHealthCheckEnabledMatchesRenderer(t *testing.T) {
	implicit := ReverseProxyHealthCheckConfig{Enabled: true, URI: ""}
	if !reverseProxyActiveHealthCheckEnabled(implicit) {
		t.Fatal("未填路径但已启用的健康检查必须被识别为启用（Caddy 会探测 /）")
	}

	var b strings.Builder
	writeReverseProxyHealthCheckDirectives(&b, implicit, "    ")
	out := b.String()
	if !strings.Contains(out, "health_uri /\n") {
		t.Fatalf("启用但留空时必须显式写出默认路径，实际渲染：\n%s", out)
	}

	// 未启用时不应写出任何健康检查指令。
	b.Reset()
	writeReverseProxyHealthCheckDirectives(&b, ReverseProxyHealthCheckConfig{Enabled: false}, "    ")
	if strings.Contains(b.String(), "health_uri") {
		t.Fatalf("未启用时不应渲染 health_uri：\n%s", b.String())
	}

	// 指定路径时原样写出。
	b.Reset()
	writeReverseProxyHealthCheckDirectives(&b, ReverseProxyHealthCheckConfig{Enabled: true, URI: "/healthz"}, "    ")
	if !strings.Contains(b.String(), "health_uri /healthz\n") {
		t.Fatalf("指定路径应原样渲染：\n%s", b.String())
	}
}

// TestHealthStatusMatches 状态码判定：默认 2xx，支持精确码与区间。
func TestHealthStatusMatches(t *testing.T) {
	cases := []struct {
		status   int
		expected string
		want     bool
	}{
		{200, "", true},
		{204, "", true},
		{301, "", false},
		{404, "", false},
		{500, "", false},
		{200, "200", true},
		{201, "200", false},
		{204, "204", true},
		// 显式把 404 配成健康状态时按 Caddy 语义算健康（罕见但合法配置）。
		{404, "404", true},
		{404, "200", false},
		{200, "2xx", true},
		{299, "2xx", true},
		{301, "2xx", false},
		{503, "5xx", true},
		{200, "bogus", false},
	}
	for _, item := range cases {
		if got := healthStatusMatches(item.status, item.expected); got != item.want {
			t.Fatalf("healthStatusMatches(%d, %q) = %v, want %v",
				item.status, item.expected, got, item.want)
		}
	}
}

// TestHealthCheckProbeDetectsWrongPath 探测接口应能发现「路径返回非 2xx」这种配错。
func TestHealthCheckProbeDetectsWrongPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// 其它路径返回 404——正是配错健康检查路径时的典型现象。
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	post := func(uri, status string) map[string]any {
		body := strings.NewReader(`{"upstream":"` + upstream.URL + `","uri":"` + uri + `","status":"` + status + `","timeout":"2s"}`)
		req := httptest.NewRequest(http.MethodPost, "/caddy/proxy-hosts/health-check/test", body)
		rec := httptest.NewRecorder()
		npmHealthCheckTestHandler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		out := map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("unmarshal error = %v", err)
		}
		return out
	}

	// 正确路径 → 通过。
	ok := post("/healthz", "")
	if ok["ok"] != true {
		t.Fatalf("正确路径应通过，结果 = %#v", ok)
	}
	if got := int(ok["status"].(float64)); got != 200 {
		t.Fatalf("status = %d, want 200", got)
	}

	// 错误路径 → 不通过，并给出可读提示。
	bad := post("/health", "")
	if bad["ok"] != false {
		t.Fatalf("错误路径不应通过，结果 = %#v", bad)
	}
	if got := int(bad["status"].(float64)); got != 404 {
		t.Fatalf("status = %d, want 404", got)
	}
	message, _ := bad["message"].(string)
	if !strings.Contains(message, "404") || !strings.Contains(message, "错误日志") {
		t.Fatalf("提示信息应说明状态码与后果，实际 = %q", message)
	}

	// 期望状态码不匹配 → 不通过。
	mismatch := post("/healthz", "204")
	if mismatch["ok"] != false {
		t.Fatalf("状态码不匹配时不应通过，结果 = %#v", mismatch)
	}
}

// TestHealthCheckProbeUsesDefaultURI 未填路径时探测 "/"，与 Caddy 行为一致。
func TestHealthCheckProbeUsesDefaultURI(t *testing.T) {
	var seenPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	body := strings.NewReader(`{"upstream":"` + upstream.URL + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/caddy/proxy-hosts/health-check/test", body)
	rec := httptest.NewRecorder()
	npmHealthCheckTestHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if seenPath != "/" {
		t.Fatalf("探测路径 = %q, want / (Caddy 默认)", seenPath)
	}
}

// TestHealthCheckProbeRejectsBadInput 输入校验。
func TestHealthCheckProbeRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"缺少上游", `{"uri":"/health"}`},
		{"路径不以斜杠开头", `{"upstream":"http://127.0.0.1:1","uri":"health"}`},
		{"路径含控制字符", `{"upstream":"http://127.0.0.1:1","uri":"/he\nalth"}`},
	}
	for _, item := range cases {
		req := httptest.NewRequest(http.MethodPost, "/caddy/proxy-hosts/health-check/test", strings.NewReader(item.body))
		rec := httptest.NewRecorder()
		npmHealthCheckTestHandler(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", item.name, rec.Code)
		}
	}
}

// TestHealthCheckProbeReportsUnreachable 上游不可达时应返回可读错误而不是 5xx。
func TestHealthCheckProbeReportsUnreachable(t *testing.T) {
	// 127.0.0.1:1 基本不会有人监听。
	body := strings.NewReader(`{"upstream":"http://127.0.0.1:1","uri":"/health","timeout":"500ms"}`)
	req := httptest.NewRequest(http.MethodPost, "/caddy/proxy-hosts/health-check/test", body)
	rec := httptest.NewRecorder()
	npmHealthCheckTestHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with error payload", rec.Code)
	}
	out := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if out["ok"] != false {
		t.Fatalf("不可达时 ok 应为 false，结果 = %#v", out)
	}
	if _, hasErr := out["error"]; !hasErr {
		t.Fatalf("应返回 error 字段，结果 = %#v", out)
	}
}
