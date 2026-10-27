package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// 本文件是 provider（DNS 服务商 / 域名商）的唯一注册表。
//
// 目标：新增一家提供商只需要在 providerSpecs 里加一条，并补齐它自己的
// API 客户端函数；显示名、别名、字段校验、凭据复用分组、DDNS 配置渲染、
// ACME DNS-01 渲染、域名商到期查询与续期全部由注册表驱动，不再散落 switch。
//
// 向后兼容约定：
//   - 历史 provider（alidns/cloudflare/dnspod/he/digitalplat/dnshe）的密钥仍存放在
//     Credential 的类型化字段里，读写统一走 credentialSecretValue / credentialSetSecret，
//     因此 credentials.json 的文件格式与 provider id 保持不变。
//   - 新增 provider 的密钥写入 Credential.Secrets（map），不会污染历史字段。

// providerField 描述一家提供商的一个密钥字段。
// Key 与 Credential 的 JSON 字段名（历史 provider）或 Secrets 的键（新 provider）一致。
type providerField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Placeholder string   `json:"placeholder,omitempty"`
	Help        string   `json:"help,omitempty"`
	Secret      bool     `json:"secret"`
	Required    bool     `json:"required"`
	EnvNames    []string `json:"-"`
}

// ddnsTargetSync 按 zone/record 目标同步（阿里云、DNSPod 这类需要 Zone+RR 的 API）。
type ddnsTargetSync func(target dynamicDNSRecordTarget, cred Credential, publicIPs []string, wantIPv4 bool, wantIPv6 bool) ([]string, error)

// ddnsHostSync 按主机名同步（Cloudflare、Hurricane Electric 这类按 FQDN 操作的 API）。
type ddnsHostSync func(item npmDynamicDNS, host string, cred Credential, publicIPs []string, wantIPv4 bool, wantIPv6 bool) ([]string, error)

// providerFieldView 是 providerField 对外的 JSON 视图（供前端按 schema 渲染表单）。
type providerFieldView struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder,omitempty"`
	Help        string `json:"help,omitempty"`
	Secret      bool   `json:"secret"`
	Required    bool   `json:"required"`
	EnvName     string `json:"env_name,omitempty"`
}

func providerFieldViews(provider string) []providerFieldView {
	fields := credentialFields(provider)
	if len(fields) == 0 {
		return nil
	}
	out := make([]providerFieldView, 0, len(fields))
	for _, f := range fields {
		envName := ""
		if len(f.EnvNames) > 0 {
			envName = f.EnvNames[0]
		}
		out = append(out, providerFieldView{
			Key:         f.Key,
			Label:       f.Label,
			Placeholder: f.Placeholder,
			Help:        f.Help,
			Secret:      f.Secret,
			Required:    f.Required,
			EnvName:     envName,
		})
	}
	return out
}

// providerSpecView 是注册表对外的 JSON 视图。
func providerSpecView(spec *providerSpec) map[string]any {
	return map[string]any{
		"id":          spec.ID,
		"name":        spec.Name,
		"dns":         spec.DNS,
		"ddns":        spec.DDNS,
		"registrar":   spec.Registrar,
		"renewable":   spec.Renewable,
		"fields":      providerFieldViews(spec.ID),
		"credentials": credentialTemplate(spec),
	}
}

// providerSpecViews 返回全部 provider 的 JSON 视图（顺序即注册表顺序）。
func providerSpecViews() []map[string]any {
	out := make([]map[string]any, 0, len(providerSpecs))
	for i := range providerSpecs {
		out = append(out, providerSpecView(&providerSpecs[i]))
	}
	return out
}

// legacySecretKeys 映射到 Credential 类型化字段的密钥键。
var legacySecretKeys = map[string]bool{
	"aliyun_key":           true,
	"aliyun_secret":        true,
	"cf_token":             true,
	"dnspod_token":         true,
	"he_api_key":           true,
	"digital_plat_api_key": true,
	"dnshe_api_key":        true,
	"dnshe_api_secret":     true,
}

// credentialApplySecrets 在请求体解析后调用：把 secrets 中的密钥回填到历史类型化字段，
// 这样前端可以统一只提交 secrets，而 credentials.json 仍保持原有格式。
func credentialApplySecrets(c *Credential) {
	if len(c.Secrets) == 0 {
		return
	}
	for key, value := range c.Secrets {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if strings.TrimSpace(credentialSecretValue(*c, key)) == "" {
			credentialSetSecret(c, key, value)
		}
	}
	// 历史字段已回填，清掉重复的 Secrets 条目。
	for key := range c.Secrets {
		if legacySecretKeys[key] {
			delete(c.Secrets, key)
		}
	}
	if len(c.Secrets) == 0 {
		c.Secrets = nil
	}
}

// providerSpec 一家提供商的完整接入信息。
type providerSpec struct {
	ID         string
	Name       string   // 展示名
	Aliases    []string // 域名商名称归一化别名（已去除空格/连字符/下划线并小写）
	MissingErr string   // 凭据校验失败时的固定文案；为空则按字段自动生成
	ImportErr  string   // 从 env 文本导入凭据失败时的固定文案；为空则自动生成

	DNS         bool // 可作为 ACME DNS-01 凭据
	DDNS        bool // 可用于动态 DNS
	Registrar   bool // 可用于域名监控（域名到期查询）
	Renewable   bool // 域名商支持 API 续期
	HostedZones []string

	Fields []providerField

	// renderACMEDNS 渲染 `dns <module> ...` 块（含尾随换行），indent 为当前缩进。
	renderACMEDNS func(b *strings.Builder, indent string, cred Credential)
	// renderDDNS 渲染 dynamic_dns 块内的 provider 部分。
	renderDDNS func(b *strings.Builder, cred Credential) error

	syncTarget      ddnsTargetSync
	syncHost        ddnsHostSync
	syncErrorText   string // 同步失败文案，默认「记录检查失败」
	syncMissingText string // 未找到记录文案，默认「未找到匹配的 A/AAAA 记录」

	fetchExpiry func(domain string, cred Credential) (time.Time, error)
	renewDomain func(domain string, cred Credential) (domainRenewResult, error)
}

