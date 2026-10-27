package main

import (
	"strings"
	"testing"
)

// secondBatchCases 覆盖第二轮 7 家海外运营商的渲染输出。
var secondBatchCases = []struct {
	id       string
	secrets  map[string]string
	acmeWant string
	ddnsWant string
}{
	{
		id: "route53",
		secrets: map[string]string{
			"route53_region":            "us-east-1",
			"route53_access_key_id":     "AKIA",
			"route53_secret_access_key": "secret",
			"route53_session_token":     "token",
			"route53_profile":           "my-profile",
			"route53_hosted_zone_id":    "/hostedzone/Z123",
		},
		acmeWant: "        dns route53 {\n            region \"us-east-1\"\n            access_key_id \"AKIA\"\n            secret_access_key \"secret\"\n            session_token \"token\"\n            profile \"my-profile\"\n            hosted_zone_id \"/hostedzone/Z123\"\n        }\n",
		ddnsWant: "    provider route53 {\n        region \"us-east-1\"\n        access_key_id \"AKIA\"\n        secret_access_key \"secret\"\n        session_token \"token\"\n        profile \"my-profile\"\n        hosted_zone_id \"/hostedzone/Z123\"\n    }\n",
	},
	{
		id:       "hetzner",
		secrets:  map[string]string{"hetzner_api_token": "tok"},
		acmeWant: "        dns hetzner {\n            api_token \"tok\"\n        }\n",
		ddnsWant: "    provider hetzner {\n        api_token \"tok\"\n    }\n",
	},
	{
		id: "gandi",
		secrets: map[string]string{
			"gandi_bearer_token": "tok",
			"gandi_sharing_id":   "sharing-1",
		},
		acmeWant: "        dns gandi {\n            bearer_token \"tok\"\n            sharing_id \"sharing-1\"\n        }\n",
		ddnsWant: "    provider gandi {\n        bearer_token \"tok\"\n        sharing_id \"sharing-1\"\n    }\n",
	},
	{
		id: "ovh",
		secrets: map[string]string{
			"ovh_application_key":    "app-key",
			"ovh_application_secret": "app-secret",
			"ovh_consumer_key":       "consumer-key",
			"ovh_endpoint":           "https://eu.api.ovh.com/1.0",
		},
		acmeWant: "        dns ovh {\n            endpoint \"https://eu.api.ovh.com/1.0\"\n            application_key \"app-key\"\n            application_secret \"app-secret\"\n            consumer_key \"consumer-key\"\n        }\n",
		ddnsWant: "    provider ovh {\n        endpoint \"https://eu.api.ovh.com/1.0\"\n        application_key \"app-key\"\n        application_secret \"app-secret\"\n        consumer_key \"consumer-key\"\n    }\n",
	},
	{
		id: "scaleway",
		secrets: map[string]string{
			"scaleway_secret_key":      "secret",
			"scaleway_organization_id": "org-123",
		},
		acmeWant: "        dns scaleway {\n            secret_key \"secret\"\n            organization_id \"org-123\"\n        }\n",
		ddnsWant: "    provider scaleway {\n        secret_key \"secret\"\n        organization_id \"org-123\"\n    }\n",
	},
	{
		id: "linode",
		secrets: map[string]string{
			"linode_api_token":   "tok",
			"linode_api_url":     "https://api.linode.com/v4",
			"linode_api_version": "v4beta",
		},
		acmeWant: "        dns linode {\n            api_token \"tok\"\n            api_url \"https://api.linode.com/v4\"\n            api_version \"v4beta\"\n        }\n",
		ddnsWant: "    provider linode {\n        api_token \"tok\"\n        api_url \"https://api.linode.com/v4\"\n        api_version \"v4beta\"\n    }\n",
	},
	{
		id:       "godaddy",
		secrets:  map[string]string{"godaddy_api_token": "tok"},
		acmeWant: "        dns godaddy {\n            api_token \"tok\"\n        }\n",
		ddnsWant: "    provider godaddy {\n        api_token \"tok\"\n    }\n",
	},
}

// TestSecondBatchRenderACMEDNS 锁定 7 家运营商的 ACME DNS-01 渲染。
func TestSecondBatchRenderACMEDNS(t *testing.T) {
	for _, item := range secondBatchCases {
		cred := Credential{Provider: item.id, Secrets: item.secrets}
		out := renderTLSBlock(cred, IssuerConfig{Provider: "auto"}, "    ")
		if !strings.Contains(out, item.acmeWant) {
			t.Fatalf("renderTLSBlock(%s):\n%s\nwant to contain:\n%s", item.id, out, item.acmeWant)
		}
	}
}

// TestSecondBatchRenderDDNS 锁定 7 家运营商的动态 DNS 渲染。
func TestSecondBatchRenderDDNS(t *testing.T) {
	for _, item := range secondBatchCases {
		conf, err := renderDynamicDNSConfig([]npmDynamicDNS{
			{
				Name:          "ddns",
				DomainNames:   []string{"example.com"},
				CredentialID:  "cred-1",
				IPv4:          true,
				CheckInterval: "5m",
				Enabled:       true,
			},
		}, []Credential{{ID: "cred-1", Provider: item.id, Secrets: item.secrets}})
		if err != nil {
			t.Fatalf("renderDynamicDNSConfig(%s) error = %v", item.id, err)
		}
		if !strings.Contains(conf, item.ddnsWant) {
			t.Fatalf("renderDynamicDNSConfig(%s):\n%s\nwant to contain:\n%s", item.id, conf, item.ddnsWant)
		}
	}
}

// TestSecondBatchCredentialValidation 7 家运营商的必填校验与 env 导入。
func TestSecondBatchCredentialValidation(t *testing.T) {
	for _, item := range secondBatchCases {
		cred := Credential{Provider: item.id, Secrets: item.secrets}
		if err := validateCredentialSecret(cred); err != nil {
			t.Fatalf("validateCredentialSecret(%s) error = %v", item.id, err)
		}
		if !credentialIsComplete(cred) {
			t.Fatalf("credentialIsComplete(%s) = false", item.id)
		}
		// 缺必填字段应报错。
		for _, field := range providerSpecByID(item.id).Fields {
			if !field.Required {
				continue
			}
			partial := map[string]string{}
			for k, v := range item.secrets {
				if k != field.Key {
					partial[k] = v
				}
			}
			err := validateCredentialSecret(Credential{Provider: item.id, Secrets: partial})
			if err == nil {
				t.Fatalf("%s 缺少 %s 时应报错", item.id, field.Key)
			}
		}
	}
}
