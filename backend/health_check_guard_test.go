package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestValidateHealthCheckProbeSafety 拦住会在配错时放大日志的探测频率组合。
func TestValidateHealthCheckProbeSafety(t *testing.T) {
	cases := []struct {
		name    string
		cfg     ReverseProxyHealthCheckConfig
		wantErr string
	}{
		{
			name: "正常配置",
			cfg:  ReverseProxyHealthCheckConfig{Enabled: true, URI: "/healthz", Interval: "30s", Timeout: "5s"},
		},
		{
			name: "未填间隔与超时（走 Caddy 默认）",
			cfg:  ReverseProxyHealthCheckConfig{Enabled: true, URI: "/healthz"},
		},
		{
			name:    "间隔过密",
			cfg:     ReverseProxyHealthCheckConfig{Enabled: true, URI: "/healthz", Interval: "200ms"},
			wantErr: "间隔不能小于",
		},
		{
			name:    "超时等于间隔",
			cfg:     ReverseProxyHealthCheckConfig{Enabled: true, URI: "/healthz", Interval: "5s", Timeout: "5s"},
			wantErr: "必须小于检查间隔",
		},
		{
			name:    "超时大于间隔",
			cfg:     ReverseProxyHealthCheckConfig{Enabled: true, URI: "/healthz", Interval: "5s", Timeout: "10s"},
			wantErr: "必须小于检查间隔",
		},
		{
			name: "超时略小于间隔",
			cfg:  ReverseProxyHealthCheckConfig{Enabled: true, URI: "/healthz", Interval: "5s", Timeout: "4s"},
		},
	}
	for _, item := range cases {
		err := validateHealthCheckProbeSafety(item.cfg, "测试")
		if item.wantErr == "" {
			if err != nil {
				t.Fatalf("%s: 不应报错，得到 %v", item.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), item.wantErr) {
			t.Fatalf("%s: err = %v, want 包含 %q", item.name, err, item.wantErr)
		}
	}
}

// TestValidateHealthCheckStatus 状态码支持精确值与 2xx 风格区间。
func TestValidateHealthCheckStatus(t *testing.T) {
	valid := []string{"", "200", "204", "503", "2xx", "3xx", "5xx", "2XX"}
	for _, v := range valid {
		if err := validateHealthCheckStatus(v, "测试"); err != nil {
			t.Fatalf("validateHealthCheckStatus(%q) 应通过，得到 %v", v, err)
		}
	}
	invalid := []string{"20", "2000", "abc", "6xx", "x2x", "two hundred"}
	for _, v := range invalid {
		if err := validateHealthCheckStatus(v, "测试"); err == nil {
			t.Fatalf("validateHealthCheckStatus(%q) 应报错", v)
		}
	}
}

// TestValidateReverseProxyAdvancedConfigHealthChecks 端到端校验入口。
func TestValidateReverseProxyAdvancedConfigHealthChecks(t *testing.T) {
	ok := ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
		Enabled: true, URI: "/healthz", Interval: "30s", Timeout: "5s", Status: "2xx",
	}}
	if err := validateReverseProxyAdvancedConfig(ok, "默认反代高级"); err != nil {
		t.Fatalf("合法配置应通过，得到 %v", err)
	}

	bad := []struct {
		name string
		cfg  ReverseProxyAdvancedConfig
		want string
	}{
		{
			name: "未填路径",
			cfg: ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
				Enabled: true,
			}},
			want: "必须填写检查路径",
		},
		{
			name: "路径缺前导斜杠",
			cfg: ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
				Enabled: true, URI: "healthz",
			}},
			want: "必须以 / 开头",
		},
		{
			name: "间隔过密",
			cfg: ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
				Enabled: true, URI: "/healthz", Interval: "100ms",
			}},
			want: "间隔不能小于",
		},
		{
			name: "探测会重叠",
			cfg: ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
				Enabled: true, URI: "/healthz", Interval: "5s", Timeout: "6s",
			}},
			want: "必须小于检查间隔",
		},
		{
			name: "非法状态码",
			cfg: ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
				Enabled: true, URI: "/healthz", Status: "ok",
			}},
			want: "必须是三位状态码",
		},
	}
	for _, item := range bad {
		err := validateReverseProxyAdvancedConfig(item.cfg, "默认反代高级")
		if err == nil || !strings.Contains(err.Error(), item.want) {
			t.Fatalf("%s: err = %v, want 包含 %q", item.name, err, item.want)
		}
	}
}

