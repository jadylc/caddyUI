package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWAFRenderDisabledByteIdentical(t *testing.T) {
	tmp := t.TempDir()
	withTempRuntimePaths(t, tmp)
	base := Site{Name: "plain", Domain: "plain.example.com", Backend: "http://127.0.0.1:8080", NoTLS: true}
	conf1, err := renderSite(base, nil)
	if err != nil {
		t.Fatalf("renderSite base error = %v", err)
	}
	withWAF := base
	withWAF.WAF = WAFConfig{Enabled: false, Mode: "detection_only"}
	conf2, err := renderSite(withWAF, nil)
	if err != nil {
		t.Fatalf("renderSite with-waf error = %v", err)
	}
	if conf1 != conf2 {
		t.Fatalf("WAF disabled output must be byte-identical\nbase: %q\nwith: %q", conf1, conf2)
	}
	if strings.Contains(conf2, "coraza_waf") {
		t.Fatalf("disabled WAF must not render coraza_waf: %q", conf2)
	}
}

func TestWAFRenderDetectionOnly(t *testing.T) {
	tmp := t.TempDir()
	withTempRuntimePaths(t, tmp)
	s := Site{Name: "waf-det", Domain: "waf.example.com", Backend: "http://127.0.0.1:8080", NoTLS: true,
		WAF: WAFConfig{Enabled: true, Mode: "detection_only"}}
	conf, err := renderSite(s, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	for _, want := range []string{
		"coraza_waf {",
		"load_owasp_crs",
		"directives `",
		"Include @coraza.conf-recommended",
		"Include @crs-setup.conf.example",
		"Include @owasp_crs/*.conf",
		"SecRuleEngine DetectionOnly",
		"SecAuditEngine RelevantOnly",
		"SecAuditLog " + wafAuditLogPath,
		"SecAuditLogType JSON",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
	assertSecInsideDirectives(t, conf, "SecRuleEngine DetectionOnly", "SecAuditLogType JSON")
}

// assertSecInsideDirectives 校验 ModSecurity 风格指令出现在 directives 反引号块内，
// 防止回退到被 coraza-caddy 拒绝的顶层裸 key。
func assertSecInsideDirectives(t *testing.T, conf string, keys ...string) {
	t.Helper()
	dpos := strings.Index(conf, "directives `")
	if dpos < 0 {
		t.Fatalf("conf missing directives block: %q", conf)
	}
	for _, key := range keys {
		if kpos := strings.Index(conf, key); kpos < dpos {
			t.Fatalf("key %q must be inside directives block: %q", key, conf)
		}
	}
}

func TestWAFRenderOn(t *testing.T) {
	tmp := t.TempDir()
	withTempRuntimePaths(t, tmp)
	s := Site{Name: "waf-on", Domain: "waf.example.com", Backend: "http://127.0.0.1:8080", NoTLS: true,
		WAF: WAFConfig{Enabled: true, Mode: "on"}}
	conf, err := renderSite(s, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	if !strings.Contains(conf, "SecRuleEngine On") {
		t.Fatalf("renderSite = %q, want SecRuleEngine On", conf)
	}
	assertSecInsideDirectives(t, conf, "SecRuleEngine On")
}

func TestWAFRenderStaticSite(t *testing.T) {
	tmp := t.TempDir()
	withTempRuntimePaths(t, tmp)
	s := Site{Name: "waf-static", Kind: "static", Domain: "static.example.com", RootPath: "/srv/www", NoTLS: true,
		WAF: WAFConfig{Enabled: true, Mode: "detection_only"}}
	conf, err := renderSite(s, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	if !strings.Contains(conf, "coraza_waf {") {
		t.Fatalf("static site should render coraza_waf: %q", conf)
	}
}

func TestWAFGlobalConfSync(t *testing.T) {
	tmp := t.TempDir()
	withTempRuntimePaths(t, tmp)
	old := wafConfPath
	wafConfPath = filepath.Join(tmp, "global.d", "waf.conf")
	t.Cleanup(func() { wafConfPath = old })
	if err := os.MkdirAll(filepath.Dir(wafConfPath), 0755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	// 无启用站点 -> 文件不存在
	if err := syncWAFGlobalConf(); err != nil {
		t.Fatalf("syncWAFGlobalConf error = %v", err)
	}
	if _, err := os.Stat(wafConfPath); !os.IsNotExist(err) {
		t.Fatalf("waf.conf should be removed, stat err = %v", err)
	}

	// 写入一个启用 WAF 的站点 -> 生成 order 行
	s := Site{Name: "waf1", Domain: "waf1.example.com", Backend: "http://127.0.0.1:8080",
		WAF: WAFConfig{Enabled: true, Mode: "detection_only"}}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("marshalSiteMeta error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sitesDir, "waf1"+metaSuffix), data, 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := syncWAFGlobalConf(); err != nil {
		t.Fatalf("syncWAFGlobalConf error = %v", err)
	}
	got, err := os.ReadFile(wafConfPath)
	if err != nil {
		t.Fatalf("ReadFile waf.conf error = %v", err)
	}
	if string(got) != "order coraza_waf first\n" {
		t.Fatalf("waf.conf = %q", string(got))
	}

	// 站点改为禁用 -> 文件被删除
	s2 := s
	s2.WAF = WAFConfig{Enabled: false}
	data2, err := json.MarshalIndent(s2, "", "  ")
	if err != nil {
		t.Fatalf("marshalSiteMeta error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sitesDir, "waf1"+metaSuffix), data2, 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := syncWAFGlobalConf(); err != nil {
		t.Fatalf("syncWAFGlobalConf error = %v", err)
	}
	if _, err := os.Stat(wafConfPath); !os.IsNotExist(err) {
		t.Fatalf("waf.conf should be removed after disable, stat err = %v", err)
	}
}