// providerSpecs 顺序即前端下拉框顺序，历史 provider 的顺序不要调整。
var providerSpecs = []providerSpec{
	{
		ID:         "alidns",
		Name:       "阿里云 DNS",
		Aliases:    []string{"aliyun", "alidns", "alibabacloud", "aliyundns", "阿里云", "阿里云dns"},
		MissingErr: "阿里云 DNS 需要 AccessKey ID 与 Secret",
		ImportErr:  "阿里云 DNS 需要 ALICLOUD_ACCESS_KEY 和 ALICLOUD_SECRET_KEY",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "aliyun_key", Label: "AccessKey ID", Secret: false, Required: true, EnvNames: []string{"ALICLOUD_ACCESS_KEY", "ALICLOUD_ACCESS_KEY_ID", "ALIYUN_ACCESS_KEY_ID"}},
			{Key: "aliyun_secret", Label: "AccessKey Secret", Secret: true, Required: true, EnvNames: []string{"ALICLOUD_SECRET_KEY", "ALICLOUD_ACCESS_KEY_SECRET", "ALIYUN_ACCESS_KEY_SECRET"}},
		},
		renderACMEDNS: renderACMEDNSAliyun,
		renderDDNS:    renderDDNSAliyun,
		syncTarget:    syncAliyunDynamicDNSRecord,
	},
	{
		ID:         "cloudflare",
		Name:       "Cloudflare",
		Aliases:    []string{"cloudflare"},
		MissingErr: "Cloudflare 需要 API Token",
		ImportErr:  "Cloudflare 需要 CLOUDFLARE_API_TOKEN",
		DNS:        true,
		DDNS:       true,
		Registrar:  true,
		Fields: []providerField{
			{Key: "cf_token", Label: "API Token", Secret: true, Required: true, EnvNames: []string{"CLOUDFLARE_API_TOKEN", "CF_API_TOKEN"}},
			{Key: "cf_account_id", Label: "Account ID", Secret: false, Required: false, EnvNames: []string{"CLOUDFLARE_ACCOUNT_ID"}, Help: "Registrar API 需要，可留空由后端自动获取"},
		},
		renderACMEDNS: renderACMEDNSCloudflare,
		renderDDNS:    renderDDNSCloudflare,
		syncHost:      syncCloudflareHostRecord,
		fetchExpiry:   fetchCloudflareRegistrarExpiry,
		renewDomain:   renewCloudflareRegistrarDomain,
	},
	{
		ID:         "dnspod",
		Name:       "DNSPod.cn",
		Aliases:    []string{"dnspod"},
		MissingErr: "DNSPod 需要 API Token，格式为 APP_ID,APP_TOKEN",
		ImportErr:  "DNSPod 需要 DNSPOD_TOKEN，格式为 APP_ID,APP_TOKEN",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "dnspod_token", Label: "APP_ID,APP_TOKEN", Placeholder: "APP_ID,APP_TOKEN", Help: "APP_ID,APP_TOKEN", Secret: true, Required: true, EnvNames: []string{"DNSPOD_TOKEN", "DNSPOD_API_TOKEN", "DNSPOD_LOGIN_TOKEN"}},
		},
		renderACMEDNS: renderACMEDNSDNSPod,
		renderDDNS:    renderDDNSDNSPod,
		syncTarget:    syncDNSPodDynamicDNSRecord,
	},
	{
		ID:         "he",
		Name:       "Hurricane Electric",
		Aliases:    []string{"he"},
		MissingErr: "Hurricane Electric 需要 API Key",
		ImportErr:  "Hurricane Electric 需要 HE_API_KEY",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "he_api_key", Label: "API Key", Secret: true, Required: true, EnvNames: []string{"HE_API_KEY", "HEDNS_API_KEY", "HURRICANE_ELECTRIC_API_KEY"}},
		},
		renderACMEDNS:   renderACMEDNSHE,
		renderDDNS:      renderDDNSHE,
		syncHost:        syncHEHostRecord,
		syncErrorText:   "记录更新失败",
		syncMissingText: "没有匹配的 A/AAAA 记录",
	},
	{
		// 华为云 DNS：region_id 可选，不填时 libdns 默认 cn-south-1。
		// 实际写入由 caddy-dynamicdns + libdns/huaweicloud 完成，
		// 后端状态巡检退化为 DNS 解析比对（无 syncHost/syncTarget）。
		ID:         "huaweicloud",
		Name:       "华为云 DNS",
		Aliases:    []string{"huaweicloud", "huawei", "华为云", "华为云dns", "hwcloud"},
		MissingErr: "华为云 DNS 需要 Access Key ID 与 Secret Access Key",
		ImportErr:  "华为云 DNS 需要 HUAWEICLOUD_ACCESS_KEY_ID 和 HUAWEICLOUD_SECRET_ACCESS_KEY",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "huawei_access_key_id", Label: "Access Key ID", Secret: false, Required: true, EnvNames: []string{"HUAWEICLOUD_ACCESS_KEY_ID", "HUAWEI_ACCESS_KEY_ID"}},
			{Key: "huawei_secret_access_key", Label: "Secret Access Key", Secret: true, Required: true, EnvNames: []string{"HUAWEICLOUD_SECRET_ACCESS_KEY", "HUAWEI_SECRET_ACCESS_KEY"}},
			{Key: "huawei_region_id", Label: "Region ID", Secret: false, Required: false, EnvNames: []string{"HUAWEICLOUD_REGION_ID"}, Help: "不填默认 cn-south-1"},
		},
		renderACMEDNS: renderACMEDNSHuawei,
		renderDDNS:    renderDDNSHuawei,
	},
	{
		// RFC2136：自建 DNS（BIND/Knot/PowerDNS 等）的通用更新协议，
		// 一份实现覆盖所有支持 TSIG 的权威服务器。
		ID:         "rfc2136",
		Name:       "RFC2136 (自建 DNS)",
		Aliases:    []string{"rfc2136", "nsupdate"},
		MissingErr: "RFC2136 需要 Server、Key Name、Key 与 Key Alg",
		ImportErr:  "RFC2136 需要 RFC2136_SERVER、RFC2136_KEY_NAME、RFC2136_KEY 和 RFC2136_KEY_ALG",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "rfc2136_server", Label: "Server", Secret: false, Required: true, EnvNames: []string{"RFC2136_SERVER", "RFC2136_SERVER_ADDR"}, Help: "权威服务器地址，如 203.0.113.53"},
			{Key: "rfc2136_key_name", Label: "Key Name", Secret: false, Required: true, EnvNames: []string{"RFC2136_KEY_NAME", "RFC2136_TSIG_KEY_NAME"}, Help: "TSIG 密钥名"},
			{Key: "rfc2136_key", Label: "Key", Secret: true, Required: true, EnvNames: []string{"RFC2136_KEY", "RFC2136_TSIG_KEY"}, Help: "TSIG 密钥"},
			{Key: "rfc2136_key_alg", Label: "Key Alg", Secret: false, Required: true, EnvNames: []string{"RFC2136_KEY_ALG", "RFC2136_TSIG_KEY_ALG"}, Help: "TSIG 算法，如 hmac-sha256"},
		},
		renderACMEDNS: renderACMEDNSRFC2136,
		renderDDNS:    renderDDNSRFC2136,
	},
	{
		ID:         "duckdns",
		Name:       "DuckDNS",
		Aliases:    []string{"duckdns"},
		MissingErr: "DuckDNS 需要 API Token",
		ImportErr:  "DuckDNS 需要 DUCKDNS_API_TOKEN",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "duckdns_token", Label: "API Token", Secret: true, Required: true, EnvNames: []string{"DUCKDNS_API_TOKEN", "DUCKDNS_TOKEN"}},
			{Key: "duckdns_override_domain", Label: "Override Domain", Secret: false, Required: false, EnvNames: []string{"DUCKDNS_OVERRIDE_DOMAIN"}, Help: "需要覆盖的 DuckDNS 子域"},
			{Key: "duckdns_resolver", Label: "Resolver", Secret: false, Required: false, EnvNames: []string{"DUCKDNS_RESOLVER"}, Help: "用于查询的解析器"},
		},
		renderACMEDNS: renderACMEDNSDuckDNS,
		renderDDNS:    renderDDNSDuckDNS,
	},
	{
		ID:         "dynu",
		Name:       "Dynu",
		Aliases:    []string{"dynu"},
		MissingErr: "Dynu 需要 API Token 与 Own Domain",
		ImportErr:  "Dynu 需要 DYNU_API_TOKEN 和 DYNU_OWN_DOMAIN",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "dynu_token", Label: "API Token", Secret: true, Required: true, EnvNames: []string{"DYNU_API_TOKEN", "DYNU_TOKEN"}},
			{Key: "dynu_own_domain", Label: "Own Domain", Secret: false, Required: true, EnvNames: []string{"DYNU_OWN_DOMAIN"}, Help: "Dynu 账号下的域名"},
		},
		renderACMEDNS: renderACMEDNSDynu,
		renderDDNS:    renderDDNSDynu,
	},
	{
		ID:         "namecheap",
		Name:       "Namecheap",
		Aliases:    []string{"namecheap"},
		MissingErr: "Namecheap 需要 API Key 与 User",
		ImportErr:  "Namecheap 需要 NAMECHEAP_API_KEY 和 NAMECHEAP_USER",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "namecheap_api_key", Label: "API Key", Secret: false, Required: true, EnvNames: []string{"NAMECHEAP_API_KEY"}},
			{Key: "namecheap_user", Label: "User", Secret: false, Required: true, EnvNames: []string{"NAMECHEAP_USER"}},
			{Key: "namecheap_api_endpoint", Label: "API Endpoint", Secret: false, Required: false, EnvNames: []string{"NAMECHEAP_API_ENDPOINT"}, Help: "默认 https://api.namecheap.com/xml.response"},
			{Key: "namecheap_client_ip", Label: "Client IP", Secret: false, Required: false, EnvNames: []string{"NAMECHEAP_CLIENT_IP"}, Help: "Namecheap 白名单 IP"},
		},
		renderACMEDNS: renderACMEDNSNamecheap,
		renderDDNS:    renderDDNSNamecheap,
	},
	{
		ID:         "porkbun",
		Name:       "Porkbun",
		Aliases:    []string{"porkbun"},
		MissingErr: "Porkbun 需要 API Key 与 API Secret Key",
		ImportErr:  "Porkbun 需要 PORKBUN_API_KEY 和 PORKBUN_API_SECRET_KEY",
		DNS:        true,
		DDNS:       true,
		Registrar:  true,
		Fields: []providerField{
			{Key: "porkbun_api_key", Label: "API Key", Secret: false, Required: true, EnvNames: []string{"PORKBUN_API_KEY"}},
			{Key: "porkbun_api_secret_key", Label: "API Secret Key", Secret: true, Required: true, EnvNames: []string{"PORKBUN_API_SECRET_KEY"}},
		},
		renderACMEDNS: renderACMEDNSPorkbun,
		renderDDNS:    renderDDNSPorkbun,
		fetchExpiry:   fetchPorkbunDomainExpiry,
		renewDomain:   renewPorkbunDomain,
	},
	{
		ID:         "desec",
		Name:       "deSEC",
		Aliases:    []string{"desec"},
		MissingErr: "deSEC 需要 Token",
		ImportErr:  "deSEC 需要 DESEC_TOKEN",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "desec_token", Label: "Token", Secret: true, Required: true, EnvNames: []string{"DESEC_TOKEN"}},
		},
		renderACMEDNS: renderACMEDNSDeSEC,
		renderDDNS:    renderDDNSDeSEC,
	},
	{
		ID:          "digitalplat",
		Name:        "DigitalPlat",
		Aliases:     []string{"digitalplat", "digitalplatform"},
		MissingErr:  "DigitalPlat 需要 API Key",
		Registrar:   true,
		HostedZones: digitalPlatHostedZones,
		Fields: []providerField{
			{Key: "digital_plat_api_key", Label: "API Key", Secret: true, Required: true, EnvNames: []string{"DIGITALPLAT_API_KEY"}},
		},
		fetchExpiry: fetchDigitalPlatExpiryForCredential,
	},
	{
		ID:          "dnshe",
		Name:        "DNSHE",
		Aliases:     []string{"dnshe"},
		MissingErr:  "DNSHE 需要 API Key 和 API Secret",
		Registrar:   true,
		Renewable:   true,
		HostedZones: dnsheHostedZones,
		Fields: []providerField{
			{Key: "dnshe_api_key", Label: "API Key", Secret: true, Required: true, EnvNames: []string{"DNSHE_API_KEY"}},
			{Key: "dnshe_api_secret", Label: "API Secret", Secret: true, Required: true, EnvNames: []string{"DNSHE_API_SECRET"}},
		},
		fetchExpiry: fetchDNSHEDomainExpiry,
		renewDomain: renewDNSHEDomainWithCred,
	},
	{
		// Route 53：AWS 托管 DNS。region 必填，密钥与 profile 二选一。
		ID:         "route53",
		Name:       "Route 53",
		Aliases:    []string{"route53", "aws"},
		MissingErr: "Route 53 需要 Region、Access Key ID 与 Secret Access Key",
		ImportErr:  "Route 53 需要 ROUTE53_REGION、ROUTE53_ACCESS_KEY_ID 和 ROUTE53_SECRET_ACCESS_KEY",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "route53_region", Label: "Region", Secret: false, Required: true, EnvNames: []string{"ROUTE53_REGION", "AWS_REGION"}},
			{Key: "route53_access_key_id", Label: "Access Key ID", Secret: false, Required: true, EnvNames: []string{"ROUTE53_ACCESS_KEY_ID", "AWS_ACCESS_KEY_ID"}},
			{Key: "route53_secret_access_key", Label: "Secret Access Key", Secret: true, Required: true, EnvNames: []string{"ROUTE53_SECRET_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY"}},
			{Key: "route53_session_token", Label: "Session Token", Secret: true, Required: false, EnvNames: []string{"ROUTE53_SESSION_TOKEN", "AWS_SESSION_TOKEN"}},
			{Key: "route53_profile", Label: "Profile", Secret: false, Required: false, EnvNames: []string{"ROUTE53_PROFILE", "AWS_PROFILE"}, Help: "用 AWS profile 时可省略上面的密钥"},
			{Key: "route53_hosted_zone_id", Label: "Hosted Zone ID", Secret: false, Required: false, EnvNames: []string{"ROUTE53_HOSTED_ZONE_ID"}},
		},
		renderACMEDNS: renderACMEDNSRoute53,
		renderDDNS:    renderDDNSRoute53,
	},
	{
		ID:         "hetzner",
		Name:       "Hetzner Cloud",
		Aliases:    []string{"hetzner"},
		MissingErr: "Hetzner Cloud 需要 API Token",
		ImportErr:  "Hetzner Cloud 需要 HETZNER_API_TOKEN",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "hetzner_api_token", Label: "API Token", Secret: true, Required: true, EnvNames: []string{"HETZNER_API_TOKEN"}},
		},
		renderACMEDNS: renderACMEDNSHetzner,
		renderDDNS:    renderDDNSHetzner,
	},
	{
		ID:         "gandi",
		Name:       "Gandi",
		Aliases:    []string{"gandi"},
		MissingErr: "Gandi 需要 Bearer Token",
		ImportErr:  "Gandi 需要 GANDI_BEARER_TOKEN",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "gandi_bearer_token", Label: "Bearer Token", Secret: true, Required: true, EnvNames: []string{"GANDI_BEARER_TOKEN"}},
			{Key: "gandi_sharing_id", Label: "Sharing ID", Secret: false, Required: false, EnvNames: []string{"GANDI_SHARING_ID"}, Help: "共享域时需要"},
		},
		renderACMEDNS: renderACMEDNSGandi,
		renderDDNS:    renderDDNSGandi,
	},
	{
		ID:         "ovh",
		Name:       "OVH",
		Aliases:    []string{"ovh"},
		MissingErr: "OVH 需要 Application Key、Application Secret 与 Consumer Key",
		ImportErr:  "OVH 需要 OVH_APPLICATION_KEY、OVH_APPLICATION_SECRET 和 OVH_CONSUMER_KEY",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "ovh_endpoint", Label: "Endpoint", Secret: false, Required: false, EnvNames: []string{"OVH_ENDPOINT"}, Help: "默认 https://eu.api.ovh.com/1.0"},
			{Key: "ovh_application_key", Label: "Application Key", Secret: false, Required: true, EnvNames: []string{"OVH_APPLICATION_KEY"}},
			{Key: "ovh_application_secret", Label: "Application Secret", Secret: true, Required: true, EnvNames: []string{"OVH_APPLICATION_SECRET"}},
			{Key: "ovh_consumer_key", Label: "Consumer Key", Secret: true, Required: true, EnvNames: []string{"OVH_CONSUMER_KEY"}},
		},
		renderACMEDNS: renderACMEDNSOVH,
		renderDDNS:    renderDDNSOVH,
	},
	{
		ID:         "scaleway",
		Name:       "Scaleway",
		Aliases:    []string{"scaleway"},
		MissingErr: "Scaleway 需要 Secret Key 与 Organization ID",
		ImportErr:  "Scaleway 需要 SCW_SECRET_KEY 和 SCW_ORGANIZATION_ID",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "scaleway_secret_key", Label: "Secret Key", Secret: true, Required: true, EnvNames: []string{"SCW_SECRET_KEY"}},
			{Key: "scaleway_organization_id", Label: "Organization ID", Secret: false, Required: true, EnvNames: []string{"SCW_ORGANIZATION_ID"}},
		},
		renderACMEDNS: renderACMEDNSScaleway,
		renderDDNS:    renderDDNSScaleway,
	},
	{
		ID:         "linode",
		Name:       "Linode",
		Aliases:    []string{"linode", "axion"},
		MissingErr: "Linode 需要 API Token",
		ImportErr:  "Linode 需要 LINODE_API_TOKEN",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "linode_api_token", Label: "API Token", Secret: true, Required: true, EnvNames: []string{"LINODE_API_TOKEN"}},
			{Key: "linode_api_url", Label: "API URL", Secret: false, Required: false, EnvNames: []string{"LINODE_API_URL"}},
			{Key: "linode_api_version", Label: "API Version", Secret: false, Required: false, EnvNames: []string{"LINODE_API_VERSION"}},
		},
		renderACMEDNS: renderACMEDNSLinode,
		renderDDNS:    renderDDNSLinode,
	},
	{
		ID:         "godaddy",
		Name:       "GoDaddy",
		Aliases:    []string{"godaddy"},
		MissingErr: "GoDaddy 需要 API Token",
		ImportErr:  "GoDaddy 需要 GODADDY_API_TOKEN",
		DNS:        true,
		DDNS:       true,
		Fields: []providerField{
			{Key: "godaddy_api_token", Label: "API Token", Secret: true, Required: true, EnvNames: []string{"GODADDY_API_TOKEN"}},
		},
		renderACMEDNS: renderACMEDNSGoDaddy,
		renderDDNS:    renderDDNSGoDaddy,
	},
}

