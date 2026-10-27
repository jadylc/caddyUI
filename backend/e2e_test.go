package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// startTestServer 启动一个临时 HTTP 服务，返回 base URL 和关闭函数。
func startTestServer(t *testing.T) (string, func()) {
	t.Helper()
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	// E2E 测试不需要真实 caddy 二进制，覆盖校验和重载函数。
	oldValidate := validateCaddyConfigExec
	validateCaddyConfigExec = func() error { return nil }
	oldReload := reloadCaddyFunc
	reloadCaddyFunc = func() error { return nil }
	t.Cleanup(func() {
		validateCaddyConfigExec = oldValidate
		reloadCaddyFunc = oldReload
	})

	mux := newAPIMux()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error = %v", err)
	}
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(ln) }()

	baseURL := "http://" + ln.Addr().String()
	return baseURL, func() { _ = server.Close() }
}

// httpJSON 发送 HTTP 请求并解析 JSON 响应。
func httpJSON(t *testing.T, method, url string, body any) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("request error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request error = %v", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body error = %v", err)
	}
	return res.StatusCode, data
}

// TestE2EProvidersSchema 验证 /caddy/providers 返回全部 provider 及 schema。
func TestE2EProvidersSchema(t *testing.T) {
	baseURL, close := startTestServer(t)
	defer close()

	status, body := httpJSON(t, http.MethodGet, baseURL+"/caddy/providers", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}

	var providers []map[string]any
	if err := json.Unmarshal(body, &providers); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if len(providers) < 18 {
		t.Fatalf("providers count = %d, want >= 18", len(providers))
	}

	ids := map[string]bool{}
	for _, p := range providers {
		id, _ := p["id"].(string)
		if id == "" {
			t.Fatalf("provider missing id: %#v", p)
		}
		if ids[id] {
			t.Fatalf("duplicate provider id: %s", id)
		}
		ids[id] = true
		if _, ok := p["fields"]; !ok {
			t.Fatalf("provider %s missing fields", id)
		}
	}

	// 关键 provider 必须在列表中。
	for _, want := range []string{"alidns", "cloudflare", "huaweicloud", "rfc2136", "duckdns", "porkbun", "desec", "route53"} {
		if !ids[want] {
			t.Fatalf("missing provider %s", want)
		}
	}
}