// TestSiteHealthCheckWarnings 结构化告警：历史遗留配置不走保存校验，必须能被巡检发现。
func TestSiteHealthCheckWarnings(t *testing.T) {
	sites := []Site{
		{
			Name:    "implicit",
			Kind:    "proxy",
			Backend: "http://10.0.0.1:8080",
			ReverseProxy: ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
				Enabled: true, // 未填 URI：Caddy 会用 "/"
			}},
		},
		{
			Name:    "root-path",
			Kind:    "proxy",
			Backend: "http://10.0.0.2:8080",
			ReverseProxy: ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
				Enabled: true, URI: "/",
			}},
		},
		{
			Name:    "dense",
			Kind:    "proxy",
			Backend: "http://10.0.0.3:8080",
			ReverseProxy: ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
				Enabled: true, URI: "/healthz", Interval: "500ms",
			}},
		},
		{
			Name:    "healthy",
			Kind:    "proxy",
			Backend: "http://10.0.0.4:8080",
			ReverseProxy: ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
				Enabled: true, URI: "/healthz", Interval: "30s", Timeout: "5s",
			}},
		},
		{
			Name:     "disabled-site",
			Kind:     "proxy",
			Backend:  "http://10.0.0.5:8080",
			Disabled: true,
			ReverseProxy: ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
				Enabled: true,
			}},
		},
	}

	warnings := siteHealthCheckWarnings(sites)
	joined := strings.Join(warnings, "\n")

	if !strings.Contains(joined, "implicit") || !strings.Contains(joined, "未填检查路径") {
		t.Fatalf("应报告未填路径站点，实际：\n%s", joined)
	}
	if !strings.Contains(joined, "root-path") || !strings.Contains(joined, "根路径") {
		t.Fatalf("应报告根路径站点，实际：\n%s", joined)
	}
	if !strings.Contains(joined, "dense") || !strings.Contains(joined, "间隔不能小于") {
		t.Fatalf("应报告间隔过密站点，实际：\n%s", joined)
	}
	if strings.Contains(joined, "healthy") {
		t.Fatalf("健康配置不应产生告警，实际：\n%s", joined)
	}
	if strings.Contains(joined, "disabled-site") {
		t.Fatalf("已禁用站点不应产生告警，实际：\n%s", joined)
	}
}