// providerSpecByID 按 id 精确查找。
func providerSpecByID(id string) *providerSpec {
	id = strings.ToLower(strings.TrimSpace(id))
	for i := range providerSpecs {
		if providerSpecs[i].ID == id {
			return &providerSpecs[i]
		}
	}
	return nil
}

// providerSpecByRegistrarAlias 按域名商名称查找，支持别名与历史写法。
func providerSpecByRegistrarAlias(provider string) *providerSpec {
	raw := strings.TrimSpace(provider)
	if raw == "" {
		return nil
	}
	key := strings.ToLower(strings.NewReplacer(" ", "", "-", "", "_", "").Replace(raw))
	for i := range providerSpecs {
		if providerSpecs[i].ID == key {
			return &providerSpecs[i]
		}
		for _, alias := range providerSpecs[i].Aliases {
			if alias == key {
				return &providerSpecs[i]
			}
		}
	}
	return nil
}

// dnsProviderSpecs 返回可作为 DNS 凭据的提供商（前端下拉框顺序）。
func dnsProviderSpecs() []*providerSpec {
	out := []*providerSpec{}
	for i := range providerSpecs {
		if providerSpecs[i].DNS {
			out = append(out, &providerSpecs[i])
		}
	}
	return out
}

// registrarProviderSpecs 返回可用于域名监控的提供商。
func registrarProviderSpecs() []*providerSpec {
	out := []*providerSpec{}
	for i := range providerSpecs {
		if providerSpecs[i].Registrar {
			out = append(out, &providerSpecs[i])
		}
	}
	return out
}

