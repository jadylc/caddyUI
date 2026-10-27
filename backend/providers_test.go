package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestProviderSpecsAreWellFormed 约束注册表本身，保证后续新增 provider 不会漏配。
func TestProviderSpecsAreWellFormed(t *testing.T) {
	ids := map[string]bool{}
	aliases := map[string]string{}
	for _, spec := range providerSpecs {
		if spec.ID == "" || spec.Name == "" {
			t.Fatalf("provider %q 缺少 id 或名称", spec.ID)
		}
		if ids[spec.ID] {
			t.Fatalf("provider id 重复: %s", spec.ID)
		}
		ids[spec.ID] = true
		if len(spec.Fields) == 0 {
			t.Fatalf("provider %s 未定义字段", spec.ID)
		}
		for _, alias := range spec.Aliases {
			if owner, ok := aliases[alias]; ok {
				t.Fatalf("别名 %q 同时属于 %s 与 %s", alias, owner, spec.ID)
			}
			aliases[alias] = spec.ID
		}
		for _, field := range spec.Fields {
			if field.Key == "" || field.Label == "" {
				t.Fatalf("provider %s 的字段缺少 key 或 label", spec.ID)
			}
		}
		if spec.DNS {
			if spec.renderACMEDNS == nil {
				t.Fatalf("provider %s 标记为 DNS 但缺少 ACME 渲染", spec.ID)
			}
			if spec.renderDDNS == nil {
				t.Fatalf("provider %s 标记为 DNS 但缺少 DDNS 渲染", spec.ID)
			}
		}
		if spec.DDNS && !spec.DNS {
			t.Fatalf("provider %s 可用于 DDNS 但未标记为 DNS", spec.ID)
		}
		if spec.Renewable && spec.renewDomain == nil {
			t.Fatalf("provider %s 标记为可续期但缺少续期实现", spec.ID)
		}
		if spec.Registrar && spec.fetchExpiry == nil {
			t.Fatalf("provider %s 可用于域名监控但缺少到期查询", spec.ID)
		}
	}
}

// TestProviderSpecLookupNormalizesAliases 校验别名归一化与历史写法兼容。
func TestProviderSpecLookupNormalizesAliases(t *testing.T) {
	cases := map[string]string{
		"阿里云":              "alidns",
		"阿里云 DNS":          "alidns",
		"CLOUDFLARE":       "cloudflare",
		"Digital Platform": "digitalplat",
		"digital-platform": "digitalplat",
		"he":               "he",
	}
	for input, want := range cases {
		if got := normalizeDomainMonitorRegistrarProvider(input); got != want {
			t.Fatalf("normalizeDomainMonitorRegistrarProvider(%q) = %q, want %q", input, got, want)
		}
	}
	if got := normalizeDomainMonitorRegistrarProvider("  阿里 云dns "); got != "alidns" {
		t.Fatalf("带空格别名归一化失败: %q", got)
	}
	if got := normalizeDomainMonitorRegistrarProvider("未知厂商"); got != "未知厂商" {
		t.Fatalf("未知厂商应原样返回，得到 %q", got)
	}
	if got := normalizeDomainMonitorRegistrarProvider("  "); got != "" {
		t.Fatalf("空输入应返回空串，得到 %q", got)
	}
}

// TestProviderCredentialTemplateMatchesHistory 锁定前端依赖的 env 模板文案。
func TestProviderCredentialTemplateMatchesHistory(t *testing.T) {
	want := map[string]string{
		"alidns":     "ALICLOUD_ACCESS_KEY=<AccessKey ID>\nALICLOUD_SECRET_KEY=<AccessKey Secret>",
		"cloudflare": "CLOUDFLARE_API_TOKEN=<API Token>\nCLOUDFLARE_ACCOUNT_ID=<Account ID>",
		"dnspod":     "DNSPOD_TOKEN=<APP_ID,APP_TOKEN>",
		"he":         "HE_API_KEY=<API Key>",
	}
	for id, expected := range want {
		spec := providerSpecByID(id)
		if spec == nil {
			t.Fatalf("provider %s 不存在", id)
		}
		if got := credentialTemplate(spec); got != expected {
			t.Fatalf("credentialTemplate(%s) = %q, want %q", id, got, expected)
		}
	}
}