// TestBuildHealthCheckAlerts 从运行时日志识别「健康检查持续失败」。
func TestBuildHealthCheckAlerts(t *testing.T) {
	events := []observabilityLogEvent{
		{Message: "status code out of tolerances", Upstream: "10.0.0.1:8080"},
		{Message: "status code out of tolerances", Upstream: "10.0.0.1:8080"},
		{Message: "status code out of tolerances", Upstream: "10.0.0.1:8080"},
		{Message: "status code out of tolerances", Upstream: "10.0.0.2:8080"},
		{Message: "unrelated error", Upstream: "10.0.0.8:8080"},
		{Message: "health check failed: 404", Host: "app.example.com"},
		{Message: "health check failed: 404", Host: "app.example.com"},
		{Message: "health check failed: 404", Host: "app.example.com"},
		{Message: "health check failed: 404", Host: "app.example.com"},
	}

	alerts := buildHealthCheckAlerts(events, healthCheckAlertMinCount)

	byTarget := map[string]observabilityHealthAlert{}
	for _, a := range alerts {
		byTarget[a.Target] = a
	}

	if len(byTarget) != 2 {
		t.Fatalf("应产生 2 条告警（低于阈值的 10.0.0.2 被过滤），实际 %#v", alerts)
	}
	if byTarget["10.0.0.1:8080"].Count != 3 {
		t.Fatalf("10.0.0.1 计数 = %d, want 3", byTarget["10.0.0.1:8080"].Count)
	}
	if byTarget["app.example.com"].Count != 4 {
		t.Fatalf("app.example.com 计数 = %d, want 4", byTarget["app.example.com"].Count)
	}
	if _, ok := byTarget["10.0.0.2:8080"]; ok {
		t.Fatal("低于阈值的上游不应产生告警")
	}
	if _, ok := byTarget["10.0.0.8:8080"]; ok {
		t.Fatal("无关错误不应产生健康检查告警")
	}
	// 失败多的排前面
	if alerts[0].Target != "app.example.com" {
		t.Fatalf("应按失败次数降序，实际第一条 = %s", alerts[0].Target)
	}
	if !strings.Contains(alerts[0].Hint, "测试健康检查") {
		t.Fatalf("提示应引导用户去验证路径，实际 = %q", alerts[0].Hint)
	}
}

// TestIsHealthCheckFailureMessage 特征匹配大小写不敏感。
func TestIsHealthCheckFailureMessage(t *testing.T) {
	yes := []string{
		"status code out of tolerances",
		"STATUS CODE OUT OF TOLERANCES",
		"health check failed",
		"upstream failed health check",
	}
	for _, m := range yes {
		if !isHealthCheckFailureMessage(m) {
			t.Fatalf("isHealthCheckFailureMessage(%q) = false, want true", m)
		}
	}
	no := []string{"", "handled request", "dial tcp: connection refused", "TLS handshake error"}
	for _, m := range no {
		if isHealthCheckFailureMessage(m) {
			t.Fatalf("isHealthCheckFailureMessage(%q) = true, want false", m)
		}
	}
}

// TestObservabilityOverviewIncludesWarnings 观测接口应把配置告警和日志体积带出来。
func TestObservabilityOverviewIncludesWarnings(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	// 造一个可疑的健康检查站点。
	site := Site{
		Name:    "warn-site",
		Kind:    "proxy",
		Domain:  "warn.example.com",
		Backend: "http://10.0.0.9:8080",
		ReverseProxy: ReverseProxyAdvancedConfig{HealthCheck: ReverseProxyHealthCheckConfig{
			Enabled: true, // 未填 URI
		}},
	}
	if err := saveRenderedSite(site, nil); err != nil {
		t.Fatalf("save site error = %v", err)
	}

	// 造一个超过阈值的大日志文件（只写必要的量，靠阈值判断）。
	old := logFileSizeWarnBytes
	defer func() { logFileSizeWarnBytes = old }()
	logFileSizeWarnBytes = 1024 // 测试里把阈值调低
	if err := os.WriteFile(logFile, []byte(strings.Repeat("x\n", 2048)), 0644); err != nil {
		t.Fatalf("write log error = %v", err)
	}

	overview := collectObservabilityOverview(time.Now())

	foundHealth := false
	foundLogSize := false
	for _, w := range overview.Warnings {
		if strings.Contains(w, "warn-site") {
			foundHealth = true
		}
		if strings.Contains(w, "caddy.log 已达") {
			foundLogSize = true
		}
	}
	if !foundHealth {
		t.Fatalf("应包含可疑健康检查告警，实际 %#v", overview.Warnings)
	}
	if !foundLogSize {
		t.Fatalf("应包含日志体积告警，实际 %#v", overview.Warnings)
	}
	if overview.LogFileSizeBytes <= 0 {
		t.Fatalf("应带出日志体积，实际 %d", overview.LogFileSizeBytes)
	}
}
