package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// newProviderCases 覆盖本轮新增的 6 家运营商的渲染与凭据往返。
var newProviderCases = []struct {
	id          string
	provider    string
	secrets     map[string]string
	acmeWant    string
	ddnsWant    string
	envTemplate string
}{
	{
		id:       "rfc2136",
		provider: "rfc2136",
		secrets: map[string]string{
			"rfc2136_server":   "203.0.113.53",
			"rfc2136_key_name": "caddy-key",
			"rfc2136_key":      "Zm9vYmFy",
			"rfc2136_key_alg":  "hmac-sha256",
		},
		acmeWant:    "        dns rfc2136 {\n            server \"203.0.113.53\"\n            key_name \"caddy-key\"\n            key \"Zm9vYmFy\"\n            key_alg \"hmac-sha256\"\n        }\n",
		ddnsWant:    "    provider rfc2136 {\n        server \"203.0.113.53\"\n        key_name \"caddy-key\"\n        key \"Zm9vYmFy\"\n        key_alg \"hmac-sha256\"\n    }\n",
		envTemplate: "RFC2136_SERVER=<Server>\nRFC2136_KEY_NAME=<Key Name>\nRFC2136_KEY=<Key>\nRFC2136_KEY_ALG=<Key Alg>",
	},
	{
		id:       "duckdns",
		provider: "duckdns",
		secrets: map[string]string{
			"duckdns_token":           "tok",
			"duckdns_override_domain": "example.duckdns.org",
			"duckdns_resolver":        "1.1.1.1",
		},
		acmeWant:    "        dns duckdns {\n            api_token \"tok\"\n            override_domain \"example.duckdns.org\"\n            resolver \"1.1.1.1\"\n        }\n",
		ddnsWant:    "    provider duckdns {\n        api_token \"tok\"\n        override_domain \"example.duckdns.org\"\n        resolver \"1.1.1.1\"\n    }\n",
		envTemplate: "DUCKDNS_API_TOKEN=<API Token>\nDUCKDNS_OVERRIDE_DOMAIN=<Override Domain>\nDUCKDNS_RESOLVER=<Resolver>",
	},
	{
		id:       "dynu",
		provider: "dynu",
		secrets: map[string]string{
			"dynu_token":      "tok",
			"dynu_own_domain": "example.dynu.com",
		},
		acmeWant:    "        dns dynu {\n            api_token \"tok\"\n            own_domain \"example.dynu.com\"\n        }\n",
		ddnsWant:    "    provider dynu {\n        api_token \"tok\"\n        own_domain \"example.dynu.com\"\n    }\n",
		envTemplate: "DYNU_API_TOKEN=<API Token>\nDYNU_OWN_DOMAIN=<Own Domain>",
	},
	{
		id:       "namecheap",
		provider: "namecheap",
		secrets: map[string]string{
			"namecheap_api_key":      "api-key",
			"namecheap_user":         "user",
			"namecheap_api_endpoint": "https://api.namecheap.com/xml.response",
			"namecheap_client_ip":    "1.2.3.4",
		},
		acmeWant:    "        dns namecheap {\n            api_key \"api-key\"\n            user \"user\"\n            api_endpoint \"https://api.namecheap.com/xml.response\"\n            client_ip \"1.2.3.4\"\n        }\n",
		ddnsWant:    "    provider namecheap {\n        api_key \"api-key\"\n        user \"user\"\n        api_endpoint \"https://api.namecheap.com/xml.response\"\n        client_ip \"1.2.3.4\"\n    }\n",
		envTemplate: "NAMECHEAP_API_KEY=<API Key>\nNAMECHEAP_USER=<User>\nNAMECHEAP_API_ENDPOINT=<API Endpoint>\nNAMECHEAP_CLIENT_IP=<Client IP>",
	},
	{
		id:       "porkbun",
		provider: "porkbun",
		secrets: map[string]string{
			"porkbun_api_key":        "api-key",
			"porkbun_api_secret_key": "secret",
		},
		acmeWant:    "        dns porkbun {\n            api_key \"api-key\"\n            api_secret_key \"secret\"\n        }\n",
		ddnsWant:    "    provider porkbun {\n        api_key \"api-key\"\n        api_secret_key \"secret\"\n    }\n",
		envTemplate: "PORKBUN_API_KEY=<API Key>\nPORKBUN_API_SECRET_KEY=<API Secret Key>",
	},
	{
		id:       "desec",
		provider: "desec",
		secrets: map[string]string{
			"desec_token": "tok",
		},
		acmeWant:    "        dns desec {\n            token \"tok\"\n        }\n",
		ddnsWant:    "    provider desec {\n        token \"tok\"\n    }\n",
		envTemplate: "DESEC_TOKEN=<Token>",
	},
}