// TestCredentialSecretsStayCompatibleWithLegacyFields 历史 provider 必须继续写回类型化字段，
// 新 provider 才写入 Secrets，从而保证 credentials.json 格式不变。
func TestCredentialSecretsStayCompatibleWithLegacyFields(t *testing.T) {
	legacy := Credential{Provider: "cloudflare"}
	credentialSetSecret(&legacy, "cf_token", "token")
	if legacy.CFToken != "token" {
		t.Fatalf("CFToken = %q, want token", legacy.CFToken)
	}
	if legacy.Secrets != nil {
		t.Fatalf("历史 provider 不应写入 Secrets: %#v", legacy.Secrets)
	}
	if got := credentialSecretValue(legacy, "cf_token"); got != "token" {
		t.Fatalf("credentialSecretValue = %q, want token", got)
	}

	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	if !strings.Contains(string(encoded), `"cf_token":"token"`) {
		t.Fatalf("历史凭据 JSON 缺少 cf_token: %s", encoded)
	}
	if strings.Contains(string(encoded), "secrets") {
		t.Fatalf("历史凭据 JSON 不应出现 secrets: %s", encoded)
	}

	// 新 provider（示例：RFC2136 风格的多字段凭据）走 Secrets。
	fresh := Credential{Provider: "rfc2136"}
	credentialSetSecret(&fresh, "tsig_key", "secret")
	if fresh.Secrets["tsig_key"] != "secret" {
		t.Fatalf("新 provider 未写入 Secrets: %#v", fresh.Secrets)
	}
	freshEncoded, err := json.Marshal(fresh)
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	if !strings.Contains(string(freshEncoded), `"secrets":{"tsig_key":"secret"}`) {
		t.Fatalf("新凭据 JSON 缺少 secrets: %s", freshEncoded)
	}
}

// TestCredentialUnmarshalLegacyJSON 旧配置文件（无 secrets 字段）必须能照常解析。
func TestCredentialUnmarshalLegacyJSON(t *testing.T) {
	raw := `[{"id":"cred-1","name":"旧凭据","provider":"dnspod","dnspod_token":"12345,token"}]`
	var creds []Credential
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		t.Fatalf("unmarshal legacy credentials error = %v", err)
	}
	if len(creds) != 1 || creds[0].DNSPodToken != "12345,token" || creds[0].Secrets != nil {
		t.Fatalf("解析旧凭据失败: %#v", creds)
	}
	if !credentialIsComplete(creds[0]) {
		t.Fatalf("旧凭据应通过必填校验: %#v", creds[0])
	}
	if got := credentialReuseKey(creds[0]); got != "dnspod:12345,token" {
		t.Fatalf("复用分组键 = %q", got)
	}
}

// TestCredentialErrorMessagesMatchHistory 锁定校验与导入的历史文案。
func TestCredentialErrorMessagesMatchHistory(t *testing.T) {
	validateCases := []struct {
		cred Credential
		want string
	}{
		{Credential{Provider: "alidns"}, "阿里云 DNS 需要 AccessKey ID 与 Secret"},
		{Credential{Provider: "cloudflare"}, "Cloudflare 需要 API Token"},
		{Credential{Provider: "dnspod"}, "DNSPod 需要 API Token，格式为 APP_ID,APP_TOKEN"},
		{Credential{Provider: "he"}, "Hurricane Electric 需要 API Key"},
		{Credential{Provider: "digitalplat"}, "DigitalPlat 需要 API Key"},
		{Credential{Provider: "dnshe"}, "DNSHE 需要 API Key 和 API Secret"},
		{Credential{Provider: "nope"}, "未知提供商：nope"},
	}
	for _, item := range validateCases {
		err := validateCredentialSecret(item.cred)
		if err == nil || err.Error() != item.want {
			t.Fatalf("validateCredentialSecret(%s) = %v, want %q", item.cred.Provider, err, item.want)
		}
	}

	importCases := []struct {
		provider string
		raw      string
		want     string
	}{
		{"alidns", "ALICLOUD_ACCESS_KEY=onlykey", "阿里云 DNS 需要 ALICLOUD_ACCESS_KEY 和 ALICLOUD_SECRET_KEY"},
		{"cloudflare", "", "Cloudflare 需要 CLOUDFLARE_API_TOKEN"},
		{"dnspod", "", "DNSPod 需要 DNSPOD_TOKEN，格式为 APP_ID,APP_TOKEN"},
		{"he", "", "Hurricane Electric 需要 HE_API_KEY"},
		{"nope", "", "不支持的 DNS 提供商：nope"},
	}
	for _, item := range importCases {
		_, err := credentialFromDNSProvider(item.provider, item.raw)
		if err == nil || err.Error() != item.want {
			t.Fatalf("credentialFromDNSProvider(%s) = %v, want %q", item.provider, err, item.want)
		}
	}

	cred, err := credentialFromDNSProvider("dnspod", "DNSPOD_LOGIN_TOKEN=\"12345,tok\"")
	if err != nil {
		t.Fatalf("导入 DNSPod 凭据失败: %v", err)
	}
	if cred.DNSPodToken != "12345,tok" {
		t.Fatalf("DNSPodToken = %q", cred.DNSPodToken)
	}
}

