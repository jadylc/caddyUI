package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestHuaweiRenderACMEDNS 华为云 DNS-01 渲染：region_id 可选。
func TestHuaweiRenderACMEDNS(t *testing.T) {
	cred := Credential{
		Provider: "huaweicloud",
		Secrets:  map[string]string{"huawei_access_key_id": "AKID", "huawei_secret_access_key": "SECRET"},
	}
	out := renderTLSBlock(cred, IssuerConfig{Provider: "auto"}, "    ")
	want := "        dns huaweicloud {\n            access_key_id \"AKID\"\n            secret_access_key \"SECRET\"\n        }\n"
	if !strings.Contains(out, want) {
		t.Fatalf("renderTLSBlock(huaweicloud):\n%s\nwant to contain:\n%s", out, want)
	}

	// 带 region_id 时多一行。
	cred.Secrets["huawei_region_id"] = "cn-north-4"
	out = renderTLSBlock(cred, IssuerConfig{Provider: "auto"}, "    ")
	wantWithRegion := "            region_id \"cn-north-4\"\n"
	if !strings.Contains(out, wantWithRegion) {
		t.Fatalf("renderTLSBlock(huaweicloud with region):\n%s\nwant to contain:\n%s", out, wantWithRegion)
	}
}

// TestHuaweiRenderDDNS 华为云动态 DNS 渲染。
func TestHuaweiRenderDDNS(t *testing.T) {
	cred := Credential{
		Provider: "huaweicloud",
		Secrets:  map[string]string{"huawei_access_key_id": "AKID", "huawei_secret_access_key": "SECRET"},
	}
	conf, err := renderDynamicDNSConfig([]npmDynamicDNS{
		{
			Name:          "ddns",
			DomainNames:   []string{"example.com"},
			CredentialID:  "cred-1",
			IPv4:          true,
			CheckInterval: "5m",
			Enabled:       true,
		},
	}, []Credential{{ID: "cred-1", Provider: "huaweicloud", Secrets: cred.Secrets}})
	if err != nil {
		t.Fatalf("renderDynamicDNSConfig error = %v", err)
	}
	want := "    provider huaweicloud {\n        access_key_id \"AKID\"\n        secret_access_key \"SECRET\"\n    }\n"
	if !strings.Contains(conf, want) {
		t.Fatalf("renderDynamicDNSConfig(huaweicloud):\n%s\nwant to contain:\n%s", conf, want)
	}
}

// TestHuaweiCredentialSecretsRoundTrip 华为云凭据写入 Secrets，校验与导入走注册表。
func TestHuaweiCredentialSecretsRoundTrip(t *testing.T) {
	cred := Credential{
		Provider: "huaweicloud",
		Secrets: map[string]string{
			"huawei_access_key_id":     "AKID",
			"huawei_secret_access_key": "SECRET",
			"huawei_region_id":         "cn-north-4",
		},
	}
	if err := validateCredentialSecret(cred); err != nil {
		t.Fatalf("validateCredentialSecret error = %v", err)
	}
	if !credentialIsComplete(cred) {
		t.Fatal("credentialIsComplete = false, want true")
	}

	// 序列化后应落在 Secrets，不带历史字段。
	encoded, err := json.Marshal(cred)
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	if !strings.Contains(string(encoded), `"secrets":{"huawei_access_key_id":"AKID"`) {
		t.Fatalf("marshaled = %s", encoded)
	}

	// 缺 secret 时应报错。
	partial := Credential{Provider: "huaweicloud", Secrets: map[string]string{"huawei_access_key_id": "AKID"}}
	err = validateCredentialSecret(partial)
	if err == nil || !strings.Contains(err.Error(), "Secret Access Key") {
		t.Fatalf("partial validate error = %v", err)
	}

	// env 文本导入。
	imported, err := credentialFromDNSProvider("huaweicloud",
		"HUAWEICLOUD_ACCESS_KEY_ID=AKID\nHUAWEICLOUD_SECRET_ACCESS_KEY=SECRET\nHUAWEICLOUD_REGION_ID=cn-north-4")
	if err != nil {
		t.Fatalf("credentialFromDNSProvider error = %v", err)
	}
	if imported.Secrets["huawei_access_key_id"] != "AKID" || imported.Secrets["huawei_secret_access_key"] != "SECRET" {
		t.Fatalf("imported = %#v", imported.Secrets)
	}
	if imported.Secrets["huawei_region_id"] != "cn-north-4" {
		t.Fatalf("region = %q", imported.Secrets["huawei_region_id"])
	}
}

// TestHuaweiTemplate 华为云凭据模板。
func TestHuaweiTemplate(t *testing.T) {
	spec := providerSpecByID("huaweicloud")
	if spec == nil {
		t.Fatal("huaweicloud spec missing")
	}
	want := "HUAWEICLOUD_ACCESS_KEY_ID=<Access Key ID>\nHUAWEICLOUD_SECRET_ACCESS_KEY=<Secret Access Key>\nHUAWEICLOUD_REGION_ID=<Region ID>"
	if got := credentialTemplate(spec); got != want {
		t.Fatalf("credentialTemplate = %q, want %q", got, want)
	}
}

// TestHuaweiSpecAppearsInDNSProviderList 华为云应出现在 DNS 提供商列表与 schema 中。
func TestHuaweiSpecAppearsInDNSProviderList(t *testing.T) {
	ids := map[string]bool{}
	for _, spec := range dnsProviderSpecs() {
		ids[spec.ID] = true
	}
	if !ids["huaweicloud"] {
		t.Fatalf("huaweicloud not in dnsProviderSpecs: %v", ids)
	}
	views := providerSpecViews()
	found := false
	for _, v := range views {
		if v["id"] == "huaweicloud" {
			found = true
			if v["dns"] != true || v["ddns"] != true {
				t.Fatalf("huaweicloud flags = %#v", v)
			}
			fields, ok := v["fields"].([]providerFieldView)
			if !ok || len(fields) != 3 {
				t.Fatalf("huaweicloud fields = %#v", v["fields"])
			}
		}
	}
	if !found {
		t.Fatal("huaweicloud missing from providerSpecViews")
	}
}