func providerHostedZones() []string {
	out := []string{}
	for _, spec := range registrarProviderSpecs() {
		out = append(out, spec.HostedZones...)
	}
	return out
}

// credentialSecretValue 读取凭据密钥，统一屏蔽历史字段与 Secrets 的差异。
func credentialSecretValue(c Credential, key string) string {
	switch key {
	case "aliyun_key":
		return c.AliyunKey
	case "aliyun_secret":
		return c.AliyunSecret
	case "cf_token":
		return c.CFToken
	case "dnspod_token":
		return c.DNSPodToken
	case "he_api_key":
		return c.HEAPIKey
	case "digital_plat_api_key":
		return c.DigitalPlatAPIKey
	case "dnshe_api_key":
		return c.DNSHEAPIKey
	case "dnshe_api_secret":
		return c.DNSHEAPISecret
	}
	if c.Secrets == nil {
		return ""
	}
	return c.Secrets[key]
}

// credentialSetSecret 写入凭据密钥。历史 provider 写回类型化字段以保持文件格式不变。
func credentialSetSecret(c *Credential, key string, value string) {
	switch key {
	case "aliyun_key":
		c.AliyunKey = value
		return
	case "aliyun_secret":
		c.AliyunSecret = value
		return
	case "cf_token":
		c.CFToken = value
		return
	case "dnspod_token":
		c.DNSPodToken = value
		return
	case "he_api_key":
		c.HEAPIKey = value
		return
	case "digital_plat_api_key":
		c.DigitalPlatAPIKey = value
		return
	case "dnshe_api_key":
		c.DNSHEAPIKey = value
		return
	case "dnshe_api_secret":
		c.DNSHEAPISecret = value
		return
	}
	if c.Secrets == nil {
		c.Secrets = map[string]string{}
	}
	c.Secrets[key] = value
}