// TestACMEDNSRenderStaysByteIdentical 锁定 ACME DNS-01 渲染格式。
func TestACMEDNSRenderStaysByteIdentical(t *testing.T) {
	cases := []struct {
		cred Credential
		want string
	}{
		{Credential{Provider: "cloudflare", CFToken: "tok"}, "        dns cloudflare \"tok\"\n"},
		{Credential{Provider: "dnspod", DNSPodToken: "1,tok"}, "        dns dnspod \"1,tok\"\n"},
		{
			Credential{Provider: "alidns", AliyunKey: "k", AliyunSecret: "s"},
			"        dns alidns {\n            access_key_id \"k\"\n            access_key_secret \"s\"\n        }\n",
		},
		{
			Credential{Provider: "he", HEAPIKey: "k"},
			"        dns he {\n            api_key \"k\"\n        }\n",
		},
	}
	for _, item := range cases {
		out := renderTLSBlock(item.cred, IssuerConfig{Provider: "auto"}, "    ")
		if !strings.Contains(out, item.want) {
			t.Fatalf("renderTLSBlock(%s):\n%s\nwant to contain:\n%s", item.cred.Provider, out, item.want)
		}
	}
}

// TestDDNSProviderRenderStaysByteIdentical 锁定 dynamic_dns provider 渲染格式。
func TestDDNSProviderRenderStaysByteIdentical(t *testing.T) {
	cases := []struct {
		cred Credential
		want string
	}{
		{Credential{Provider: "cloudflare", CFToken: "tok"}, "    provider cloudflare \"tok\"\n"},
		{Credential{Provider: "dnspod", DNSPodToken: "1,tok"}, "    provider dnspod \"1,tok\"\n"},
		{
			Credential{Provider: "alidns", AliyunKey: "k", AliyunSecret: "s"},
			"    provider alidns {\n        access_key_id \"k\"\n        access_key_secret \"s\"\n    }\n",
		},
		{
			Credential{Provider: "he", HEAPIKey: "k"},
			"    provider he {\n        api_key \"k\"\n    }\n",
		},
	}
	for _, item := range cases {
		conf, err := renderDynamicDNSConfig([]npmDynamicDNS{
			{
				Name:          "ddns",
				DomainNames:   []string{"example.com"},
				CredentialID:  "cred-1",
				IPv4:          true,
				CheckInterval: "5m",
				Enabled:       true,
			},
		}, []Credential{{ID: "cred-1", Provider: item.cred.Provider, CFToken: item.cred.CFToken, DNSPodToken: item.cred.DNSPodToken, AliyunKey: item.cred.AliyunKey, AliyunSecret: item.cred.AliyunSecret, HEAPIKey: item.cred.HEAPIKey}})
		if err != nil {
			t.Fatalf("renderDynamicDNSConfig(%s) error = %v", item.cred.Provider, err)
		}
		if !strings.Contains(conf, item.want) {
			t.Fatalf("renderDynamicDNSConfig(%s):\n%s\nwant to contain:\n%s", item.cred.Provider, conf, item.want)
		}
	}
}

// TestDDNSCredentialRejectsRegistrarOnlyCredential 域名商凭据不能用于动态 DNS。
// 注意：renderDynamicDNSConfig 对这类条目是「跳过并写日志」，与历史行为一致，
// 因此这里断言凭据解析层返回错误。
func TestDDNSCredentialRejectsRegistrarOnlyCredential(t *testing.T) {
	item := npmDynamicDNS{
		Name:          "ddns",
		DomainNames:   []string{"example.com"},
		CredentialID:  "cred-1",
		IPv4:          true,
		CheckInterval: "5m",
		Enabled:       true,
	}
	creds := []Credential{{ID: "cred-1", Provider: "dnshe", DNSHEAPIKey: "k", DNSHEAPISecret: "s"}}
	if _, err := dynamicDNSCredentialForItem(item, creds); err == nil || !strings.Contains(err.Error(), "不支持的 DNS 提供商") {
		t.Fatalf("err = %v, want 不支持的 DNS 提供商", err)
	}
	conf, err := renderDynamicDNSConfig([]npmDynamicDNS{item}, creds)
	if err != nil {
		t.Fatalf("renderDynamicDNSConfig error = %v, want nil（历史行为为跳过）", err)
	}
	if strings.Contains(conf, "provider dnshe") {
		t.Fatalf("域名商凭据不应出现在配置中:\n%s", conf)
	}
}

// TestRegistrarLookupByHostedZone 托管后缀 → 域名商映射。
func TestRegistrarLookupByHostedZone(t *testing.T) {
	cases := map[string]string{
		"wvw.cc.cd":         "dnshe",
		"a.b.us.kg":         "digitalplat",
		"example.dpdns.org": "digitalplat",
		"kdns.fr":           "", // 未托管：交给 RDAP
		"example.com":       "",
	}
	for host, want := range cases {
		if got := domainMonitorRegistrarProvider(host); got != want {
			t.Fatalf("domainMonitorRegistrarProvider(%q) = %q, want %q", host, got, want)
		}
	}
}