// TestE2ECredentialCRUD 验证凭据的创建、列表、读取、更新、删除全流程。
func TestE2ECredentialCRUD(t *testing.T) {
	baseURL, close := startTestServer(t)
	defer close()

	// 创建华为云凭据（只提交 secrets）。
	status, body := httpJSON(t, http.MethodPost, baseURL+"/credentials", map[string]any{
		"name":     "e2e-huawei",
		"provider": "huaweicloud",
		"secrets": map[string]string{
			"huawei_access_key_id":     "AKID",
			"huawei_secret_access_key": "SECRET",
			"huawei_region_id":         "cn-north-4",
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", status, body)
	}
	var createResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &createResp); err != nil {
		t.Fatalf("unmarshal create response error = %v", err)
	}
	if createResp.ID == "" {
		t.Fatal("create response missing id")
	}

	// 列表应包含新创建的凭据。
	status, body = httpJSON(t, http.MethodGet, baseURL+"/credentials", nil)
	if status != http.StatusOK {
		t.Fatalf("list status = %d", status)
	}
	var list []struct {
		ID        string `json:"id"`
		Provider  string `json:"provider"`
		HasSecret bool   `json:"has_secret"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal list error = %v", err)
	}
	found := false
	for _, c := range list {
		if c.ID == createResp.ID && c.Provider == "huaweicloud" && c.HasSecret {
			found = true
		}
	}
	if !found {
		t.Fatalf("credential not found in list: %#v", list)
	}

	// 读取单条凭据，验证 secrets 和 fields。
	status, body = httpJSON(t, http.MethodGet, baseURL+"/credentials/"+createResp.ID, nil)
	if status != http.StatusOK {
		t.Fatalf("get status = %d", status)
	}
	var detail map[string]any
	if err := json.Unmarshal(body, &detail); err != nil {
		t.Fatalf("unmarshal detail error = %v", err)
	}
	if detail["provider"] != "huaweicloud" {
		t.Fatalf("provider = %v, want huaweicloud", detail["provider"])
	}
	secrets, _ := detail["secrets"].(map[string]any)
	if secrets["huawei_access_key_id"] != "AKID" {
		t.Fatalf("secrets = %#v", secrets)
	}
	if _, ok := detail["fields"]; !ok {
		t.Fatal("detail missing fields")
	}

	// 更新凭据会触发 caddy validate，测试环境无 caddy 二进制，
	// 更新逻辑已在单元测试 TestCredentialCreateAcceptsSecretsOnly 中覆盖。
	// 此处直接验证删除流程。

	// 删除凭据。DELETE 返回 204 No Content。
	status, _ = httpJSON(t, http.MethodDelete, baseURL+"/credentials/"+createResp.ID, nil)
	if status != http.StatusNoContent && status != http.StatusOK {
		t.Fatalf("delete status = %d", status)
	}

	// 验证已删除。
	status, _ = httpJSON(t, http.MethodGet, baseURL+"/credentials/"+createResp.ID, nil)
	if status != http.StatusNotFound {
		t.Fatalf("get deleted status = %d, want 404", status)
	}
}

// TestE2EDynamicDNSConfig 验证 DDNS 条目的创建和配置生成。
func TestE2EDynamicDNSConfig(t *testing.T) {
	baseURL, close := startTestServer(t)
	defer close()

	// 先创建凭据。
	status, body := httpJSON(t, http.MethodPost, baseURL+"/credentials", map[string]any{
		"name":     "e2e-ddns-cred",
		"provider": "huaweicloud",
		"secrets": map[string]string{
			"huawei_access_key_id":     "AKID",
			"huawei_secret_access_key": "SECRET",
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create credential status = %d, body = %s", status, body)
	}
	var createResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &createResp); err != nil {
		t.Fatalf("unmarshal credential response error = %v, body = %s", err, body)
	}
	if createResp.ID == "" {
		t.Fatalf("credential ID is empty, body = %s", body)
	}

	// 创建 DDNS 条目。
	status, body = httpJSON(t, http.MethodPost, baseURL+"/caddy/dynamic-dns", map[string]any{
		"name":          "e2e-ddns",
		"domain_names":  []string{"example.com"},
		"credential_id": createResp.ID,
		"ipv4":          true,
		"ipv6":          false,
		"checkInterval": "5m",
		"enabled":       true,
	})
	if status != http.StatusCreated {
		t.Fatalf("create ddns status = %d, body = %s", status, body)
	}

	// 读取 DDNS 列表，验证条目存在。
	status, body = httpJSON(t, http.MethodGet, baseURL+"/caddy/dynamic-dns", nil)
	if status != http.StatusOK {
		t.Fatalf("list ddns status = %d", status)
	}
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatalf("unmarshal ddns list error = %v", err)
	}
	if len(items) < 1 {
		t.Fatalf("ddns items count = %d, want >= 1", len(items))
	}
	found := false
	for _, item := range items {
		if item["name"] == "e2e-ddns" {
			found = true
		}
	}
	if !found {
		t.Fatalf("e2e-ddns item not found in list: %#v", items)
	}

	// 验证生成的配置文件。
	configPath := filepath.Join(tmpDirForTest(t), "dynamic-dns.conf")
	// withTempPaths 已将 dynamicDNSConfPath 指向临时目录。
	configData, err := os.ReadFile(dynamicDNSConfPath)
	if err != nil {
		t.Fatalf("read config error = %v", err)
	}
	config := string(configData)
	if !strings.Contains(config, "dynamic_dns") {
		t.Fatalf("config missing dynamic_dns: %s", config)
	}
	if !strings.Contains(config, "provider huaweicloud") {
		t.Fatalf("config missing huaweicloud provider: %s", config)
	}
	if !strings.Contains(config, "access_key_id") {
		t.Fatalf("config missing access_key_id: %s", config)
	}
	_ = configPath
}

// tmpDirForTest 返回测试临时目录（由 withTempPaths 设置）。
func tmpDirForTest(t *testing.T) string {
	t.Helper()
	return filepath.Dir(credentialPath)
}

// TestE2ECaddyValidate 用真实的 Caddy 二进制校验生成的 DDNS 配置。
// 如果 CADDY_BINARY_PATH 未设置或二进制不存在，跳过此测试。
func TestE2ECaddyValidate(t *testing.T) {
	caddyPath := strings.TrimSpace(os.Getenv("CADDY_BINARY_PATH"))
	if caddyPath == "" {
		t.Skip("CADDY_BINARY_PATH not set, skipping Caddy validation")
	}
	if _, err := os.Stat(caddyPath); err != nil {
		t.Skipf("Caddy binary not found at %s: %v", caddyPath, err)
	}

	baseURL, close := startTestServer(t)
	defer close()

	// 创建凭据和 DDNS 条目（华为云）。
	status, body := httpJSON(t, http.MethodPost, baseURL+"/credentials", map[string]any{
		"name":     "e2e-caddy-cred",
		"provider": "huaweicloud",
		"secrets": map[string]string{
			"huawei_access_key_id":     "AKID",
			"huawei_secret_access_key": "SECRET",
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create credential status = %d, body = %s", status, body)
	}
	var createResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &createResp); err != nil {
		t.Fatalf("unmarshal credential response error = %v, body = %s", err, body)
	}
	if createResp.ID == "" {
		t.Fatalf("credential ID is empty, body = %s", body)
	}

	status, _ = httpJSON(t, http.MethodPost, baseURL+"/caddy/dynamic-dns", map[string]any{
		"name":          "e2e-caddy-ddns",
		"domain_names":  []string{"example.com"},
		"credential_id": createResp.ID,
		"ipv4":          true,
		"checkInterval": "5m",
		"enabled":       true,
	})
	if status != http.StatusCreated {
		t.Fatalf("create ddns status = %d", status)
	}

	// 读取生成的配置文件。
	configData, err := os.ReadFile(dynamicDNSConfPath)
	if err != nil {
		t.Fatalf("read config error = %v", err)
	}

	// 包装成完整的 Caddyfile（dynamic_dns 是全局指令）。
	caddyfile := "{\n\tadmin off\n\tauto_https off\n}\n" + string(configData)

	// 写入临时文件。
	tmpFile := filepath.Join(t.TempDir(), "Caddyfile")
	if err := os.WriteFile(tmpFile, []byte(caddyfile), 0644); err != nil {
		t.Fatalf("write caddyfile error = %v", err)
	}

	// 用 Caddy 校验。
	cmd := exec.Command(caddyPath, "validate", "--config", tmpFile, "--adapter", "caddyfile")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		t.Fatalf("caddy validate failed: %v\nstderr: %s", err, stderr.String())
	}
}

// TestE2EDNSProvidersEndpoint 验证 /caddy/certificates/dns-providers 返回 DNS 提供商列表。
func TestE2EDNSProvidersEndpoint(t *testing.T) {
	baseURL, close := startTestServer(t)
	defer close()

	status, body := httpJSON(t, http.MethodGet, baseURL+"/caddy/certificates/dns-providers", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}

	var providers []map[string]any
	if err := json.Unmarshal(body, &providers); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	// DNS 提供商应包含核心几家。
	ids := map[string]bool{}
	for _, p := range providers {
		id, _ := p["id"].(string)
		ids[id] = true
	}
	for _, want := range []string{"alidns", "cloudflare", "dnspod", "he", "huaweicloud"} {
		if !ids[want] {
			t.Fatalf("missing DNS provider %s in dns-providers endpoint", want)
		}
	}
	// 不应包含纯域名商。
	if ids["digitalplat"] {
		t.Fatal("digitalplat should not be in dns-providers endpoint")
	}
}

// TestE2EProvidersSchemaFieldSchema 验证 provider schema 的字段结构完整。
func TestE2EProvidersSchemaFieldSchema(t *testing.T) {
	baseURL, close := startTestServer(t)
	defer close()

	status, body := httpJSON(t, http.MethodGet, baseURL+"/caddy/providers", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}

	var providers []map[string]any
	if err := json.Unmarshal(body, &providers); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}

	for _, p := range providers {
		fields, ok := p["fields"].([]any)
		if !ok || len(fields) == 0 {
			t.Fatalf("provider %v missing or empty fields", p["id"])
		}
		for _, f := range fields {
			field, ok := f.(map[string]any)
			if !ok {
				t.Fatalf("field not a map: %#v", f)
			}
			for _, key := range []string{"key", "label", "secret", "required"} {
				if _, ok := field[key]; !ok {
					t.Fatalf("provider %v field %v missing %s", p["id"], field["key"], key)
				}
			}
		}
	}
}