// credentialSecrets 返回该 provider 全部字段的当前值（键为 providerField.Key）。
func credentialSecrets(c Credential) map[string]string {
	spec := providerSpecByID(c.Provider)
	if spec == nil {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, f := range spec.Fields {
		out[f.Key] = credentialSecretValue(c, f.Key)
	}
	return out
}

// credentialFields 返回该 provider 的字段描述，供前端按 schema 渲染。
func credentialFields(provider string) []providerField {
	spec := providerSpecByID(provider)
	if spec == nil {
		return nil
	}
	return spec.Fields
}

// credentialIsComplete 判断必填字段是否齐备。
func credentialIsComplete(c Credential) bool {
	spec := providerSpecByID(c.Provider)
	if spec == nil {
		return false
	}
	for _, f := range spec.Fields {
		if f.Required && strings.TrimSpace(credentialSecretValue(c, f.Key)) == "" {
			return false
		}
	}
	return true
}

// credentialTemplate 生成前端展示用的 env 文本模板（与历史输出保持一致）。
func credentialTemplate(spec *providerSpec) string {
	if spec == nil {
		return ""
	}
	lines := []string{}
	for _, f := range spec.Fields {
		if len(f.EnvNames) == 0 {
			continue
		}
		lines = append(lines, f.EnvNames[0]+"="+fPlaceholderText(f))
	}
	return strings.Join(lines, "\n")
}

// fPlaceholderText 取出字段用于模板的占位文本。
func fPlaceholderText(f providerField) string {
	text := f.Placeholder
	if strings.TrimSpace(text) == "" {
		text = f.Label
	}
	if !strings.HasPrefix(text, "<") {
		text = "<" + text + ">"
	}
	return text
}

// providerIsDNSCredential 是否可作为 DNS（ACME DNS-01）凭据。
func providerIsDNSCredential(provider string) bool {
	spec := providerSpecByID(provider)
	return spec != nil && spec.DNS
}

// providerDisplayName 展示名，未知 provider 原样返回（保持历史行为）。
func providerDisplayName(provider string) string {
	if spec := providerSpecByID(provider); spec != nil {
		return spec.Name
	}
	return strings.TrimSpace(provider)
}

// credentialReuseKeyBySpec 用于「复用凭据」分组；必填字段缺失或非 DNS 凭据返回空串。
func credentialReuseKeyBySpec(c Credential) string {
	spec := providerSpecByID(c.Provider)
	if spec == nil || !spec.DNS {
		return ""
	}
	parts := []string{c.Provider}
	for _, f := range spec.Fields {
		value := strings.TrimSpace(credentialSecretValue(c, f.Key))
		if f.Required && value == "" {
			return ""
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, ":")
}

// credContentEqualBySpec 比较两条凭据的提供商与全部字段。
func credContentEqualBySpec(a Credential, b Credential) bool {
	if a.Provider != b.Provider {
		return false
	}
	spec := providerSpecByID(a.Provider)
	if spec == nil {
		return false
	}
	for _, f := range spec.Fields {
		if credentialSecretValue(a, f.Key) != credentialSecretValue(b, f.Key) {
			return false
		}
	}
	return true
}

// validateCredentialSecretBySpec 按注册表校验必填密钥。
func validateCredentialSecretBySpec(c Credential) error {
	spec := providerSpecByID(c.Provider)
	if spec == nil {
		return fmt.Errorf("未知提供商：%s", c.Provider)
	}
	for _, f := range spec.Fields {
		if !f.Required {
			continue
		}
		if strings.TrimSpace(credentialSecretValue(c, f.Key)) == "" {
			if spec.MissingErr != "" {
				return errors.New(spec.MissingErr)
			}
			return fmt.Errorf("%s 需要 %s", spec.Name, f.Label)
		}
	}
	return nil
}

// credentialFromDNSProvider 解析 env 文本形式的凭据（历史导入路径）。
func credentialFromDNSProvider(provider string, raw string) (Credential, error) {
	spec := providerSpecByID(provider)
	if spec == nil {
		return Credential{}, fmt.Errorf("不支持的 DNS 提供商：%s", provider)
	}
	values := parseCredentialLines(raw)
	cred := Credential{Provider: spec.ID}
	for _, f := range spec.Fields {
		value := ""
		for _, name := range f.EnvNames {
			if value = strings.TrimSpace(values[name]); value != "" {
				break
			}
		}
		if value == "" && f.Required && len(f.EnvNames) > 0 {
			if spec.ImportErr != "" {
				return Credential{}, errors.New(spec.ImportErr)
			}
			return Credential{}, fmt.Errorf("%s 需要 %s", spec.Name, f.EnvNames[0])
		}
		credentialSetSecret(&cred, f.Key, value)
	}
	return cred, nil
}

// syncErrorLabel 同步失败文案。
func syncErrorLabel(spec *providerSpec) string {
	if spec != nil && strings.TrimSpace(spec.syncErrorText) != "" {
		return spec.syncErrorText
	}
	return "记录检查失败"
}

// syncMissingLabel 未找到记录文案。
func syncMissingLabel(spec *providerSpec) string {
	if spec != nil && strings.TrimSpace(spec.syncMissingText) != "" {
		return spec.syncMissingText
	}
	return "未找到匹配的 A/AAAA 记录"
}

//
// ===== DNS 提供商渲染 =====
//

func renderACMEDNSAliyun(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns alidns {\n", indent)
	fmt.Fprintf(b, "%s        access_key_id %s\n", indent, caddyQuote(cred.AliyunKey))
	fmt.Fprintf(b, "%s        access_key_secret %s\n", indent, caddyQuote(cred.AliyunSecret))
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderACMEDNSCloudflare(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns cloudflare %s\n", indent, caddyQuote(cred.CFToken))
}

func renderACMEDNSDNSPod(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns dnspod %s\n", indent, caddyQuote(cred.DNSPodToken))
}

func renderACMEDNSHE(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns he {\n", indent)
	fmt.Fprintf(b, "%s        api_key %s\n", indent, caddyQuote(cred.HEAPIKey))
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSAliyun(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider alidns {\n")
	fmt.Fprintf(b, "        access_key_id %s\n", caddyQuote(cred.AliyunKey))
	fmt.Fprintf(b, "        access_key_secret %s\n", caddyQuote(cred.AliyunSecret))
	b.WriteString("    }\n")
	return nil
}

func renderDDNSCloudflare(b *strings.Builder, cred Credential) error {
	fmt.Fprintf(b, "    provider cloudflare %s\n", caddyQuote(cred.CFToken))
	return nil
}

func renderDDNSDNSPod(b *strings.Builder, cred Credential) error {
	fmt.Fprintf(b, "    provider dnspod %s\n", caddyQuote(cred.DNSPodToken))
	return nil
}

func renderDDNSHE(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider he {\n")
	fmt.Fprintf(b, "        api_key %s\n", caddyQuote(cred.HEAPIKey))
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSHuawei(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns huaweicloud {\n", indent)
	fmt.Fprintf(b, "%s        access_key_id %s\n", indent, caddyQuote(credentialSecretValue(cred, "huawei_access_key_id")))
	fmt.Fprintf(b, "%s        secret_access_key %s\n", indent, caddyQuote(credentialSecretValue(cred, "huawei_secret_access_key")))
	if region := strings.TrimSpace(credentialSecretValue(cred, "huawei_region_id")); region != "" {
		fmt.Fprintf(b, "%s        region_id %s\n", indent, caddyQuote(region))
	}
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSHuawei(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider huaweicloud {\n")
	fmt.Fprintf(b, "        access_key_id %s\n", caddyQuote(credentialSecretValue(cred, "huawei_access_key_id")))
	fmt.Fprintf(b, "        secret_access_key %s\n", caddyQuote(credentialSecretValue(cred, "huawei_secret_access_key")))
	if region := strings.TrimSpace(credentialSecretValue(cred, "huawei_region_id")); region != "" {
		fmt.Fprintf(b, "        region_id %s\n", caddyQuote(region))
	}
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSRFC2136(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns rfc2136 {\n", indent)
	fmt.Fprintf(b, "%s        server %s\n", indent, caddyQuote(credentialSecretValue(cred, "rfc2136_server")))
	fmt.Fprintf(b, "%s        key_name %s\n", indent, caddyQuote(credentialSecretValue(cred, "rfc2136_key_name")))
	fmt.Fprintf(b, "%s        key %s\n", indent, caddyQuote(credentialSecretValue(cred, "rfc2136_key")))
	fmt.Fprintf(b, "%s        key_alg %s\n", indent, caddyQuote(credentialSecretValue(cred, "rfc2136_key_alg")))
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSRFC2136(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider rfc2136 {\n")
	fmt.Fprintf(b, "        server %s\n", caddyQuote(credentialSecretValue(cred, "rfc2136_server")))
	fmt.Fprintf(b, "        key_name %s\n", caddyQuote(credentialSecretValue(cred, "rfc2136_key_name")))
	fmt.Fprintf(b, "        key %s\n", caddyQuote(credentialSecretValue(cred, "rfc2136_key")))
	fmt.Fprintf(b, "        key_alg %s\n", caddyQuote(credentialSecretValue(cred, "rfc2136_key_alg")))
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSDuckDNS(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns duckdns {\n", indent)
	fmt.Fprintf(b, "%s        api_token %s\n", indent, caddyQuote(credentialSecretValue(cred, "duckdns_token")))
	if domain := strings.TrimSpace(credentialSecretValue(cred, "duckdns_override_domain")); domain != "" {
		fmt.Fprintf(b, "%s        override_domain %s\n", indent, caddyQuote(domain))
	}
	if resolver := strings.TrimSpace(credentialSecretValue(cred, "duckdns_resolver")); resolver != "" {
		fmt.Fprintf(b, "%s        resolver %s\n", indent, caddyQuote(resolver))
	}
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSDuckDNS(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider duckdns {\n")
	fmt.Fprintf(b, "        api_token %s\n", caddyQuote(credentialSecretValue(cred, "duckdns_token")))
	if domain := strings.TrimSpace(credentialSecretValue(cred, "duckdns_override_domain")); domain != "" {
		fmt.Fprintf(b, "        override_domain %s\n", caddyQuote(domain))
	}
	if resolver := strings.TrimSpace(credentialSecretValue(cred, "duckdns_resolver")); resolver != "" {
		fmt.Fprintf(b, "        resolver %s\n", caddyQuote(resolver))
	}
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSDynu(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns dynu {\n", indent)
	fmt.Fprintf(b, "%s        api_token %s\n", indent, caddyQuote(credentialSecretValue(cred, "dynu_token")))
	fmt.Fprintf(b, "%s        own_domain %s\n", indent, caddyQuote(credentialSecretValue(cred, "dynu_own_domain")))
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSDynu(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider dynu {\n")
	fmt.Fprintf(b, "        api_token %s\n", caddyQuote(credentialSecretValue(cred, "dynu_token")))
	fmt.Fprintf(b, "        own_domain %s\n", caddyQuote(credentialSecretValue(cred, "dynu_own_domain")))
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSNamecheap(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns namecheap {\n", indent)
	fmt.Fprintf(b, "%s        api_key %s\n", indent, caddyQuote(credentialSecretValue(cred, "namecheap_api_key")))
	fmt.Fprintf(b, "%s        user %s\n", indent, caddyQuote(credentialSecretValue(cred, "namecheap_user")))
	if endpoint := strings.TrimSpace(credentialSecretValue(cred, "namecheap_api_endpoint")); endpoint != "" {
		fmt.Fprintf(b, "%s        api_endpoint %s\n", indent, caddyQuote(endpoint))
	}
	if ip := strings.TrimSpace(credentialSecretValue(cred, "namecheap_client_ip")); ip != "" {
		fmt.Fprintf(b, "%s        client_ip %s\n", indent, caddyQuote(ip))
	}
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSNamecheap(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider namecheap {\n")
	fmt.Fprintf(b, "        api_key %s\n", caddyQuote(credentialSecretValue(cred, "namecheap_api_key")))
	fmt.Fprintf(b, "        user %s\n", caddyQuote(credentialSecretValue(cred, "namecheap_user")))
	if endpoint := strings.TrimSpace(credentialSecretValue(cred, "namecheap_api_endpoint")); endpoint != "" {
		fmt.Fprintf(b, "        api_endpoint %s\n", caddyQuote(endpoint))
	}
	if ip := strings.TrimSpace(credentialSecretValue(cred, "namecheap_client_ip")); ip != "" {
		fmt.Fprintf(b, "        client_ip %s\n", caddyQuote(ip))
	}
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSPorkbun(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns porkbun {\n", indent)
	fmt.Fprintf(b, "%s        api_key %s\n", indent, caddyQuote(credentialSecretValue(cred, "porkbun_api_key")))
	fmt.Fprintf(b, "%s        api_secret_key %s\n", indent, caddyQuote(credentialSecretValue(cred, "porkbun_api_secret_key")))
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSPorkbun(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider porkbun {\n")
	fmt.Fprintf(b, "        api_key %s\n", caddyQuote(credentialSecretValue(cred, "porkbun_api_key")))
	fmt.Fprintf(b, "        api_secret_key %s\n", caddyQuote(credentialSecretValue(cred, "porkbun_api_secret_key")))
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSDeSEC(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns desec {\n", indent)
	fmt.Fprintf(b, "%s        token %s\n", indent, caddyQuote(credentialSecretValue(cred, "desec_token")))
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSDeSEC(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider desec {\n")
	fmt.Fprintf(b, "        token %s\n", caddyQuote(credentialSecretValue(cred, "desec_token")))
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSRoute53(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns route53 {\n", indent)
	fmt.Fprintf(b, "%s        region %s\n", indent, caddyQuote(credentialSecretValue(cred, "route53_region")))
	fmt.Fprintf(b, "%s        access_key_id %s\n", indent, caddyQuote(credentialSecretValue(cred, "route53_access_key_id")))
	fmt.Fprintf(b, "%s        secret_access_key %s\n", indent, caddyQuote(credentialSecretValue(cred, "route53_secret_access_key")))
	if token := strings.TrimSpace(credentialSecretValue(cred, "route53_session_token")); token != "" {
		fmt.Fprintf(b, "%s        session_token %s\n", indent, caddyQuote(token))
	}
	if profile := strings.TrimSpace(credentialSecretValue(cred, "route53_profile")); profile != "" {
		fmt.Fprintf(b, "%s        profile %s\n", indent, caddyQuote(profile))
	}
	if zone := strings.TrimSpace(credentialSecretValue(cred, "route53_hosted_zone_id")); zone != "" {
		fmt.Fprintf(b, "%s        hosted_zone_id %s\n", indent, caddyQuote(zone))
	}
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSRoute53(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider route53 {\n")
	fmt.Fprintf(b, "        region %s\n", caddyQuote(credentialSecretValue(cred, "route53_region")))
	fmt.Fprintf(b, "        access_key_id %s\n", caddyQuote(credentialSecretValue(cred, "route53_access_key_id")))
	fmt.Fprintf(b, "        secret_access_key %s\n", caddyQuote(credentialSecretValue(cred, "route53_secret_access_key")))
	if token := strings.TrimSpace(credentialSecretValue(cred, "route53_session_token")); token != "" {
		fmt.Fprintf(b, "        session_token %s\n", caddyQuote(token))
	}
	if profile := strings.TrimSpace(credentialSecretValue(cred, "route53_profile")); profile != "" {
		fmt.Fprintf(b, "        profile %s\n", caddyQuote(profile))
	}
	if zone := strings.TrimSpace(credentialSecretValue(cred, "route53_hosted_zone_id")); zone != "" {
		fmt.Fprintf(b, "        hosted_zone_id %s\n", caddyQuote(zone))
	}
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSHetzner(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns hetzner {\n", indent)
	fmt.Fprintf(b, "%s        api_token %s\n", indent, caddyQuote(credentialSecretValue(cred, "hetzner_api_token")))
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSHetzner(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider hetzner {\n")
	fmt.Fprintf(b, "        api_token %s\n", caddyQuote(credentialSecretValue(cred, "hetzner_api_token")))
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSGandi(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns gandi {\n", indent)
	fmt.Fprintf(b, "%s        bearer_token %s\n", indent, caddyQuote(credentialSecretValue(cred, "gandi_bearer_token")))
	if sharing := strings.TrimSpace(credentialSecretValue(cred, "gandi_sharing_id")); sharing != "" {
		fmt.Fprintf(b, "%s        sharing_id %s\n", indent, caddyQuote(sharing))
	}
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSGandi(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider gandi {\n")
	fmt.Fprintf(b, "        bearer_token %s\n", caddyQuote(credentialSecretValue(cred, "gandi_bearer_token")))
	if sharing := strings.TrimSpace(credentialSecretValue(cred, "gandi_sharing_id")); sharing != "" {
		fmt.Fprintf(b, "        sharing_id %s\n", caddyQuote(sharing))
	}
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSOVH(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns ovh {\n", indent)
	if endpoint := strings.TrimSpace(credentialSecretValue(cred, "ovh_endpoint")); endpoint != "" {
		fmt.Fprintf(b, "%s        endpoint %s\n", indent, caddyQuote(endpoint))
	}
	fmt.Fprintf(b, "%s        application_key %s\n", indent, caddyQuote(credentialSecretValue(cred, "ovh_application_key")))
	fmt.Fprintf(b, "%s        application_secret %s\n", indent, caddyQuote(credentialSecretValue(cred, "ovh_application_secret")))
	fmt.Fprintf(b, "%s        consumer_key %s\n", indent, caddyQuote(credentialSecretValue(cred, "ovh_consumer_key")))
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSOVH(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider ovh {\n")
	if endpoint := strings.TrimSpace(credentialSecretValue(cred, "ovh_endpoint")); endpoint != "" {
		fmt.Fprintf(b, "        endpoint %s\n", caddyQuote(endpoint))
	}
	fmt.Fprintf(b, "        application_key %s\n", caddyQuote(credentialSecretValue(cred, "ovh_application_key")))
	fmt.Fprintf(b, "        application_secret %s\n", caddyQuote(credentialSecretValue(cred, "ovh_application_secret")))
	fmt.Fprintf(b, "        consumer_key %s\n", caddyQuote(credentialSecretValue(cred, "ovh_consumer_key")))
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSScaleway(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns scaleway {\n", indent)
	fmt.Fprintf(b, "%s        secret_key %s\n", indent, caddyQuote(credentialSecretValue(cred, "scaleway_secret_key")))
	fmt.Fprintf(b, "%s        organization_id %s\n", indent, caddyQuote(credentialSecretValue(cred, "scaleway_organization_id")))
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSScaleway(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider scaleway {\n")
	fmt.Fprintf(b, "        secret_key %s\n", caddyQuote(credentialSecretValue(cred, "scaleway_secret_key")))
	fmt.Fprintf(b, "        organization_id %s\n", caddyQuote(credentialSecretValue(cred, "scaleway_organization_id")))
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSLinode(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns linode {\n", indent)
	fmt.Fprintf(b, "%s        api_token %s\n", indent, caddyQuote(credentialSecretValue(cred, "linode_api_token")))
	if url := strings.TrimSpace(credentialSecretValue(cred, "linode_api_url")); url != "" {
		fmt.Fprintf(b, "%s        api_url %s\n", indent, caddyQuote(url))
	}
	if version := strings.TrimSpace(credentialSecretValue(cred, "linode_api_version")); version != "" {
		fmt.Fprintf(b, "%s        api_version %s\n", indent, caddyQuote(version))
	}
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSLinode(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider linode {\n")
	fmt.Fprintf(b, "        api_token %s\n", caddyQuote(credentialSecretValue(cred, "linode_api_token")))
	if url := strings.TrimSpace(credentialSecretValue(cred, "linode_api_url")); url != "" {
		fmt.Fprintf(b, "        api_url %s\n", caddyQuote(url))
	}
	if version := strings.TrimSpace(credentialSecretValue(cred, "linode_api_version")); version != "" {
		fmt.Fprintf(b, "        api_version %s\n", caddyQuote(version))
	}
	b.WriteString("    }\n")
	return nil
}

func renderACMEDNSGoDaddy(b *strings.Builder, indent string, cred Credential) {
	fmt.Fprintf(b, "%s    dns godaddy {\n", indent)
	fmt.Fprintf(b, "%s        api_token %s\n", indent, caddyQuote(credentialSecretValue(cred, "godaddy_api_token")))
	fmt.Fprintf(b, "%s    }\n", indent)
}

func renderDDNSGoDaddy(b *strings.Builder, cred Credential) error {
	b.WriteString("    provider godaddy {\n")
	fmt.Fprintf(b, "        api_token %s\n", caddyQuote(credentialSecretValue(cred, "godaddy_api_token")))
	b.WriteString("    }\n")
	return nil
}

//
// ===== DDNS 状态巡检适配 =====
//

// syncCloudflareHostRecord 适配 Cloudflare 的主机名同步签名。
func syncCloudflareHostRecord(_ npmDynamicDNS, host string, cred Credential, publicIPs []string, wantIPv4 bool, wantIPv6 bool) ([]string, error) {
	return syncCloudflareDynamicDNSRecord(host, cred, publicIPs, wantIPv4, wantIPv6)
}

// syncHEHostRecord 适配 Hurricane Electric 的主机名同步签名（需要解析器配置）。
func syncHEHostRecord(item npmDynamicDNS, host string, cred Credential, publicIPs []string, wantIPv4 bool, wantIPv6 bool) ([]string, error) {
	return syncHEDynamicDNSRecord(host, cred, publicIPs, item.Resolvers, wantIPv4, wantIPv6)
}

// fetchDigitalPlatExpiryForCredential 适配 DigitalPlat 的到期查询签名。
func fetchDigitalPlatExpiryForCredential(domain string, cred Credential) (time.Time, error) {
	return fetchDigitalPlatDomainExpiry(domain, cred.DigitalPlatAPIKey)
}