// TestRegistrarCredentialValidation 域名商凭据校验按注册表必填字段判定。
func TestRegistrarCredentialValidation(t *testing.T) {
	if !registrarCredentialValid("dnshe", Credential{Provider: "dnshe", DNSHEAPIKey: "k", DNSHEAPISecret: "s"}) {
		t.Fatal("完整 DNSHE 凭据应有效")
	}
	if registrarCredentialValid("dnshe", Credential{Provider: "dnshe", DNSHEAPIKey: "k"}) {
		t.Fatal("缺少 secret 的 DNSHE 凭据应无效")
	}
	if registrarCredentialValid("dnshe", Credential{Provider: "digitalplat", DigitalPlatAPIKey: "k"}) {
		t.Fatal("provider 不匹配的凭据应无效")
	}
	if registrarCredentialValid("dnspod", Credential{Provider: "dnspod", DNSPodToken: "1,tok"}) {
		t.Fatal("非域名商凭据应无效")
	}
}

// TestProvidersEndpointExposesSchema 前端依赖 /providers 下发的字段 schema。
func TestProvidersEndpointExposesSchema(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/caddy/providers", nil)
	rec := httptest.NewRecorder()
	npmProvidersHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var payload []struct {
		ID        string              `json:"id"`
		Name      string              `json:"name"`
		DNS       bool                `json:"dns"`
		DDNS      bool                `json:"ddns"`
		Registrar bool                `json:"registrar"`
		Renewable bool                `json:"renewable"`
		Fields    []providerFieldView `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal providers = %v", err)
	}
	if len(payload) != len(providerSpecs) {
		t.Fatalf("返回 %d 个 provider, want %d", len(payload), len(providerSpecs))
	}
	byID := map[string]int{}
	for i, item := range payload {
		byID[item.ID] = i
		if item.Name == "" || len(item.Fields) == 0 {
			t.Fatalf("provider %s 缺少名称或字段: %#v", item.ID, item)
		}
	}
	// DNS 服务商必须带 env 名称，供 DNS-01 凭据文本拼装。
	cf, ok := byID["cloudflare"]
	if !ok {
		t.Fatal("缺少 cloudflare")
	}
	if !payload[cf].DNS || !payload[cf].DDNS {
		t.Fatalf("cloudflare 应同时支持 DNS 与 DDNS: %#v", payload[cf])
	}
	if payload[cf].Fields[0].EnvName != "CLOUDFLARE_API_TOKEN" {
		t.Fatalf("cloudflare env_name = %q", payload[cf].Fields[0].EnvName)
	}
	// 域名商必须标记 registrar，且续期能力可查询。
	dnshe, ok := byID["dnshe"]
	if !ok {
		t.Fatal("缺少 dnshe")
	}
	if !payload[dnshe].Registrar || !payload[dnshe].Renewable {
		t.Fatalf("dnshe 应为可续期域名商: %#v", payload[dnshe])
	}
	if payload[dnshe].DNS || payload[dnshe].DDNS {
		t.Fatalf("dnshe 不应被标记为 DNS 凭据: %#v", payload[dnshe])
	}
}

// TestCredentialCreateAcceptsSecretsOnly 前端统一只提交 secrets，
// 后端要回填历史字段并保持 credentials.json 不含重复存储。
func TestCredentialCreateAcceptsSecretsOnly(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	body := strings.NewReader(`{"name":"CF","provider":"cloudflare","secrets":{"cf_token":"tok-123"}}`)
	req := httptest.NewRequest(http.MethodPost, "/credentials", body)
	rec := httptest.NewRecorder()
	credentialsHandler(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}

	creds, err := loadCredentials()
	if err != nil {
		t.Fatalf("loadCredentials error = %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("凭据数量 = %d, want 1", len(creds))
	}
	if creds[0].CFToken != "tok-123" {
		t.Fatalf("CFToken = %q, want tok-123", creds[0].CFToken)
	}
	if len(creds[0].Secrets) != 0 {
		t.Fatalf("历史字段已回填，不应保留 secrets: %#v", creds[0].Secrets)
	}
	if !credentialHasSecret(creds[0]) {
		t.Fatal("凭据应被识别为已配置密钥")
	}

	// 落盘内容仍使用历史字段名，保证旧版本可读。
	raw, err := os.ReadFile(credentialPath)
	if err != nil {
		t.Fatalf("read credentials.json error = %v", err)
	}
	if !strings.Contains(string(raw), `"cf_token": "tok-123"`) {
		t.Fatalf("credentials.json 未写入 cf_token: %s", raw)
	}
	if strings.Contains(string(raw), "secrets") {
		t.Fatalf("credentials.json 不应出现 secrets: %s", raw)
	}
}