// TestNewProvidersRenderACMEDNS 锁定 ACME DNS-01 渲染输出。
func TestNewProvidersRenderACMEDNS(t *testing.T) {
	for _, item := range newProviderCases {
		cred := Credential{Provider: item.provider, Secrets: item.secrets}
		out := renderTLSBlock(cred, IssuerConfig{Provider: "auto"}, "    ")
		if !strings.Contains(out, item.acmeWant) {
			t.Fatalf("renderTLSBlock(%s):\n%s\nwant to contain:\n%s", item.provider, out, item.acmeWant)
		}
	}
}

// TestNewProvidersRenderDDNS 锁定动态 DNS 渲染输出。
func TestNewProvidersRenderDDNS(t *testing.T) {
	for _, item := range newProviderCases {
		creds := []Credential{{ID: "cred-1", Provider: item.provider, Secrets: item.secrets}}
		conf, err := renderDynamicDNSConfig([]npmDynamicDNS{
			{
				Name:          "ddns",
				DomainNames:   []string{"example.com"},
				CredentialID:  "cred-1",
				IPv4:          true,
				CheckInterval: "5m",
				Enabled:       true,
			},
		}, creds)
		if err != nil {
			t.Fatalf("renderDynamicDNSConfig(%s) error = %v", item.provider, err)
		}
		if !strings.Contains(conf, item.ddnsWant) {
			t.Fatalf("renderDynamicDNSConfig(%s):\n%s\nwant to contain:\n%s", item.provider, conf, item.ddnsWant)
		}
	}
}

// TestNewProviderCredentialRoundTrip 新运营商凭据写入 Secrets，校验与导入走注册表。
func TestNewProviderCredentialRoundTrip(t *testing.T) {
	for _, item := range newProviderCases {
		cred := Credential{Provider: item.provider, Secrets: item.secrets}
		if err := validateCredentialSecret(cred); err != nil {
			t.Fatalf("validateCredentialSecret(%s) error = %v", item.provider, err)
		}
		if !credentialIsComplete(cred) {
			t.Fatalf("credentialIsComplete(%s) = false", item.provider)
		}
		encoded, err := json.Marshal(cred)
		if err != nil {
			t.Fatalf("marshal(%s) error = %v", item.provider, err)
		}
		if !strings.Contains(string(encoded), `"secrets":{`) {
			t.Fatalf("marshaled(%s) = %s", item.provider, encoded)
		}

		// 缺必填字段应报错。
		for _, field := range providerSpecByID(item.provider).Fields {
			if !field.Required {
				continue
			}
			partial := map[string]string{}
			for k, v := range item.secrets {
				if k != field.Key {
					partial[k] = v
				}
			}
			err := validateCredentialSecret(Credential{Provider: item.provider, Secrets: partial})
			if err == nil {
				t.Fatalf("%s 缺少 %s 时应报错", item.provider, field.Key)
			}
		}
	}
}

// TestNewProviderTemplates 锁定 env 模板文案。
func TestNewProviderTemplates(t *testing.T) {
	for _, item := range newProviderCases {
		spec := providerSpecByID(item.provider)
		if spec == nil {
			t.Fatalf("%s spec missing", item.provider)
		}
		if got := credentialTemplate(spec); got != item.envTemplate {
			t.Fatalf("credentialTemplate(%s) = %q, want %q", item.provider, got, item.envTemplate)
		}
	}
}

// TestNewProvidersAbsentFromRegistrarList 新运营商不是域名商（porkbun 除外，它同时是 DNS 提供商和域名商）。
func TestNewProvidersAbsentFromRegistrarList(t *testing.T) {
	for _, item := range newProviderCases {
		if item.provider == "porkbun" {
			continue
		}
		spec := providerSpecByID(item.provider)
		if spec.Registrar {
			t.Fatalf("%s 不应标记为 registrar", item.provider)
		}
		for _, name := range registrarProviderSpecs() {
			if name.ID == item.provider {
				t.Fatalf("%s 不应出现在 registrar 列表", item.provider)
			}
		}
	}
}
