package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/publicsuffix"
)

var (
	sitesDir                  = "/config/sites"
	managedCertPrefix         = "__cert_"
	wildcardGroupPrefix       = "__wildcard_group_"
	credentialPath            = "/config/credentials.json"
	issuerCredentialPath      = "/config/issuer-credentials.json"
	accessListPath            = "/config/access-lists.json"
	wakeDevicePath            = "/config/wake-devices.json"
	autheliaConfigPath        = "/config/authelia.json"
	authentikConfigPath       = "/config/authentik.json"
	streamPath                = "/config/streams.json"
	dynamicDNSPath            = "/config/dynamic-dns.json"
	domainMonitorPath         = "/config/domain-monitor.json"
	notificationChannelPath   = "/config/notification-channels.json"
	notificationStatePath     = "/config/notification-state.json"
	settingsPath              = "/config/settings.json"
	layer4ConfPath            = "/etc/caddy/global.d/layer4.conf"
	dynamicDNSConfPath        = "/etc/caddy/global.d/dynamic-dns.conf"
	customCertMeta            = "/config/custom-certs.json"
	customCertsDir            = "/config/custom-certs"
	auditLogPath              = "/config/audit-log.json"
	caddyfile                 = "/etc/caddy/Caddyfile"
	legacyAcmeFile            = "/etc/caddy/global.d/acme.conf"
	caddyDataDir              = "/data/caddy"
	caddyCertsDir             = "/data/caddy/certificates"
	logFile                   = "/config/caddy/caddy.log"
	listen                    = "127.0.0.1:9001"
	caddyAdmin                = "http://127.0.0.1:2019"
	certLogListen             = "127.0.0.1:9002" // caddy net log writer 推送 TLS 事件到这里
	ddnsLogListen             = "127.0.0.1:9003" // caddy net log writer 推送动态 DNS 事件到这里
	publicIPv4Endpoint        = "https://api.ipify.org"
	publicIPv6Endpoint        = "https://api64.ipify.org"
	publicIPFallbackEndpoints = []string{"https://ifconfig.me/ip", "https://ipinfo.io/ip", "https://api.ip.sb/ip", "https://icanhazip.com"}
	aliyunDNSAPIBaseURL       = "https://alidns.aliyuncs.com/"
	cloudflareAPIBaseURL      = "https://api.cloudflare.com/client/v4"
	confSuffix                = ".conf"
	metaSuffix                = ".json"
	probeTimeout              = 2 * time.Second
	domainMonitorTimeout      = 3 * time.Second
	domainMonitorRDAPTimeout  = 8 * time.Second
	domainMonitorAPITimeout   = 8 * time.Second
	digitalPlatAPIBaseURL     = "https://domain-api.digitalplat.org/api/v1"
	dnsheAPIBaseURL           = "https://api005.dnshe.com/index.php?m=domain_hub"
)

var (
	digitalPlatHostedZones = []string{"us.kg", "dpdns.org", "qzz.io", "xx.kg", "qd.je"}
	dnsheHostedZones       = []string{"cc.cd", "ccwu.cc", "bbroot.com"}
	unmanagedHostedZones   = []string{"kdns.fr"}
)

var errDomainRegistrationExpiryUnavailable = errors.New("域名注册到期信息不可用")

// Credential 一条可复用凭据。同一 provider 允许多条（多账号）。
type Credential struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Provider          string       `json:"provider"` // alidns | cloudflare | dnspod | he | digitalplat | dnshe
	AliyunKey         string       `json:"aliyun_key,omitempty"`
	AliyunSecret      string       `json:"aliyun_secret,omitempty"`
	CFToken           string       `json:"cf_token,omitempty"`
	DNSPodToken       string       `json:"dnspod_token,omitempty"`
	HEAPIKey          string       `json:"he_api_key,omitempty"`
	DigitalPlatAPIKey string       `json:"digital_plat_api_key,omitempty"`
	DNSHEAPIKey       string       `json:"dnshe_api_key,omitempty"`
	DNSHEAPISecret    string       `json:"dnshe_api_secret,omitempty"`
	Issuer            IssuerConfig `json:"issuer,omitempty"`
}

type IssuerConfig struct {
	Provider      string `json:"provider,omitempty"`
	CADirectory   string `json:"ca_directory,omitempty"`
	EABKeyID      string `json:"eab_key_id,omitempty"`
	EABMACKey     string `json:"eab_mac_key,omitempty"`
	ZeroSSLAPIKey string `json:"zerossl_api_key,omitempty"`
}

// ProxyLocation 反代路径规则，允许同一站点下多个路径指向不同后端。
type ProxyLocation struct {
	Path          string `json:"path"`
	ForwardScheme string `json:"forward_scheme,omitempty"`
	ForwardHost   string `json:"forward_host"`
	ForwardPort   int    `json:"forward_port"`
	ForwardPath   string `json:"forward_path,omitempty"` // 目标路径重写，如 /B 表示 /A → /B
	Backend       string `json:"backend,omitempty"`      // 计算字段: scheme://host:port[/path]
}

// Site 反代条目。
type Site struct {
	CreatedOn                  string               `json:"created_on,omitempty"`
	ModifiedOn                 string               `json:"modified_on,omitempty"`
	Name                       string               `json:"name"`
	ServiceName                string               `json:"service_name,omitempty"`
	Kind                       string               `json:"kind,omitempty"` // proxy | redirection | dead
	Domain                     string               `json:"domain"`
	Path                       string               `json:"path,omitempty"`
	Backend                    string               `json:"backend"`
	Locations                  []ProxyLocation        `json:"locations,omitempty"`
	RedirectURL                string               `json:"redirect_url,omitempty"`
	RedirectCode               int                  `json:"redirect_code,omitempty"`
	PreservePath               bool                 `json:"preserve_path,omitempty"`
	Headers                    string               `json:"headers,omitempty"`
	AccessListID               int                  `json:"access_list_id,omitempty"`
	Wildcard                   bool                 `json:"wildcard,omitempty"`
	NoTLS                      bool                 `json:"no_tls,omitempty"`           // true = 仅 HTTP，不申请证书
	ChallengePref              string               `json:"challenge_pref,omitempty"`   // "http" | "dns"，默认 http
	CredentialID               string               `json:"credential_id,omitempty"`    // DNS-01 或 wildcard 必填
	CertificateMode            string               `json:"certificate_mode,omitempty"` // auto = 由 Caddy 为每个域名自动管理证书
	CertificateBindings        []CertificateBinding `json:"certificate_bindings,omitempty"`
	Issuer                     IssuerConfig         `json:"issuer,omitempty"`
	CustomCertFile             string               `json:"custom_cert_file,omitempty"`
	CustomKeyFile              string               `json:"custom_key_file,omitempty"`
	ForwardAuth                ForwardAuthConfig    `json:"forward_auth,omitempty"`
	UpstreamInsecureSkipVerify bool                 `json:"upstream_insecure_skip_verify,omitempty"`
	Disabled                   bool                 `json:"disabled,omitempty"`
	LastError                  string               `json:"last_error,omitempty"`
	LastErrorAt                time.Time            `json:"last_error_at,omitempty"`
}

type ForwardAuthConfig struct {
	Enabled     bool     `json:"enabled,omitempty"`
	Provider    string   `json:"provider,omitempty"`
	Upstream    string   `json:"upstream,omitempty"`
	URI         string   `json:"uri,omitempty"`
	CopyHeaders []string `json:"copy_headers,omitempty"`
	UseGlobal   bool     `json:"use_global,omitempty"`
}

type AutheliaConfig struct {
	Enabled     bool     `json:"enabled"`
	Upstream    string   `json:"upstream"`
	URI         string   `json:"uri"`
	CopyHeaders []string `json:"copy_headers"`
	FailOpen    bool     `json:"fail_open"`
	LoginURL    string   `json:"login_url"`
}

type SystemSettings struct {
	ACMEContactEmail string `json:"acme_contact_email"`
}

type AuthentikConfig struct {
	Enabled     bool     `json:"enabled"`
	Upstream    string   `json:"upstream"`
	URI         string   `json:"uri"`
	CopyHeaders []string `json:"copy_headers"`
}

type ForwardAuthGlobalConfig struct {
	Provider    string
	Enabled     bool
	Upstream    string
	URI         string
	CopyHeaders []string
	FailOpen    bool
	LoginURL    string
}

type forwardAuthProviderSpec struct {
	ID                string
	DisplayName       string
	DefaultUpstream   string
	DefaultURI        string
	DefaultCopyHeader []string
	SupportsFailOpen  bool
	SupportsLoginURL  bool
}

var defaultForwardAuthCopyHeaders = []string{"Remote-User", "Remote-Groups", "Remote-Email", "Remote-Name"}
var defaultAuthentikCopyHeaders = []string{"X-authentik-username", "X-authentik-groups", "X-authentik-email", "X-authentik-name", "X-authentik-uid", "X-authentik-jwt", "X-authentik-meta-jwks", "X-authentik-meta-outpost", "X-authentik-meta-provider", "X-authentik-meta-app", "X-authentik-meta-version"}
var forwardAuthProviderSpecs = map[string]forwardAuthProviderSpec{
	"authelia": {
		ID:                "authelia",
		DisplayName:       "Authelia",
		DefaultUpstream:   "http://authelia:9091",
		DefaultURI:        "/api/authz/forward-auth",
		DefaultCopyHeader: defaultForwardAuthCopyHeaders,
		SupportsFailOpen:  true,
		SupportsLoginURL:  true,
	},
	"authentik": {
		ID:                "authentik",
		DisplayName:       "authentik",
		DefaultUpstream:   "http://authentik-server:9000",
		DefaultURI:        "/outpost.goauthentik.io/auth/caddy",
		DefaultCopyHeader: defaultAuthentikCopyHeaders,
	},
}

var headerNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type CertificateBinding struct {
	Domain            string       `json:"domain"`
	CertificateID     int          `json:"certificate_id,omitempty"`
	CertificateDomain string       `json:"certificate_domain,omitempty"`
	Mode              string       `json:"mode,omitempty"` // auto | selected
	ChallengePref     string       `json:"challenge_pref,omitempty"`
	CredentialID      string       `json:"credential_id,omitempty"`
	Issuer            IssuerConfig `json:"issuer,omitempty"`
	Provider          string       `json:"provider,omitempty"`
	NiceName          string       `json:"nice_name,omitempty"`
	LastError         string       `json:"last_error,omitempty"`
	LastErrorAt       time.Time    `json:"last_error_at,omitempty"`
}

// placeholderMeta 通配符占位 site 的元数据。
type placeholderMeta struct {
	Domain       string       `json:"domain"`        // 形如 *.example.com
	CredentialID string       `json:"credential_id"` // 申请此通配符所用凭据
	Issuer       IssuerConfig `json:"issuer,omitempty"`
	Disabled     bool         `json:"disabled,omitempty"` // true = 仅保留备份元数据，不参与 Caddy 自动化配置
	LastError    string       `json:"last_error,omitempty"`
	LastErrorAt  time.Time    `json:"last_error_at,omitempty"`
	CreatedOn    string       `json:"created_on,omitempty"`
	ModifiedOn   string       `json:"modified_on,omitempty"`
}

type CertOverview struct {
	Domain         string       `json:"domain"`
	CreatedOn      string       `json:"created_on,omitempty"`
	ModifiedOn     string       `json:"modified_on,omitempty"`
	IsWildcard     bool         `json:"is_wildcard"`
	Status         string       `json:"status"` // issued | pending | expiring | expired | failed | disabled
	Issued         bool         `json:"issued"`
	Provider       string       `json:"provider,omitempty"`
	Issuer         string       `json:"issuer,omitempty"`
	NotAfter       time.Time    `json:"not_after,omitempty"`
	DaysLeft       int          `json:"days_left"`
	SignMethod     string       `json:"sign_method"` // HTTP-01 | DNS-01
	CredentialID   string       `json:"credential_id,omitempty"`
	CredentialName string       `json:"credential_name,omitempty"`
	LinkedSites    []string     `json:"linked_sites,omitempty"`
	LastError      string       `json:"last_error,omitempty"`
	LastErrorAt    time.Time    `json:"last_error_at,omitempty"`
	IssuerConfig   IssuerConfig `json:"-"`
}

type certRequestConfig struct {
	Provider     string
	SignMethod   string
	CredentialID string
	Issuer       IssuerConfig
}

type certSAN struct {
	NotAfter  time.Time
	NotBefore time.Time
	Issuer    string
	Provider  string
}

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
var idRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-sync" {
		if err := startupMigrate(); err != nil {
			log.Fatalf("startup migrate: %v", err)
		}
		return
	}

	applyEnvOverrides()

	for _, d := range []string{sitesDir, filepath.Dir(legacyAcmeFile), filepath.Dir(logFile), filepath.Dir(layer4ConfPath), filepath.Dir(dynamicDNSConfPath)} {
		if err := os.MkdirAll(d, 0755); err != nil {
			log.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := startupMigrate(); err != nil {
		log.Printf("warn: startup migrate failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sites", listSites)
	mux.HandleFunc("/sites/", siteHandler)
	mux.HandleFunc("/credentials", credentialsHandler)
	mux.HandleFunc("/credentials/", credentialItemHandler)
	mux.HandleFunc("/notifications/channels", notificationChannelsHandler)
	mux.HandleFunc("/notifications/channels/", notificationChannelItemHandler)
	mux.HandleFunc("/certs", certsHandler)
	mux.HandleFunc("/certs/reissue", certReissueHandler)
	mux.HandleFunc("/certs/delete", certDeleteHandler)
	mux.HandleFunc("/probe", probeHandler)
	mux.HandleFunc("/reload", reloadHandler)
	mux.HandleFunc("/logs", logsHandler)
	mux.HandleFunc("/", npmHealthHandler)
	mux.HandleFunc("/reports/hosts", npmHostReportHandler)
	mux.HandleFunc("/caddy/proxy-hosts", npmProxyHostsHandler)
	mux.HandleFunc("/caddy/proxy-hosts/", npmProxyHostItemHandler)
	mux.HandleFunc("/caddy/settings/forward-auth/", npmForwardAuthSettingsHandler)
	mux.HandleFunc("/caddy/settings/authelia", npmAutheliaSettingsHandler)
	mux.HandleFunc("/caddy/settings/authentik", npmAuthentikSettingsHandler)
	mux.HandleFunc("/caddy/settings", settingsHandler)
	mux.HandleFunc("/caddy/certificates", npmCertificatesHandler)
	mux.HandleFunc("/caddy/certificates/authorities", npmCertificateAuthoritiesHandler)
	mux.HandleFunc("/caddy/certificates/dns-providers", npmDNSProvidersHandler)
	mux.HandleFunc("/caddy/certificates/test-http", npmTestHTTPCertificateHandler)
	mux.HandleFunc("/caddy/certificates/validate", npmValidateCertificateHandler)
	mux.HandleFunc("/caddy/certificates/", npmCertificateItemHandler)
	mux.HandleFunc("/caddy/access-lists", npmAccessListsHandler)
	mux.HandleFunc("/caddy/access-lists/", npmAccessListItemHandler)
	mux.HandleFunc("/caddy/wake-devices", npmWakeDevicesHandler)
	mux.HandleFunc("/caddy/wake-devices/", npmWakeDeviceItemHandler)
	mux.HandleFunc("/caddy/redirection-hosts", npmRedirectionHostsHandler)
	mux.HandleFunc("/caddy/redirection-hosts/", npmRedirectionHostItemHandler)
	mux.HandleFunc("/caddy/dead-hosts", npmDeadHostsHandler)
	mux.HandleFunc("/caddy/dead-hosts/", npmDeadHostItemHandler)
	mux.HandleFunc("/caddy/streams", npmStreamsHandler)
	mux.HandleFunc("/caddy/streams/", npmStreamItemHandler)
	mux.HandleFunc("/caddy/dynamic-dns", npmDynamicDNSHandler)
	mux.HandleFunc("/caddy/dynamic-dns/", npmDynamicDNSItemHandler)
	mux.HandleFunc("/caddy/domain-monitor", npmDomainMonitorsHandler)
	mux.HandleFunc("/caddy/domain-monitor/", npmDomainMonitorItemHandler)
	mux.HandleFunc("/caddy/notifications/channels", notificationChannelsHandler)
	mux.HandleFunc("/caddy/notifications/channels/", notificationChannelItemHandler)
	mux.HandleFunc("/nginx/proxy-hosts", npmProxyHostsHandler)
	mux.HandleFunc("/nginx/proxy-hosts/", npmProxyHostItemHandler)
	mux.HandleFunc("/nginx/settings/forward-auth/", npmForwardAuthSettingsHandler)
	mux.HandleFunc("/nginx/settings/authelia", npmAutheliaSettingsHandler)
	mux.HandleFunc("/nginx/settings/authentik", npmAuthentikSettingsHandler)
	mux.HandleFunc("/nginx/certificates", npmCertificatesHandler)
	mux.HandleFunc("/nginx/certificates/authorities", npmCertificateAuthoritiesHandler)
	mux.HandleFunc("/nginx/certificates/dns-providers", npmDNSProvidersHandler)
	mux.HandleFunc("/nginx/certificates/test-http", npmTestHTTPCertificateHandler)
	mux.HandleFunc("/nginx/certificates/validate", npmValidateCertificateHandler)
	mux.HandleFunc("/nginx/certificates/", npmCertificateItemHandler)
	mux.HandleFunc("/audit-log/", npmAuditLogItemHandler)
	mux.HandleFunc("/nginx/access-lists", npmAccessListsHandler)
	mux.HandleFunc("/nginx/access-lists/", npmAccessListItemHandler)
	mux.HandleFunc("/nginx/wake-devices", npmWakeDevicesHandler)
	mux.HandleFunc("/nginx/wake-devices/", npmWakeDeviceItemHandler)
	mux.HandleFunc("/nginx/redirection-hosts", npmRedirectionHostsHandler)
	mux.HandleFunc("/nginx/redirection-hosts/", npmRedirectionHostItemHandler)
	mux.HandleFunc("/nginx/dead-hosts", npmDeadHostsHandler)
	mux.HandleFunc("/nginx/dead-hosts/", npmDeadHostItemHandler)
	mux.HandleFunc("/nginx/streams", npmStreamsHandler)
	mux.HandleFunc("/nginx/streams/", npmStreamItemHandler)
	mux.HandleFunc("/nginx/dynamic-dns", npmDynamicDNSHandler)
	mux.HandleFunc("/nginx/dynamic-dns/", npmDynamicDNSItemHandler)
	mux.HandleFunc("/nginx/domain-monitor", npmDomainMonitorsHandler)
	mux.HandleFunc("/nginx/domain-monitor/", npmDomainMonitorItemHandler)
	mux.HandleFunc("/nginx/notifications/channels", notificationChannelsHandler)
	mux.HandleFunc("/nginx/notifications/channels/", notificationChannelItemHandler)
	mux.HandleFunc("/audit-log", npmEmptyListHandler)
	mux.HandleFunc("/version/check", npmVersionCheckHandler)

	log.Printf("caddy-api listening on %s", listen)
	go listenCaddyLogs()
	go listenDDNSLogs()
	go runDynamicDNSStatusLoop()
	go runDomainMonitorLoop()
	log.Fatal(http.ListenAndServe(listen, mux))
}

func applyEnvOverrides() {
	if value := strings.TrimSpace(os.Getenv("CADDY_UI_API_LISTEN")); value != "" {
		listen = value
	}
}

// ============================================================
// 启动期迁移：legacy credentials.json + 老 acme.conf + 重新渲染所有 site .conf
// ============================================================

func startupMigrate() error {
	// 删除遗留的 global.d/acme.conf
	os.Remove(legacyAcmeFile)

	creds, err := loadCredentials()
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}

	// 给老站点回填 credential_id（仅 wildcard 且只有 1 条凭据时）
	if len(creds) >= 1 {
		_ = backfillSites(creds)
	}

	if err := syncStreamsConfig(); err != nil {
		return fmt.Errorf("sync streams: %w", err)
	}
	if err := syncDynamicDNSConfig(); err != nil {
		return fmt.Errorf("sync dynamic dns: %w", err)
	}
	if err := cleanupManagedCertConflicts(); err != nil {
		return fmt.Errorf("cleanup managed cert conflicts: %w", err)
	}
	if err := cleanupLegacyExactManagedCertPlaceholders(); err != nil {
		return fmt.Errorf("cleanup legacy exact managed cert placeholders: %w", err)
	}
	if err := recoverOrphanCertificateRequests(creds); err != nil {
		return fmt.Errorf("recover orphan certificate requests: %w", err)
	}
	if backups, err := syncWildcardPlaceholders(creds); err != nil {
		restoreBackups(backups)
		return fmt.Errorf("sync wildcard placeholders: %w", err)
	}

	// 用最新凭据全量重新渲染 site .conf
	return renderAllSiteConfs(creds)
}

func backfillSites(creds []Credential) error {
	dnsCreds := dnsCredentials(creds)
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		metaPath := filepath.Join(sitesDir, e.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var s Site
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		changed := false
		if s.Wildcard && s.CredentialID == "" && len(dnsCreds) == 1 {
			s.CredentialID = dnsCreds[0].ID
			s.ChallengePref = "dns"
			changed = true
		}
		if changed {
			out, _ := json.MarshalIndent(s, "", "  ")
			os.WriteFile(metaPath, out, 0644)
		}
	}
	// 同样给占位文件回填 credential_id
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if !strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		metaPath := filepath.Join(sitesDir, e.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var p placeholderMeta
		if json.Unmarshal(data, &p) != nil {
			continue
		}
		if p.CredentialID == "" && len(dnsCreds) == 1 {
			p.CredentialID = dnsCreds[0].ID
			out, _ := json.MarshalIndent(p, "", "  ")
			os.WriteFile(metaPath, out, 0644)
		}
	}
	return nil
}

// ============================================================
// 凭据
// ============================================================

func loadCredentials() ([]Credential, error) {
	data, err := os.ReadFile(credentialPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var creds []Credential
		if err := json.Unmarshal(data, &creds); err != nil {
			return nil, err
		}
		return creds, nil
	}
	// 旧格式（单对象），迁移
	var legacy Credential
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}
	if legacy.Provider == "" {
		return nil, nil
	}
	legacy.ID = newCredID()
	legacy.Name = "默认凭据"
	creds := []Credential{legacy}
	if err := saveCredentials(creds); err != nil {
		log.Printf("warn: migrate credentials.json: %v", err)
	}
	return creds, nil
}

func saveCredentials(creds []Credential) error {
	if creds == nil {
		creds = []Credential{}
	}
	data, _ := json.MarshalIndent(creds, "", "  ")
	return os.WriteFile(credentialPath, data, 0600)
}

func findCredential(id string, creds []Credential) (Credential, bool) {
	for _, c := range creds {
		if c.ID == id {
			return c, true
		}
	}
	return Credential{}, false
}

func credContentEqual(a, b Credential) bool {
	if a.Provider != b.Provider {
		return false
	}
	switch a.Provider {
	case "alidns":
		return a.AliyunKey == b.AliyunKey && a.AliyunSecret == b.AliyunSecret
	case "cloudflare":
		return a.CFToken == b.CFToken
	case "dnspod":
		return a.DNSPodToken == b.DNSPodToken
	case "he":
		return a.HEAPIKey == b.HEAPIKey
	case "digitalplat":
		return a.DigitalPlatAPIKey == b.DigitalPlatAPIKey
	case "dnshe":
		return a.DNSHEAPIKey == b.DNSHEAPIKey && a.DNSHEAPISecret == b.DNSHEAPISecret
	}
	return false
}

func isDNSCredentialProvider(provider string) bool {
	return provider == "alidns" || provider == "cloudflare" || provider == "dnspod" || provider == "he"
}

func dnsProviderDisplayName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "alidns":
		return "阿里云 DNS"
	case "cloudflare":
		return "Cloudflare"
	case "dnspod":
		return "DNSPod.cn"
	case "he":
		return "Hurricane Electric"
	case "digitalplat":
		return "DigitalPlat"
	case "dnshe":
		return "DNSHE"
	default:
		return strings.TrimSpace(provider)
	}
}

func dnsProviderForCredentialID(credentialID string, creds []Credential) string {
	if cred, ok := findCredential(strings.TrimSpace(credentialID), creds); ok {
		return dnsProviderDisplayName(cred.Provider)
	}
	return ""
}

func dnsCredentials(creds []Credential) []Credential {
	out := []Credential{}
	for _, cred := range creds {
		if isDNSCredentialProvider(cred.Provider) {
			out = append(out, cred)
		}
	}
	return out
}

func credentialHasSecret(c Credential) bool {
	return c.AliyunKey != "" || c.AliyunSecret != "" || c.CFToken != "" || c.DNSPodToken != "" || c.HEAPIKey != "" || c.DigitalPlatAPIKey != "" || c.DNSHEAPIKey != "" || c.DNSHEAPISecret != ""
}

func newCredID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func validateCredential(c Credential) error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("凭据名称不能为空")
	}
	switch c.Provider {
	case "alidns":
		if c.AliyunKey == "" || c.AliyunSecret == "" {
			return errors.New("阿里云 DNS 需要 AccessKey ID 与 Secret")
		}
	case "cloudflare":
		if c.CFToken == "" {
			return errors.New("Cloudflare 需要 API Token")
		}
	case "dnspod":
		if c.DNSPodToken == "" {
			return errors.New("DNSPod 需要 API Token，格式为 APP_ID,APP_TOKEN")
		}
	case "he":
		if strings.TrimSpace(c.HEAPIKey) == "" {
			return errors.New("Hurricane Electric 需要 API Key")
		}
	case "digitalplat":
		if strings.TrimSpace(c.DigitalPlatAPIKey) == "" {
			return errors.New("DigitalPlat 需要 API Key")
		}
	case "dnshe":
		if strings.TrimSpace(c.DNSHEAPIKey) == "" || strings.TrimSpace(c.DNSHEAPISecret) == "" {
			return errors.New("DNSHE 需要 API Key 和 API Secret")
		}
	default:
		return fmt.Errorf("未知提供商：%s", c.Provider)
	}
	return nil
}

type certificateAuthority struct {
	ID          string
	Name        string
	CADirectory string
	NeedsURL    bool
	SupportsEAB bool
	Internal    bool
}

var certificateAuthorities = map[string]certificateAuthority{
	"auto": {
		ID:   "auto",
		Name: "Caddy 自动",
	},
	"letsencrypt": {
		ID:          "letsencrypt",
		Name:        "Let's Encrypt",
		CADirectory: "https://acme-v02.api.letsencrypt.org/directory",
	},
	"letsencrypt-staging": {
		ID:          "letsencrypt-staging",
		Name:        "Let's Encrypt Staging",
		CADirectory: "https://acme-staging-v02.api.letsencrypt.org/directory",
	},
	"zerossl": {
		ID:          "zerossl",
		Name:        "ZeroSSL",
		CADirectory: "https://acme.zerossl.com/v2/DV90",
		SupportsEAB: true,
	},
	"google": {
		ID:          "google",
		Name:        "Google Trust Services",
		CADirectory: "https://dv.acme-v02.api.pki.goog/directory",
		SupportsEAB: true,
	},
	"custom": {
		ID:          "custom",
		Name:        "自定义 ACME",
		NeedsURL:    true,
		SupportsEAB: true,
	},
	"internal": {
		ID:       "internal",
		Name:     "Caddy Internal",
		Internal: true,
	},
}

func normalizeIssuerConfig(in IssuerConfig) (IssuerConfig, error) {
	provider := strings.TrimSpace(in.Provider)
	if provider == "" {
		provider = "auto"
	}
	spec, ok := certificateAuthorities[provider]
	if !ok {
		return IssuerConfig{}, fmt.Errorf("不支持的签发机构：%s", provider)
	}
	out := IssuerConfig{
		Provider:      provider,
		CADirectory:   strings.TrimSpace(in.CADirectory),
		EABKeyID:      strings.TrimSpace(in.EABKeyID),
		EABMACKey:     strings.TrimSpace(in.EABMACKey),
		ZeroSSLAPIKey: strings.TrimSpace(in.ZeroSSLAPIKey),
	}
	if spec.Internal {
		return out, nil
	}
	if provider == "auto" {
		out.CADirectory = ""
		out.EABKeyID = ""
		out.EABMACKey = ""
		out.ZeroSSLAPIKey = ""
		return out, nil
	}
	if !spec.NeedsURL {
		out.CADirectory = spec.CADirectory
	}
	if spec.NeedsURL && out.CADirectory == "" {
		return IssuerConfig{}, errors.New("自定义 ACME 需要填写 Directory URL")
	}
	if out.EABKeyID != "" && out.EABMACKey == "" || out.EABMACKey != "" && out.EABKeyID == "" {
		return IssuerConfig{}, errors.New("EAB Key ID 和 EAB MAC Key 必须同时填写")
	}
	if provider == "google" && (out.EABKeyID == "" || out.EABMACKey == "") {
		return IssuerConfig{}, errors.New("Google Trust Services 需要 Google Cloud 项目生成的 EAB，无法无凭据静默申请")
	}
	return out, nil
}

func issuerFromNPM(provider string, meta map[string]any) (IssuerConfig, error) {
	if provider == "" {
		provider = "auto"
	}
	cfg := IssuerConfig{Provider: provider}
	if meta != nil {
		if v := metaString(meta, "issuer_provider", "issuerProvider"); v != "" {
			cfg.Provider = v
		}
		if v := metaString(meta, "ca_directory", "caDirectory"); v != "" {
			cfg.CADirectory = v
		}
		if v := metaString(meta, "acme_ca", "acmeCa"); v != "" && cfg.CADirectory == "" {
			cfg.CADirectory = v
		}
		if v := metaString(meta, "eab_key_id", "eabKeyId"); v != "" {
			cfg.EABKeyID = v
		}
		if v := metaString(meta, "eab_mac_key", "eabMacKey"); v != "" {
			cfg.EABMACKey = v
		}
		if v := metaString(meta, "zerossl_api_key", "zerosslApiKey"); v != "" {
			cfg.ZeroSSLAPIKey = v
		}
	}
	if saved, ok := loadSavedIssuerConfig(cfg.Provider); ok {
		if cfg.CADirectory == "" {
			cfg.CADirectory = saved.CADirectory
		}
		if cfg.EABKeyID == "" {
			cfg.EABKeyID = saved.EABKeyID
		}
		if cfg.EABMACKey == "" {
			cfg.EABMACKey = saved.EABMACKey
		}
		if cfg.ZeroSSLAPIKey == "" {
			cfg.ZeroSSLAPIKey = saved.ZeroSSLAPIKey
		}
	}
	return normalizeIssuerConfig(cfg)
}

func metaString(meta map[string]any, keys ...string) string {
	if meta == nil {
		return ""
	}
	for _, key := range keys {
		if v, ok := meta[key].(string); ok {
			v = strings.TrimSpace(v)
			if v != "" {
				return v
			}
		}
	}
	return ""
}

func metaBool(meta map[string]any, keys ...string) bool {
	if meta == nil {
		return false
	}
	for _, key := range keys {
		switch v := meta[key].(type) {
		case bool:
			return v
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(v))
			return err == nil && parsed
		}
	}
	return false
}

func loadIssuerCredentials() (map[string]IssuerConfig, error) {
	data, err := os.ReadFile(issuerCredentialPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]IssuerConfig{}, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]IssuerConfig{}, nil
	}
	out := map[string]IssuerConfig{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func loadSavedIssuerConfig(provider string) (IssuerConfig, bool) {
	all, err := loadIssuerCredentials()
	if err != nil {
		log.Printf("warn: load issuer credentials: %v", err)
		return IssuerConfig{}, false
	}
	cfg, ok := all[provider]
	return cfg, ok
}

func rememberIssuerConfig(cfg IssuerConfig) {
	cfg, err := normalizeIssuerConfig(cfg)
	if err != nil {
		return
	}
	if cfg.Provider == "" || cfg.Provider == "auto" || cfg.Provider == "letsencrypt" || cfg.Provider == "letsencrypt-staging" || cfg.Provider == "internal" {
		return
	}
	if cfg.EABKeyID == "" && cfg.EABMACKey == "" && cfg.ZeroSSLAPIKey == "" && cfg.Provider != "custom" {
		return
	}
	all, err := loadIssuerCredentials()
	if err != nil {
		log.Printf("warn: load issuer credentials: %v", err)
		return
	}
	all[cfg.Provider] = cfg
	data, _ := json.MarshalIndent(all, "", "  ")
	if err := os.WriteFile(issuerCredentialPath, data, 0600); err != nil {
		log.Printf("warn: save issuer credentials: %v", err)
	}
}

func issuerMeta(cfg IssuerConfig) map[string]any {
	cfg, _ = normalizeIssuerConfig(cfg)
	out := map[string]any{"issuer_provider": cfg.Provider}
	if cfg.CADirectory != "" {
		out["ca_directory"] = cfg.CADirectory
	}
	if cfg.EABKeyID != "" {
		out["eab_key_id"] = cfg.EABKeyID
	}
	if cfg.EABMACKey != "" {
		out["eab_mac_key"] = cfg.EABMACKey
	}
	if cfg.ZeroSSLAPIKey != "" {
		out["zerossl_api_key"] = cfg.ZeroSSLAPIKey
	}
	return out
}

func publicIssuerMeta(cfg IssuerConfig) map[string]any {
	meta := issuerMeta(cfg)
	cfg, _ = normalizeIssuerConfig(cfg)
	meta["name"] = certificateAuthorities[cfg.Provider].Name
	meta["supports_eab"] = certificateAuthorities[cfg.Provider].SupportsEAB
	return meta
}

func mergeMeta(base map[string]any, extra map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// usageOfCredential 返回所有引用此凭据的反代名（用于删除阻止）
func usageOfCredential(id string) ([]string, error) {
	var out []string
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sitesDir, e.Name()))
		if err != nil {
			continue
		}
		var s Site
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		if s.CredentialID == id {
			out = append(out, s.Name)
		}
	}
	if items, err := loadNPMDynamicDNS(); err == nil {
		for _, item := range items {
			if item.CredentialID == id {
				out = append(out, "动态 DNS: "+item.Name)
			}
		}
	}
	if items, err := loadNPMDomainMonitors(); err == nil {
		for _, item := range items {
			if item.CredentialID == id {
				out = append(out, "域名监控: "+item.Name)
			}
		}
	}
	return out, nil
}

// 列表 GET + 新建 POST
func credentialsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		creds, err := loadCredentials()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// 计算每条引用次数
		type credView struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Provider   string `json:"provider"`
			HasSecret  bool   `json:"has_secret"`
			UsageCount int    `json:"usage_count"`
		}
		out := []credView{}
		for _, c := range creds {
			usage, _ := usageOfCredential(c.ID)
			has := credentialHasSecret(c)
			out = append(out, credView{c.ID, c.Name, c.Provider, has, len(usage)})
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var c Credential
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateCredential(c); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		creds, err := loadCredentials()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		c.ID = newCredID()
		creds = append(creds, c)
		if err := saveCredentials(creds); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": c.ID})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// 单条 GET / PUT / DELETE
func credentialItemHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/credentials/")
	if !idRe.MatchString(id) {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	creds, err := loadCredentials()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idx := -1
	for i, c := range creds {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		http.Error(w, "credential not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		c := creds[idx]
		writeJSON(w, http.StatusOK, map[string]any{
			"id":                   c.ID,
			"name":                 c.Name,
			"provider":             c.Provider,
			"has_secret":           credentialHasSecret(c),
			"aliyun_key":           c.AliyunKey,
			"aliyun_secret":        c.AliyunSecret,
			"cf_token":             c.CFToken,
			"dnspod_token":         c.DNSPodToken,
			"he_api_key":           c.HEAPIKey,
			"digital_plat_api_key": c.DigitalPlatAPIKey,
			"dnshe_api_key":        c.DNSHEAPIKey,
			"dnshe_api_secret":     c.DNSHEAPISecret,
		})
	case http.MethodPut:
		var c Credential
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		c.ID = id
		if err := validateCredential(c); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		creds[idx] = c
		// 备份 sites + creds
		backups, err := snapshotAll()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := saveCredentials(creds); err != nil {
			restoreBackups(backups)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := renderAllSiteConfs(creds); err != nil {
			restoreBackups(backups)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := reloadCaddy(); err != nil {
			restoreBackups(backups)
			http.Error(w, "reload failed: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		usage, err := usageOfCredential(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(usage) > 0 {
			http.Error(w, fmt.Sprintf("仍有配置引用此凭据：%s", strings.Join(usage, ", ")), http.StatusBadRequest)
			return
		}
		creds = append(creds[:idx], creds[idx+1:]...)
		backups, _ := snapshotAll()
		if err := saveCredentials(creds); err != nil {
			restoreBackups(backups)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := renderAllSiteConfs(creds); err != nil {
			restoreBackups(backups)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := reloadCaddy(); err != nil {
			restoreBackups(backups)
			http.Error(w, "reload failed: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ============================================================
// Site
// ============================================================

func listSites(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sites := []Site{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		metaPath := filepath.Join(sitesDir, e.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var s Site
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		s.CreatedOn, s.ModifiedOn = normalizePersistentTimestamps(s.CreatedOn, s.ModifiedOn, metaPath)
		if s.ChallengePref == "" {
			s.ChallengePref = "http"
		}
		sites = append(sites, s)
	}
	sort.Slice(sites, func(i, j int) bool { return sites[i].Name < sites[j].Name })
	writeJSON(w, http.StatusOK, sites)
}

func siteHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/sites/")
	if !nameRe.MatchString(name) {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	if strings.HasPrefix(name, managedCertPrefix) {
		http.Error(w, "name 不能以 __cert_ 开头", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		putSite(w, r, name)
	case http.MethodDelete:
		deleteSite(w, name)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func putSite(w http.ResponseWriter, r *http.Request, name string) {
	var s Site
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.Name = name
	if s.ChallengePref == "" {
		s.ChallengePref = "http"
	}
	if s.Kind == "" {
		s.Kind = "proxy"
	}
	if strings.TrimSpace(s.Domain) == "" {
		http.Error(w, "域名不能为空", http.StatusBadRequest)
		return
	}
	if s.Kind == "proxy" && strings.TrimSpace(s.Backend) == "" {
		http.Error(w, "后端地址不能为空", http.StatusBadRequest)
		return
	}
	if s.Kind == "redirection" && strings.TrimSpace(s.RedirectURL) == "" {
		http.Error(w, "重定向目标不能为空", http.StatusBadRequest)
		return
	}
	if s.ChallengePref != "http" && s.ChallengePref != "dns" {
		http.Error(w, "challenge_pref 必须为 http 或 dns", http.StatusBadRequest)
		return
	}
	if s.Wildcard {
		s.ChallengePref = "dns" // 通配符必须 DNS-01
		if s.NoTLS {
			http.Error(w, "仅 HTTP 模式下不能勾选通配符", http.StatusBadRequest)
			return
		}
		for _, d := range splitDomains(s.Domain) {
			if _, ok := wildcardForDomain(d); !ok {
				http.Error(w, fmt.Sprintf("域名 %s 无法计算父域通配符（段数不足或本身已是通配符）", d), http.StatusBadRequest)
				return
			}
		}
	}

	creds, err := loadCredentials()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 通配符场景：自动复用已有父域凭据
	if s.Wildcard {
		resolvedID, err := resolveWildcardCredential(s, creds)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if resolvedID != "" {
			s.CredentialID = resolvedID
		}
	}

	usesCustomCertificate := s.CustomCertFile != "" || s.CustomKeyFile != ""
	// DNS-01 或 wildcard：必须有 credential_id（no_tls / 自定义证书除外）
	if !s.NoTLS && !usesCustomCertificate && (s.ChallengePref == "dns" || s.Wildcard) && s.CredentialID == "" {
		http.Error(w, "DNS-01 / 通配符 需选择 ACME 凭据", http.StatusBadRequest)
		return
	}
	if !s.NoTLS && !usesCustomCertificate && s.CredentialID != "" {
		cred, ok := findCredential(s.CredentialID, creds)
		if !ok {
			http.Error(w, "选择的凭据不存在", http.StatusBadRequest)
			return
		}
		if (s.ChallengePref == "dns" || s.Wildcard) && !isDNSCredentialProvider(cred.Provider) {
			http.Error(w, "DNS-01 / 通配符 需选择 DNS 凭据", http.StatusBadRequest)
			return
		}
	}
	if !s.NoTLS && usesCustomCertificate {
		if s.CustomCertFile == "" || s.CustomKeyFile == "" {
			http.Error(w, "自定义证书文件不完整", http.StatusBadRequest)
			return
		}
		if _, err := os.Stat(s.CustomCertFile); err != nil {
			http.Error(w, "自定义证书文件不存在", http.StatusBadRequest)
			return
		}
		if _, err := os.Stat(s.CustomKeyFile); err != nil {
			http.Error(w, "自定义证书密钥不存在", http.StatusBadRequest)
			return
		}
	}

	if err := checkDomainConflict(splitDomains(s.Domain), name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := renderSite(s, creds); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	confPath := filepath.Join(sitesDir, name+confSuffix)
	metaPath := filepath.Join(sitesDir, name+metaSuffix)
	oldConf, _ := os.ReadFile(confPath)
	oldMeta, _ := os.ReadFile(metaPath)
	rollbackSite := func() {
		rollback(confPath, oldConf)
		rollback(metaPath, oldMeta)
		_ = renderAllSiteConfs(creds)
	}

	stampSiteForSave(&s, metaPath)
	meta, _ := json.MarshalIndent(s, "", "  ")
	if err := os.WriteFile(metaPath, meta, 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := renderAllSiteConfs(creds); err != nil {
		rollbackSite()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	backups, err := syncWildcardPlaceholders(creds)
	if err != nil {
		restoreBackups(backups)
		rollbackSite()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := reloadCaddy(); err != nil {
		restoreBackups(backups)
		rollbackSite()
		http.Error(w, "reload failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func deleteSite(w http.ResponseWriter, name string) {
	confPath := filepath.Join(sitesDir, name+confSuffix)
	metaPath := filepath.Join(sitesDir, name+metaSuffix)
	oldConf, _ := os.ReadFile(confPath)
	oldMeta, _ := os.ReadFile(metaPath)
	os.Remove(confPath)
	os.Remove(metaPath)

	creds, _ := loadCredentials()
	backups, err := syncWildcardPlaceholders(creds)
	if err != nil {
		restoreBackups(backups)
		rollback(confPath, oldConf)
		rollback(metaPath, oldMeta)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := reloadCaddy(); err != nil {
		restoreBackups(backups)
		rollback(confPath, oldConf)
		rollback(metaPath, oldMeta)
		http.Error(w, "reload failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// 通配符自动复用：扫描其他已存在的 wildcard 反代，若它们的父通配符与本次保存相同
// → 拿那条的 credential_id；若用户也提交了 credential_id 且与已有内容不同 → 报错。
// 返回值 resolvedID：若有共享，是要采纳的凭据 ID；否则为空。
func resolveWildcardCredential(s Site, creds []Credential) (string, error) {
	parentWildcards := map[string]bool{}
	for _, d := range splitDomains(s.Domain) {
		if w, ok := wildcardForDomain(d); ok {
			parentWildcards[w] = true
		}
	}
	if len(parentWildcards) == 0 {
		return "", nil
	}

	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return "", err
	}
	// 扫描其他反代条目，看它们的 wildcard 父域是否与本次重叠
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if strings.HasPrefix(baseName, managedCertPrefix) || baseName == s.Name {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sitesDir, e.Name()))
		if err != nil {
			continue
		}
		var other Site
		if json.Unmarshal(data, &other) != nil {
			continue
		}
		if !other.Wildcard {
			continue
		}
		for _, d := range splitDomains(other.Domain) {
			w, ok := wildcardForDomain(d)
			if !ok || !parentWildcards[w] {
				continue
			}
			// 找到一条共享 parent 的现有反代
			otherCred, okC := findCredential(other.CredentialID, creds)
			if !okC {
				continue
			}
			// 如果用户主动选了凭据，比对内容
			if s.CredentialID != "" && s.CredentialID != other.CredentialID {
				newCred, okN := findCredential(s.CredentialID, creds)
				if okN && !credContentEqual(newCred, otherCred) {
					return "", fmt.Errorf("父域 %s 已由反代 %s 使用凭据「%s」申请，新选凭据与其内容不一致，不能共用同一张通配符证书", w, other.Name, otherCred.Name)
				}
			}
			return other.CredentialID, nil
		}
	}
	return "", nil
}

func splitDomains(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func httpSiteAddressList(domains []string) string {
	out := make([]string, 0, len(domains))
	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}
		out = append(out, "http://"+domain)
	}
	return strings.Join(out, ", ")
}

func splitNPMListenDomains(s string, defaultPort int) ([]string, int, []int) {
	entries := splitDomains(s)
	domains := []string{}
	seenDomains := map[string]bool{}
	ports := []int{}
	seenPorts := map[int]bool{}
	hasDefaultAddress := false
	for _, entry := range entries {
		host, port, hasPort := splitHostListenPort(entry)
		if !hasPort {
			host = strings.TrimSpace(entry)
			hasDefaultAddress = true
		}
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		key := strings.ToLower(host)
		if !seenDomains[key] {
			seenDomains[key] = true
			domains = append(domains, host)
		}
		if hasPort && port > 0 && !seenPorts[port] {
			seenPorts[port] = true
			ports = append(ports, port)
		}
	}
	if len(ports) == 0 {
		return domains, 0, nil
	}
	if hasDefaultAddress && defaultPort > 0 {
		if !seenPorts[defaultPort] {
			ports = append([]int{defaultPort}, ports...)
		} else if ports[0] != defaultPort {
			next := []int{defaultPort}
			for _, port := range ports {
				if port != defaultPort {
					next = append(next, port)
				}
			}
			ports = next
		}
		return domains, defaultPort, ports
	}
	if len(ports) > 1 {
		return domains, ports[0], ports
	}
	if hasDefaultAddress {
		return domains, 0, ports
	}
	return domains, ports[0], nil
}

func splitHostListenPort(value string) (string, int, bool) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, ":") {
		port, err := strconv.Atoi(strings.TrimPrefix(value, ":"))
		return "", port, err == nil && port > 0
	}
	host, portText, err := net.SplitHostPort(value)
	if err == nil {
		port, _ := strconv.Atoi(portText)
		return strings.Trim(host, "[]"), port, port > 0
	}
	if idx := strings.LastIndex(value, ":"); idx > 0 && !strings.Contains(value[:idx], ":") {
		port, err := strconv.Atoi(value[idx+1:])
		if err == nil && port > 0 {
			return value[:idx], port, true
		}
	}
	return value, 0, false
}

func normalizeListenPorts(ports []int) []int {
	out := []int{}
	seen := map[int]bool{}
	for _, port := range ports {
		if port <= 0 || port > 65535 || seen[port] {
			continue
		}
		seen[port] = true
		out = append(out, port)
	}
	return out
}

func joinListenDomains(domains []string, listenPort int) string {
	clean := make([]string, 0, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if listenPort > 0 && !strings.Contains(d, ":") {
			d = fmt.Sprintf("%s:%d", d, listenPort)
		}
		clean = append(clean, d)
	}
	return strings.Join(clean, ", ")
}

func joinListenDomainsWithPorts(domains []string, listenPort int, listenPorts []int) string {
	out := []string{}
	seen := map[string]bool{}
	add := func(domain string, port int) {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			return
		}
		value := domain
		if port > 0 {
			value = fmt.Sprintf("%s:%d", domain, port)
		}
		key := strings.ToLower(value)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, value)
	}
	ports := normalizeListenPorts(listenPorts)
	if len(ports) > 0 {
		if listenPort == 0 {
			for _, domain := range domains {
				host, _, ok := splitHostListenPort(domain)
				if ok {
					domain = host
				}
				add(domain, 0)
			}
		}
		for _, port := range ports {
			for _, domain := range domains {
				host, _, ok := splitHostListenPort(domain)
				if ok {
					domain = host
				}
				add(domain, port)
			}
		}
		return strings.Join(out, ", ")
	}
	if listenPort > 0 {
		for _, domain := range domains {
			host, _, ok := splitHostListenPort(domain)
			if ok {
				domain = host
			}
			add(domain, listenPort)
		}
	} else {
		for _, domain := range domains {
			add(domain, 0)
		}
	}
	return strings.Join(out, ", ")
}

func certDomainName(value string) string {
	host, _, ok := splitHostListenPort(value)
	if ok {
		return strings.TrimRight(strings.TrimSpace(host), ".")
	}
	return strings.TrimRight(strings.TrimSpace(value), ".")
}

func certDomainKey(value string) string {
	return strings.ToLower(certDomainName(value))
}

func certDomainMatchesSite(domain string, s Site) bool {
	key := certDomainKey(domain)
	for _, d := range splitDomains(s.Domain) {
		if certDomainKey(d) == key {
			return true
		}
	}
	return false
}

func listenAddressKey(value string) string {
	host, port, ok := splitHostListenPort(value)
	if ok {
		return strings.ToLower(fmt.Sprintf("%s:%d", host, port))
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func listenAddressKeys(domains []string, listenPort int) []string {
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if _, _, ok := splitHostListenPort(d); ok || listenPort <= 0 {
			out = append(out, listenAddressKey(d))
			continue
		}
		out = append(out, listenAddressKey(fmt.Sprintf("%s:%d", d, listenPort)))
	}
	return out
}

func certDomainNames(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		d := certDomainName(v)
		if d == "" {
			continue
		}
		key := strings.ToLower(d)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, d)
	}
	return out
}

func defaultAutheliaConfig() AutheliaConfig {
	spec := forwardAuthProviderSpecs["authelia"]
	return AutheliaConfig{
		Enabled:     false,
		Upstream:    spec.DefaultUpstream,
		URI:         spec.DefaultURI,
		CopyHeaders: append([]string{}, spec.DefaultCopyHeader...),
		FailOpen:    true,
	}
}

func defaultAuthentikConfig() AuthentikConfig {
	spec := forwardAuthProviderSpecs["authentik"]
	return AuthentikConfig{
		Enabled:     false,
		Upstream:    spec.DefaultUpstream,
		URI:         spec.DefaultURI,
		CopyHeaders: append([]string{}, spec.DefaultCopyHeader...),
	}
}

func forwardAuthProviderSpecByID(provider string) (forwardAuthProviderSpec, error) {
	provider = strings.ToLower(strings.TrimSpace(defaultString(provider, "authelia")))
	spec, ok := forwardAuthProviderSpecs[provider]
	if !ok {
		return forwardAuthProviderSpec{}, fmt.Errorf("unsupported forward_auth provider: %s", provider)
	}
	return spec, nil
}

func normalizeAutheliaConfig(cfg AutheliaConfig) (AutheliaConfig, error) {
	defaults := defaultAutheliaConfig()
	if cfg.Upstream == "" {
		cfg.Upstream = defaults.Upstream
	}
	if cfg.URI == "" {
		cfg.URI = defaults.URI
	}
	if len(cfg.CopyHeaders) == 0 {
		cfg.CopyHeaders = defaults.CopyHeaders
	}
	forwardAuth, err := normalizeForwardAuthConfig(ForwardAuthConfig{
		Enabled:     true,
		Provider:    "authelia",
		Upstream:    cfg.Upstream,
		URI:         cfg.URI,
		CopyHeaders: cfg.CopyHeaders,
	})
	if err != nil {
		return AutheliaConfig{}, err
	}
	return AutheliaConfig{
		Enabled:     cfg.Enabled,
		Upstream:    forwardAuth.Upstream,
		URI:         forwardAuth.URI,
		CopyHeaders: forwardAuth.CopyHeaders,
		FailOpen:    cfg.FailOpen,
		LoginURL:    cfg.LoginURL,
	}, nil
}

func normalizeAuthentikConfig(cfg AuthentikConfig) (AuthentikConfig, error) {
	defaults := defaultAuthentikConfig()
	if cfg.Upstream == "" {
		cfg.Upstream = defaults.Upstream
	}
	if cfg.URI == "" {
		cfg.URI = defaults.URI
	}
	if len(cfg.CopyHeaders) == 0 {
		cfg.CopyHeaders = defaults.CopyHeaders
	}
	forwardAuth, err := normalizeForwardAuthConfig(ForwardAuthConfig{
		Enabled:     true,
		Provider:    "authentik",
		Upstream:    cfg.Upstream,
		URI:         cfg.URI,
		CopyHeaders: cfg.CopyHeaders,
	})
	if err != nil {
		return AuthentikConfig{}, err
	}
	return AuthentikConfig{
		Enabled:     cfg.Enabled,
		Upstream:    forwardAuth.Upstream,
		URI:         forwardAuth.URI,
		CopyHeaders: forwardAuth.CopyHeaders,
	}, nil
}

func loadAutheliaConfig() (AutheliaConfig, error) {
	cfg := defaultAutheliaConfig()
	if err := loadJSONFile(autheliaConfigPath, &cfg); err != nil {
		return AutheliaConfig{}, err
	}
	return normalizeAutheliaConfig(cfg)
}

func saveAutheliaConfig(cfg AutheliaConfig) error {
	cfg, err := normalizeAutheliaConfig(cfg)
	if err != nil {
		return err
	}
	return saveJSONFile(autheliaConfigPath, cfg)
}

func loadAuthentikConfig() (AuthentikConfig, error) {
	cfg := defaultAuthentikConfig()
	if err := loadJSONFile(authentikConfigPath, &cfg); err != nil {
		return AuthentikConfig{}, err
	}
	return normalizeAuthentikConfig(cfg)
}

func saveAuthentikConfig(cfg AuthentikConfig) error {
	cfg, err := normalizeAuthentikConfig(cfg)
	if err != nil {
		return err
	}
	return saveJSONFile(authentikConfigPath, cfg)
}

func loadForwardAuthGlobalConfig(provider string) (ForwardAuthGlobalConfig, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "authelia":
		cfg, err := loadAutheliaConfig()
		if err != nil {
			return ForwardAuthGlobalConfig{}, err
		}
		return ForwardAuthGlobalConfig{
			Provider:    "authelia",
			Enabled:     cfg.Enabled,
			Upstream:    cfg.Upstream,
			URI:         cfg.URI,
			CopyHeaders: cfg.CopyHeaders,
			FailOpen:    cfg.FailOpen,
			LoginURL:    cfg.LoginURL,
		}, nil
	case "authentik":
		cfg, err := loadAuthentikConfig()
		if err != nil {
			return ForwardAuthGlobalConfig{}, err
		}
		return ForwardAuthGlobalConfig{
			Provider:    "authentik",
			Enabled:     cfg.Enabled,
			Upstream:    cfg.Upstream,
			URI:         cfg.URI,
			CopyHeaders: cfg.CopyHeaders,
		}, nil
	default:
		return ForwardAuthGlobalConfig{}, fmt.Errorf("unsupported forward_auth provider: %s", provider)
	}
}

func forwardAuthUsesGlobal(cfg ForwardAuthConfig) bool {
	return cfg.UseGlobal || (cfg.Enabled && strings.TrimSpace(cfg.Upstream) == "" && strings.TrimSpace(cfg.URI) == "" && len(cfg.CopyHeaders) == 0)
}

func normalizeProxyHostForwardAuthConfig(cfg ForwardAuthConfig) (ForwardAuthConfig, error) {
	if !cfg.Enabled {
		return ForwardAuthConfig{}, nil
	}
	provider := strings.ToLower(strings.TrimSpace(defaultString(cfg.Provider, "authelia")))
	spec, err := forwardAuthProviderSpecByID(provider)
	if err != nil {
		return ForwardAuthConfig{}, err
	}
	if forwardAuthUsesGlobal(cfg) {
		global, err := loadForwardAuthGlobalConfig(provider)
		if err != nil {
			return ForwardAuthConfig{}, err
		}
		if !global.Enabled {
			return ForwardAuthConfig{}, fmt.Errorf("global %s is disabled", spec.DisplayName)
		}
		return ForwardAuthConfig{Enabled: true, Provider: provider, UseGlobal: true}, nil
	}
	return normalizeForwardAuthConfig(cfg)
}

func resolveForwardAuthConfig(cfg ForwardAuthConfig) (ForwardAuthConfig, ForwardAuthGlobalConfig, error) {
	if !cfg.Enabled {
		return ForwardAuthConfig{}, ForwardAuthGlobalConfig{}, nil
	}
	provider := strings.ToLower(strings.TrimSpace(defaultString(cfg.Provider, "authelia")))
	if forwardAuthUsesGlobal(cfg) {
		spec, err := forwardAuthProviderSpecByID(provider)
		if err != nil {
			return ForwardAuthConfig{}, ForwardAuthGlobalConfig{}, err
		}
		global, err := loadForwardAuthGlobalConfig(provider)
		if err != nil {
			return ForwardAuthConfig{}, ForwardAuthGlobalConfig{}, err
		}
		if !global.Enabled {
			return ForwardAuthConfig{}, ForwardAuthGlobalConfig{}, fmt.Errorf("global %s is disabled", spec.DisplayName)
		}
		return ForwardAuthConfig{
			Enabled:     true,
			Provider:    global.Provider,
			Upstream:    global.Upstream,
			URI:         global.URI,
			CopyHeaders: global.CopyHeaders,
			UseGlobal:   true,
		}, global, nil
	}
	normalized, err := normalizeForwardAuthConfig(cfg)
	return normalized, ForwardAuthGlobalConfig{}, err
}

func normalizeForwardAuthConfig(cfg ForwardAuthConfig) (ForwardAuthConfig, error) {
	if !cfg.Enabled {
		return ForwardAuthConfig{}, nil
	}
	provider := strings.ToLower(strings.TrimSpace(defaultString(cfg.Provider, "authelia")))
	spec, err := forwardAuthProviderSpecByID(provider)
	if err != nil {
		return ForwardAuthConfig{}, err
	}
	upstream := strings.TrimSpace(defaultString(cfg.Upstream, spec.DefaultUpstream))
	if strings.ContainsAny(upstream, "\r\n\t") {
		return ForwardAuthConfig{}, errors.New("forward_auth upstream contains invalid characters")
	}
	u, err := url.Parse(upstream)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ForwardAuthConfig{}, errors.New("forward_auth upstream must be an http or https URL")
	}
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return ForwardAuthConfig{}, errors.New("forward_auth upstream must not include user info, query, or fragment")
	}
	if u.Path != "" && u.Path != "/" {
		return ForwardAuthConfig{}, errors.New("forward_auth upstream must not include a path")
	}
	u.Path = ""
	uri := strings.TrimSpace(defaultString(cfg.URI, spec.DefaultURI))
	if !strings.HasPrefix(uri, "/") || strings.ContainsAny(uri, "\r\n\t") {
		return ForwardAuthConfig{}, errors.New("forward_auth uri must start with / and not contain control characters")
	}
	headers := cfg.CopyHeaders
	if len(headers) == 0 {
		headers = spec.DefaultCopyHeader
	}
	seen := map[string]bool{}
	copyHeaders := []string{}
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}
		if !headerNameRe.MatchString(header) {
			return ForwardAuthConfig{}, fmt.Errorf("invalid forward_auth copy header: %s", header)
		}
		key := strings.ToLower(header)
		if seen[key] {
			continue
		}
		seen[key] = true
		copyHeaders = append(copyHeaders, header)
	}
	if len(copyHeaders) == 0 {
		copyHeaders = append(copyHeaders, spec.DefaultCopyHeader...)
	}
	return ForwardAuthConfig{Enabled: true, Provider: provider, Upstream: u.String(), URI: uri, CopyHeaders: copyHeaders}, nil
}

func collectClaimedDomains(excludeBaseName string) (map[string]string, error) {
	out := map[string]string{}
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if baseName == excludeBaseName {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sitesDir, e.Name()))
		if err != nil {
			continue
		}
		var meta struct {
			Domain string `json:"domain"`
		}
		if json.Unmarshal(data, &meta) != nil {
			continue
		}
		var display string
		if strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		} else {
			display = "反代 " + baseName
		}
		for _, d := range splitDomains(meta.Domain) {
			out[listenAddressKey(d)] = display
		}
	}
	return out, nil
}

func checkDomainConflict(domains []string, excludeBaseName string) error {
	claimed, err := collectClaimedDomains(excludeBaseName)
	if err != nil {
		return err
	}
	for i, key := range listenAddressKeys(domains, 0) {
		if owner, ok := claimed[key]; ok {
			return fmt.Errorf("域名 %s 已被%s占用", domains[i], owner)
		}
	}
	return nil
}

// ============================================================
// 渲染 Caddyfile
// ============================================================

type renderSiteOptions struct {
	UseWildcardSiteAddress bool
}

type namedSite struct {
	BaseName string
	Site     Site
}

type wildcardRouteGroup struct {
	Address string
	Items   []namedSite
}

func renderSite(s Site, creds []Credential) (string, error) {
	return renderSiteWithOptions(s, creds, renderSiteOptions{UseWildcardSiteAddress: true})
}

func renderSiteWithOptions(s Site, creds []Credential, opts renderSiteOptions) (string, error) {
	if s.Disabled {
		return "", nil
	}
	var b strings.Builder
	domains := splitDomains(s.Domain)
	if len(domains) == 0 {
		return "", errors.New("domain empty")
	}
	if !s.NoTLS && len(s.CertificateBindings) > 0 {
		for _, domain := range domains {
			child := certificateBoundSiteForDomainWithCredentials(s, domain, creds)
			conf, err := renderSiteWithOptions(child, creds, opts)
			if err != nil {
				return "", err
			}
			b.WriteString(conf)
			if !strings.HasSuffix(conf, "\n") {
				b.WriteString("\n")
			}
		}
		return b.String(), nil
	}
	if !s.NoTLS && s.CertificateMode == "auto" && len(domains) > 1 {
		for _, domain := range domains {
			child := autoCertificateSiteForDomain(s, domain)
			conf, err := renderSiteWithOptions(child, creds, opts)
			if err != nil {
				return "", err
			}
			b.WriteString(conf)
			if !strings.HasSuffix(conf, "\n") {
				b.WriteString("\n")
			}
		}
		return b.String(), nil
	}
	if !s.NoTLS && s.CertificateMode == "auto" && len(domains) == 1 {
		s = autoCertificateSiteForDomain(s, domains[0])
	}

	// no_tls = 仅 HTTP，域名加 http:// 前缀，不触发证书申请
	siteAddr := strings.Join(domains, ", ")
	hostScoped := false
	hostMatcher := "@site_host"
	hostDomains := []string{}
	if !s.NoTLS && s.Wildcard && opts.UseWildcardSiteAddress {
		if wildcardAddr, exactHosts, ok := wildcardSiteAddressForDomains(domains); ok {
			siteAddr = wildcardAddr
			hostDomains = exactHosts
			hostScoped = len(hostDomains) > 0
			hostMatcher = siteHostMatcherName(s, hostDomains)
		}
	}
	if s.NoTLS {
		siteAddr = httpSiteAddressList(domains)
	}
	fmt.Fprintf(&b, "%s {\n", siteAddr)
	blockInner := "    "
	if hostScoped {
		fmt.Fprintf(&b, "%s%s host %s\n", blockInner, hostMatcher, strings.Join(hostDomains, " "))
		fmt.Fprintf(&b, "%shandle %s {\n", blockInner, hostMatcher)
		blockInner = "        "
	}
	closeHostScope := func() {
		if hostScoped {
			b.WriteString("    }\n")
		}
	}

	if err := writeSiteRouteContent(&b, s, blockInner); err != nil {
		return "", err
	}
	closeHostScope()

	// DNS-01 或 wildcard 单域名签发，emit tls block
	if err := writeOptionalTLSBlock(&b, s, creds); err != nil {
		return "", err
	}

	b.WriteString("}\n")
	return b.String(), nil
}

func writeSiteRouteContent(b *strings.Builder, s Site, blockInner string) error {
	switch s.Kind {
	case "redirection":
		code := s.RedirectCode
		if code == 0 {
			code = http.StatusMovedPermanently
		}
		target := strings.TrimSpace(s.RedirectURL)
		if s.PreservePath && !strings.Contains(target, "{uri}") {
			target = strings.TrimRight(target, "/") + "{uri}"
		}
		fmt.Fprintf(b, "%sredir %s %d\n", blockInner, target, code)
		return nil
	case "dead":
		fmt.Fprintf(b, "%srespond 404\n", blockInner)
		return nil
	}

	// 多路径模式：优先使用 Locations
	if len(s.Locations) > 0 {
		return writeMultiLocationRouteContent(b, s, blockInner)
	}

	inner := blockInner
	path := strings.TrimSpace(s.Path)
	if path != "" {
		if err := writeForwardAuthPublicRoutes(b, s, blockInner); err != nil {
			return err
		}
	}
	if path != "" {
		fmt.Fprintf(b, "%shandle %s {\n", inner, path)
		inner = "        "
	}

	accessList, hasAccessList := accessListByID(s.AccessListID)
	if hasAccessList {
		if err := writeAccessListBlock(b, accessList, inner); err != nil {
			return err
		}
	}
	if err := writeProtectedReverseProxyBlock(b, s, inner, path != ""); err != nil {
		return err
	}

	if path != "" {
		fmt.Fprintf(b, "%s}\n", blockInner)
	}
	return nil
}

func writeMultiLocationRouteContent(b *strings.Builder, s Site, blockInner string) error {
	var defaultLoc *ProxyLocation
	var pathLocs []ProxyLocation
	for i, loc := range s.Locations {
		if strings.TrimSpace(loc.Path) == "" {
			defaultLoc = &s.Locations[i]
		} else {
			pathLocs = append(pathLocs, loc)
		}
	}

	if err := writeForwardAuthPublicRoutes(b, s, blockInner); err != nil {
		return err
	}

	// 按路径长度降序排列，确保更具体的路径优先匹配（如 /api/v2 排在 /api 前面）
	sort.Slice(pathLocs, func(i, j int) bool {
		return len(pathLocs[i].Path) > len(pathLocs[j].Path)
	})

	for _, loc := range pathLocs {
		locPath := strings.TrimRight(loc.Path, "/")
		if locPath == "" {
			locPath = "/"
		}
		inner := blockInner + "    "
		fp := strings.TrimSpace(loc.ForwardPath)
		if fp != "" {
			fp = strings.TrimRight(fp, "/")
		}
		if locPath != "/" {
			// 精确路径: /hh → 重写到 forwardPath（如 /pricing）
			fmt.Fprintf(b, "%shandle_path %s {\n", blockInner, locPath)
			if fp != "" {
				fmt.Fprintf(b, "%srewrite * %s\n", inner, fp)
			}
			if err := writeLocationReverseProxy(b, s, loc, inner); err != nil {
				return err
			}
			fmt.Fprintf(b, "%s}\n", blockInner)
			// 子路径: /hh/test → 重写到 /pricing/test
			fmt.Fprintf(b, "%shandle_path %s/* {\n", blockInner, locPath)
			if fp != "" {
				fmt.Fprintf(b, "%srewrite * %s{uri}\n", inner, fp)
			}
			if err := writeLocationReverseProxy(b, s, loc, inner); err != nil {
				return err
			}
			fmt.Fprintf(b, "%s}\n", blockInner)
		} else {
			// 根路径 location
			fmt.Fprintf(b, "%shandle_path / {\n", blockInner)
			if fp != "" {
				fmt.Fprintf(b, "%srewrite * %s\n", inner, fp)
			}
			if err := writeLocationReverseProxy(b, s, loc, inner); err != nil {
				return err
			}
			fmt.Fprintf(b, "%s}\n", blockInner)
		}
	}

	if defaultLoc != nil {
		return writeLocationReverseProxy(b, s, *defaultLoc, blockInner)
	}
	accessList, hasAccessList := accessListByID(s.AccessListID)
	if hasAccessList {
		if err := writeAccessListBlock(b, accessList, blockInner); err != nil {
			return err
		}
	}
	return writeProtectedReverseProxyBlock(b, s, blockInner, false)
}

func writeLocationReverseProxy(b *strings.Builder, s Site, loc ProxyLocation, indent string) error {
	backend := loc.Backend
	if backend == "" {
		backend = s.Backend
	}
	cleanBackend, _ := sanitizeBackendURL(backend)

	accessList, hasAccessList := accessListByID(s.AccessListID)
	if hasAccessList {
		if err := writeAccessListBlock(b, accessList, indent); err != nil {
			return err
		}
	}

	needsInsecureSkipVerify := s.UpstreamInsecureSkipVerify && strings.HasPrefix(strings.ToLower(strings.TrimSpace(cleanBackend)), "https://")
	// Extract host:port for Host header (strip scheme)
	upstreamHostPort := cleanBackend
	if idx := strings.Index(upstreamHostPort, "://"); idx != -1 {
		upstreamHostPort = upstreamHostPort[idx+3:]
	}
	if !needsInsecureSkipVerify {
		fmt.Fprintf(b, "%sreverse_proxy %s {\n", indent, cleanBackend)
		fmt.Fprintf(b, "%s    header_up Host %s\n", indent, upstreamHostPort)
		fmt.Fprintf(b, "%s}\n", indent)
	} else {
		fmt.Fprintf(b, "%sreverse_proxy %s {\n", indent, cleanBackend)
		fmt.Fprintf(b, "%s    transport http {\n", indent)
		fmt.Fprintf(b, "%s        tls_insecure_skip_verify\n", indent)
		fmt.Fprintf(b, "%s    }\n", indent)
		fmt.Fprintf(b, "%s    header_up Host %s\n", indent, upstreamHostPort)
		fmt.Fprintf(b, "%s}\n", indent)
	}
	return nil
}

func wildcardSiteAddressForDomains(domains []string) (string, []string, bool) {
	addresses, exactHosts, ok := wildcardSiteAddressesForDomains(domains)
	if !ok {
		return "", nil, false
	}
	return strings.Join(addresses, ", "), exactHosts, true
}

func wildcardSiteAddressesForDomains(domains []string) ([]string, []string, bool) {
	addresses := []string{}
	exactHosts := []string{}
	seenAddresses := map[string]bool{}
	seenHosts := map[string]bool{}
	for _, domain := range domains {
		host := certDomainName(domain)
		port := 0
		if h, p, ok := splitHostListenPort(domain); ok {
			host = certDomainName(h)
			port = p
		}
		wildcardDomain, ok := wildcardForDomain(host)
		if !ok {
			return nil, nil, false
		}
		address := wildcardDomain
		if port > 0 {
			address = fmt.Sprintf("%s:%d", wildcardDomain, port)
		}
		if !seenAddresses[strings.ToLower(address)] {
			seenAddresses[strings.ToLower(address)] = true
			addresses = append(addresses, address)
		}
		if host != "" && !seenHosts[strings.ToLower(host)] {
			seenHosts[strings.ToLower(host)] = true
			exactHosts = append(exactHosts, host)
		}
	}
	if len(addresses) == 0 || len(exactHosts) == 0 {
		return nil, nil, false
	}
	return addresses, exactHosts, true
}

func renderedWildcardSiteAddressKeys(s Site, creds []Credential) []string {
	if s.Disabled || s.NoTLS {
		return nil
	}
	domains := splitDomains(s.Domain)
	if len(domains) == 0 {
		return nil
	}
	if len(s.CertificateBindings) > 0 {
		out := []string{}
		for _, domain := range domains {
			child := certificateBoundSiteForDomainWithCredentials(s, domain, creds)
			out = append(out, renderedWildcardSiteAddressKeys(child, creds)...)
		}
		return uniqueSortedStrings(out)
	}
	if s.CertificateMode == "auto" && len(domains) > 1 {
		out := []string{}
		for _, domain := range domains {
			child := autoCertificateSiteForDomain(s, domain)
			out = append(out, renderedWildcardSiteAddressKeys(child, creds)...)
		}
		return uniqueSortedStrings(out)
	}
	if s.CertificateMode == "auto" && len(domains) == 1 {
		s = autoCertificateSiteForDomain(s, domains[0])
		domains = splitDomains(s.Domain)
	}
	if !s.Wildcard {
		return nil
	}
	addresses, _, ok := wildcardSiteAddressesForDomains(domains)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		out = append(out, strings.ToLower(strings.TrimSpace(address)))
	}
	return uniqueSortedStrings(out)
}

func expandSiteRenderLeaves(s Site, creds []Credential) []Site {
	if s.Disabled {
		return []Site{s}
	}
	domains := splitDomains(s.Domain)
	if len(domains) == 0 {
		return []Site{s}
	}
	if !s.NoTLS && len(s.CertificateBindings) > 0 {
		out := []Site{}
		for _, domain := range domains {
			out = append(out, expandSiteRenderLeaves(certificateBoundSiteForDomainWithCredentials(s, domain, creds), creds)...)
		}
		return out
	}
	if !s.NoTLS && s.CertificateMode == "auto" && len(domains) > 1 {
		out := []Site{}
		for _, domain := range domains {
			out = append(out, expandSiteRenderLeaves(autoCertificateSiteForDomain(s, domain), creds)...)
		}
		return out
	}
	if !s.NoTLS && s.CertificateMode == "auto" && len(domains) == 1 {
		return expandSiteRenderLeaves(autoCertificateSiteForDomain(s, domains[0]), creds)
	}
	return []Site{s}
}

func wildcardRouteGroupAddress(s Site) (string, bool) {
	if s.Disabled || s.NoTLS || !s.Wildcard || s.CustomCertFile != "" || s.CustomKeyFile != "" {
		return "", false
	}
	addresses, _, ok := wildcardSiteAddressesForDomains(splitDomains(s.Domain))
	if !ok || len(addresses) != 1 {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(addresses[0])), true
}

func wildcardTLSCompatibilityKey(s Site) string {
	issuer, _ := normalizeIssuerConfig(s.Issuer)
	parts := []string{
		strings.TrimSpace(s.CredentialID),
		strings.TrimSpace(s.ChallengePref),
		issuer.Provider,
		issuer.CADirectory,
		issuer.EABKeyID,
		issuer.EABMACKey,
		issuer.ZeroSSLAPIKey,
	}
	return strings.Join(parts, "\x00")
}

func renderWildcardRouteGroup(group wildcardRouteGroup, creds []Credential) (string, error) {
	items := append([]namedSite(nil), group.Items...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].BaseName < items[j].BaseName
	})
	var b strings.Builder
	fmt.Fprintf(&b, "%s {\n", group.Address)
	for _, item := range items {
		site := item.Site
		if strings.TrimSpace(site.Name) == "" {
			site.Name = item.BaseName
		}
		_, hosts, ok := wildcardSiteAddressForDomains(splitDomains(site.Domain))
		if !ok || len(hosts) == 0 {
			continue
		}
		matcher := siteHostMatcherName(site, hosts)
		fmt.Fprintf(&b, "    %s host %s\n", matcher, strings.Join(hosts, " "))
		fmt.Fprintf(&b, "    handle %s {\n", matcher)
		if err := writeSiteRouteContent(&b, site, "        "); err != nil {
			return "", err
		}
		b.WriteString("    }\n")
	}
	b.WriteString("    handle {\n")
	b.WriteString("        respond 404\n")
	b.WriteString("    }\n")
	if len(items) > 0 {
		if err := writeOptionalTLSBlock(&b, items[0].Site, creds); err != nil {
			return "", err
		}
	}
	b.WriteString("}\n")
	return b.String(), nil
}

func siteHostMatcherName(s Site, hosts []string) string {
	seed := strings.Join(hosts, ",")
	if s.Name != "" {
		seed = s.Name + ":" + seed
	}
	return fmt.Sprintf("@site_host_%d", stableID(seed))
}

func writeAccessListBlock(b *strings.Builder, access npmAccessList, indent string) error {
	denies := accessClientAddresses(access, "deny")
	if len(denies) > 0 {
		fmt.Fprintf(b, "%s@access_deny remote_ip %s\n", indent, strings.Join(denies, " "))
		fmt.Fprintf(b, "%srespond @access_deny 403\n", indent)
	}
	allows := accessClientAddresses(access, "allow")
	authItems := accessBasicAuthItems(access)
	authBypassAllows := len(allows) > 0 && len(authItems) > 0 && access.SatisfyAny
	if len(allows) > 0 && !authBypassAllows {
		fmt.Fprintf(b, "%s@access_allow {\n", indent)
		fmt.Fprintf(b, "%s    not remote_ip %s\n", indent, strings.Join(allows, " "))
		fmt.Fprintf(b, "%s}\n", indent)
		fmt.Fprintf(b, "%srespond @access_allow 403\n", indent)
	}
	if len(authItems) > 0 {
		authOnly := []string{}
		authSkip := []string{}
		if len(allows) > 0 {
			if access.SatisfyAny {
				authSkip = appendUniqueStrings(authSkip, allows...)
			} else {
				authOnly = appendUniqueStrings(authOnly, allows...)
			}
		}
		authSkip = appendUniqueStrings(authSkip, denies...)
		matcher := ""
		if len(authOnly) > 0 || len(authSkip) > 0 {
			matcher = "@access_auth_required"
			fmt.Fprintf(b, "%s%s {\n", indent, matcher)
			if len(authOnly) > 0 {
				fmt.Fprintf(b, "%s    remote_ip %s\n", indent, strings.Join(authOnly, " "))
			}
			if len(authSkip) > 0 {
				fmt.Fprintf(b, "%s    not remote_ip %s\n", indent, strings.Join(authSkip, " "))
			}
			fmt.Fprintf(b, "%s}\n", indent)
		}
		fmt.Fprintf(b, "%sbasic_auth", indent)
		if matcher != "" {
			fmt.Fprintf(b, " %s", matcher)
		}
		fmt.Fprintf(b, " {\n")
		for _, item := range authItems {
			hash, err := accessBasicAuthPasswordHash(item.Password)
			if err != nil {
				return fmt.Errorf("通信规则 %s 的用户 %s 密码处理失败: %w", access.Name, item.Username, err)
			}
			fmt.Fprintf(b, "%s    %s %s\n", indent, caddyQuote(strings.TrimSpace(item.Username)), caddyQuote(hash))
		}
		fmt.Fprintf(b, "%s}\n", indent)
		if !access.PassAuth {
			fmt.Fprintf(b, "%srequest_header -Authorization\n", indent)
		}
	}
	return nil
}

func appendUniqueStrings(values []string, next ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range next {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	return values
}

func accessBasicAuthItems(access npmAccessList) []npmAccessListItem {
	out := []npmAccessListItem{}
	seen := map[string]bool{}
	for _, item := range access.Items {
		username := strings.TrimSpace(item.Username)
		if username == "" || item.Password == "" || seen[username] {
			continue
		}
		seen[username] = true
		item.Username = username
		out = append(out, item)
	}
	return out
}

func accessBasicAuthPasswordHash(password string) (string, error) {
	if _, err := bcrypt.Cost([]byte(password)); err == nil {
		return password, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func writeReverseProxyBlock(b *strings.Builder, backend string, _ string, upstreamInsecureSkipVerify bool, indent string) {
	cleanBackend, _ := sanitizeBackendURL(backend)
	needsInsecureSkipVerify := upstreamInsecureSkipVerify && strings.HasPrefix(strings.ToLower(strings.TrimSpace(cleanBackend)), "https://")

	if !needsInsecureSkipVerify {
		fmt.Fprintf(b, "%sreverse_proxy %s\n", indent, cleanBackend)
	} else {
		fmt.Fprintf(b, "%sreverse_proxy %s {\n", indent, cleanBackend)
		fmt.Fprintf(b, "%s    transport http {\n", indent)
		fmt.Fprintf(b, "%s        tls_insecure_skip_verify\n", indent)
		fmt.Fprintf(b, "%s    }\n", indent)
		fmt.Fprintf(b, "%s}\n", indent)
	}
}

// sanitizeBackendURL 从可能包含路径的 backend 中提取 scheme://host:port 部分，
// 并返回需要在 reverse_proxy 中附加的 uri rewrite 指令（如果有路径的话）。
func sanitizeBackendURL(raw string) (clean string, upstreamPath string) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		return raw, ""
	}
	schemeEnd := strings.Index(raw, "://")
	scheme := raw[:schemeEnd]
	rest := raw[schemeEnd+3:]

	// 将 rest 拆为 host[:port] + path
	hostPort := rest
	upstreamPath = ""
	if slash := strings.Index(rest, "/"); slash != -1 {
		hostPort = rest[:slash]
		upstreamPath = strings.TrimRight(rest[slash:], "/")
	}

	return scheme + "://" + hostPort, upstreamPath
}

// splitHostAndPath 从可能包含路径的 host 字段中分离出纯 host 和路径部分。
// 例如 "10.0.0.1/environments" → ("10.0.0.1", "/environments")
func splitHostAndPath(raw string) (host string, path string) {
	raw = strings.TrimSpace(raw)
	if slash := strings.Index(raw, "/"); slash != -1 {
		return raw[:slash], strings.TrimRight(raw[slash:], "/")
	}
	return raw, ""
}

func writeProtectedReverseProxyBlock(b *strings.Builder, s Site, indent string, pathScoped bool) error {
	cfg, global, err := resolveForwardAuthConfig(s.ForwardAuth)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		writeReverseProxyBlock(b, s.Backend, s.Headers, s.UpstreamInsecureSkipVerify, indent)
		return nil
	}
	loginURL := global.LoginURL
	if cfg.Provider == "authelia" && loginURL == "" {
		if gc, err := loadAutheliaConfig(); err == nil {
			loginURL = gc.LoginURL
		}
	}
	if cfg.Provider == "authentik" && !pathScoped {
		writeAuthentikProtectedReverseProxyBlock(b, cfg, s.Backend, s.Headers, s.UpstreamInsecureSkipVerify, indent)
		return nil
	}
	if cfg.UseGlobal && global.FailOpen {
		writeForwardAuthFailOpenBlock(b, cfg, s.Backend, s.Headers, s.UpstreamInsecureSkipVerify, loginURL, indent)
		return nil
	}
	writeForwardAuthBlock(b, cfg, s.Backend, loginURL, indent)
	writeReverseProxyBlock(b, s.Backend, s.Headers, s.UpstreamInsecureSkipVerify, indent)
	return nil
}

func writeForwardAuthPublicRoutes(b *strings.Builder, s Site, indent string) error {
	cfg, _, err := resolveForwardAuthConfig(s.ForwardAuth)
	if err != nil {
		return err
	}
	if !cfg.Enabled || cfg.Provider != "authentik" {
		return nil
	}
	writeAuthentikOutpostRoute(b, cfg, indent)
	return nil
}

func writeForwardAuthBlock(b *strings.Builder, cfg ForwardAuthConfig, backend string, loginURL string, indent string) error {
	cfg, err := normalizeForwardAuthConfig(cfg)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}
	if cfg.Provider == "authentik" {
		fmt.Fprintf(b, "%sforward_auth %s {\n", indent, cfg.Upstream)
		fmt.Fprintf(b, "%s    uri %s\n", indent, cfg.URI)
		fmt.Fprintf(b, "%s    copy_headers %s\n", indent, strings.Join(cfg.CopyHeaders, " "))
		fmt.Fprintf(b, "%s}\n", indent)
		return nil
	}

	loginPath := strings.TrimRight(loginURL, "/")
	sameSite := strings.HasPrefix(loginURL, "/")

	if sameSite {
		// Same-site proxy: Authelia UI served under the same domain
		fmt.Fprintf(b, "%sroute {\n", indent)
		inner := indent + "    "

		// Authelia login page
		fmt.Fprintf(b, "%shandle /authelia/* {\n", inner)
		fmt.Fprintf(b, "%suri strip_prefix /authelia\n", inner+"    ")
		fmt.Fprintf(b, "%sreverse_proxy %s\n", inner+"    ", cfg.Upstream)
		fmt.Fprintf(b, "%s}\n", inner)

		// Authelia static assets (404 fallback to backend)
		for _, prefix := range []string{"/static/*", "/locales/*"} {
			tag := strings.Trim(prefix, "/*")
			fmt.Fprintf(b, "%shandle %s {\n", inner, prefix)
			fmt.Fprintf(b, "%sreverse_proxy %s {\n", inner+"    ", cfg.Upstream)
			fmt.Fprintf(b, "%s@%s_404 status 404\n", inner+"        ", tag)
			fmt.Fprintf(b, "%shandle_response @%s_404 {\n", inner+"        ", tag)
			fmt.Fprintf(b, "%sreverse_proxy %s\n", inner+"            ", backend)
			fmt.Fprintf(b, "%s}\n", inner+"        ")
			fmt.Fprintf(b, "%s}\n", inner+"    ")
			fmt.Fprintf(b, "%s}\n", inner)
		}

		// Authelia API endpoints (listed individually to avoid intercepting backend APIs)
		apiPaths := []string{
			"/api/firstfactor",
			"/api/secondfactor*",
			"/api/user/*",
			"/api/state",
			"/api/configuration*",
			"/api/checks/*",
			"/api/logout",
			"/api/reset-password*",
			"/api/oidc/*",
		}
		for _, p := range apiPaths {
			fmt.Fprintf(b, "%shandle %s {\n", inner, p)
			fmt.Fprintf(b, "%sreverse_proxy %s\n", inner+"    ", cfg.Upstream)
			fmt.Fprintf(b, "%s}\n", inner)
		}

		// forward_auth check
		fmt.Fprintf(b, "%sforward_auth %s {\n", inner, cfg.Upstream)
		fmt.Fprintf(b, "%suri %s\n", inner+"    ", cfg.URI)
		fmt.Fprintf(b, "%scopy_headers %s\n", inner+"    ", strings.Join(cfg.CopyHeaders, " "))
		fmt.Fprintf(b, "%sheader_up X-Forwarded-Method {method}\n", inner+"    ")
		fmt.Fprintf(b, "%sheader_up X-Forwarded-Uri {uri}\n", inner+"    ")
		fmt.Fprintf(b, "%sheader_up X-Forwarded-Proto {scheme}\n", inner+"    ")
		fmt.Fprintf(b, "%sheader_up X-Forwarded-Host {host}\n", inner+"    ")
		fmt.Fprintf(b, "%sheader_up X-Real-IP {remote_host}\n", inner+"    ")
		fmt.Fprintf(b, "%s@auth_error status 401 403\n", inner+"    ")
		fmt.Fprintf(b, "%shandle_response @auth_error {\n", inner+"    ")
		fmt.Fprintf(b, "%sredir * %s/?rd={scheme}://{host}{uri} 302\n", inner+"        ", loginPath)
		fmt.Fprintf(b, "%s}\n", inner+"    ")
		fmt.Fprintf(b, "%s}\n", inner)

		fmt.Fprintf(b, "%s}\n", indent)
	} else {
		// Cross-domain redirect: simple forward_auth block
		fmt.Fprintf(b, "%sforward_auth %s {\n", indent, cfg.Upstream)
		fmt.Fprintf(b, "%s    uri %s\n", indent, cfg.URI)
		fmt.Fprintf(b, "%s    copy_headers %s\n", indent, strings.Join(cfg.CopyHeaders, " "))
		fmt.Fprintf(b, "%s    header_up X-Forwarded-Method {method}\n", indent)
		fmt.Fprintf(b, "%s    header_up X-Forwarded-Uri {uri}\n", indent)
		fmt.Fprintf(b, "%s    header_up X-Forwarded-Proto {scheme}\n", indent)
		fmt.Fprintf(b, "%s    header_up X-Forwarded-Host {host}\n", indent)
		fmt.Fprintf(b, "%s    header_up X-Real-IP {remote_host}\n", indent)
		if loginPath != "" {
			fmt.Fprintf(b, "%s    @auth_error status 401 403\n", indent)
			fmt.Fprintf(b, "%s    handle_response @auth_error {\n", indent)
			fmt.Fprintf(b, "%s        redir * %s?rd={scheme}://{host}{uri} 302\n", indent, loginPath)
			fmt.Fprintf(b, "%s    }\n", indent)
		}
		fmt.Fprintf(b, "%s}\n", indent)
	}
	return nil
}

func writeAuthentikProtectedReverseProxyBlock(b *strings.Builder, cfg ForwardAuthConfig, backend string, headers string, upstreamInsecureSkipVerify bool, indent string) {
	fmt.Fprintf(b, "%sroute {\n", indent)
	writeAuthentikOutpostRoute(b, cfg, indent+"    ")
	fmt.Fprintf(b, "%s    forward_auth %s {\n", indent, cfg.Upstream)
	fmt.Fprintf(b, "%s        uri %s\n", indent, cfg.URI)
	fmt.Fprintf(b, "%s        copy_headers %s\n", indent, strings.Join(cfg.CopyHeaders, " "))
	fmt.Fprintf(b, "%s    }\n", indent)
	writeReverseProxyBlock(b, backend, headers, upstreamInsecureSkipVerify, indent+"    ")
	fmt.Fprintf(b, "%s}\n", indent)
}

func writeAuthentikOutpostRoute(b *strings.Builder, cfg ForwardAuthConfig, indent string) {
	fmt.Fprintf(b, "%shandle /outpost.goauthentik.io/* {\n", indent)
	if strings.HasPrefix(cfg.Upstream, "https://") {
		fmt.Fprintf(b, "%s    reverse_proxy %s {\n", indent, cfg.Upstream)
		fmt.Fprintf(b, "%s        header_up Host {upstream_hostport}\n", indent)
		fmt.Fprintf(b, "%s    }\n", indent)
	} else {
		fmt.Fprintf(b, "%s    reverse_proxy %s\n", indent, cfg.Upstream)
	}
	fmt.Fprintf(b, "%s}\n", indent)
}

func writeForwardAuthFailOpenBlock(b *strings.Builder, cfg ForwardAuthConfig, backend string, headers string, upstreamInsecureSkipVerify bool, loginURL string, indent string) {
	fmt.Fprintf(b, "%sroute {\n", indent)
	fmt.Fprintf(b, "%s    vars authelia_phase true\n", indent)
	fmt.Fprintf(b, "%s    reverse_proxy %s {\n", indent, cfg.Upstream)
	fmt.Fprintf(b, "%s        method GET\n", indent)
	fmt.Fprintf(b, "%s        rewrite %s\n", indent, cfg.URI)
	fmt.Fprintf(b, "%s        header_up X-Forwarded-Method {method}\n", indent)
	fmt.Fprintf(b, "%s        header_up X-Forwarded-Uri {uri}\n", indent)
	fmt.Fprintf(b, "%s        header_up X-Forwarded-Host {host}\n", indent)
	fmt.Fprintf(b, "%s        @auth_ok status 2xx\n", indent)
	fmt.Fprintf(b, "%s        handle_response @auth_ok {\n", indent)
	for _, header := range cfg.CopyHeaders {
		fmt.Fprintf(b, "%s            request_header %s {rp.header.%s}\n", indent, header, header)
	}
	fmt.Fprintf(b, "%s            vars authelia_phase false\n", indent)
	writeReverseProxyBlock(b, backend, headers, upstreamInsecureSkipVerify, indent+"            ")
	fmt.Fprintf(b, "%s        }\n", indent)
	if loginURL != "" {
		fmt.Fprintf(b, "%s        @auth_error status 401 403\n", indent)
		fmt.Fprintf(b, "%s        handle_response @auth_error {\n", indent)
		fmt.Fprintf(b, "%s            redir * %s?rd={scheme}://{host}{uri} 302\n", indent, strings.TrimRight(loginURL, "/"))
		fmt.Fprintf(b, "%s        }\n", indent)
	}
	fmt.Fprintf(b, "%s    }\n", indent)
	fmt.Fprintf(b, "%s    vars authelia_phase false\n", indent)
	fmt.Fprintf(b, "%s}\n", indent)
	fmt.Fprintf(b, "%shandle_errors {\n", indent)
	fmt.Fprintf(b, "%s    @authelia_unavailable expression `{http.vars.authelia_phase} == true && {err.status_code} >= 500`\n", indent)
	fmt.Fprintf(b, "%s    handle @authelia_unavailable {\n", indent)
	writeReverseProxyBlock(b, backend, headers, upstreamInsecureSkipVerify, indent+"        ")
	fmt.Fprintf(b, "%s    }\n", indent)
	fmt.Fprintf(b, "%s}\n", indent)
}

func accessClientAddresses(access npmAccessList, directive string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, client := range access.Clients {
		if !strings.EqualFold(strings.TrimSpace(client.Directive), directive) {
			continue
		}
		addr := strings.TrimSpace(client.Address)
		if addr == "" || seen[addr] {
			continue
		}
		seen[addr] = true
		out = append(out, addr)
	}
	return out
}

func writeOptionalTLSBlock(b *strings.Builder, s Site, creds []Credential) error {
	if s.CustomCertFile != "" || s.CustomKeyFile != "" {
		if s.CustomCertFile == "" || s.CustomKeyFile == "" {
			return errors.New("自定义证书文件不完整")
		}
		fmt.Fprintf(b, "    tls %s %s\n", caddyQuote(s.CustomCertFile), caddyQuote(s.CustomKeyFile))
		return nil
	}
	if s.ChallengePref != "dns" && !s.Wildcard {
		if issuerNeedsTLSBlock(s.Issuer) {
			b.WriteString(renderTLSBlock(Credential{}, s.Issuer, "    "))
		}
		return nil
	}
	credentialID := s.CredentialID
	issuer := s.Issuer
	if strings.TrimSpace(credentialID) == "" && s.Wildcard {
		for _, domain := range splitDomains(s.Domain) {
			wildcardDomain, ok := wildcardForDomain(certDomainName(domain))
			if !ok {
				continue
			}
			cfg, ok := certificateRequestConfigForDomain(wildcardDomain, creds)
			if !ok || strings.TrimSpace(cfg.CredentialID) == "" {
				continue
			}
			credentialID = cfg.CredentialID
			if issuer.Provider == "" || issuer.Provider == "auto" {
				issuer = cfg.Issuer
			}
			break
		}
	}
	cred, ok := findCredential(credentialID, creds)
	if !ok {
		return fmt.Errorf("凭据 %s 不存在", credentialID)
	}
	if !isDNSCredentialProvider(cred.Provider) {
		return fmt.Errorf("凭据 %s 不是 ACME DNS 凭据", credentialID)
	}
	b.WriteString(renderTLSBlock(cred, issuer, "    "))
	return nil
}

func autoCertificateSiteForDomain(s Site, domain string) Site {
	out := s
	out.Domain = domain
	out.CertificateMode = ""
	out.CertificateBindings = nil
	out.CustomCertFile = ""
	out.CustomKeyFile = ""
	if cfg, ok := certificateConfigForDomain(domain); ok {
		out.Issuer = cfg.Issuer
		out.CredentialID = cfg.CredentialID
		out.ChallengePref = cfg.ChallengePref
		out.Wildcard = strings.HasPrefix(certDomainName(cfg.Domain), "*.")
	}
	return out
}

func certificateBoundSiteForDomain(s Site, domain string) Site {
	return certificateBoundSiteForDomainWithCredentials(s, domain, nil)
}

func certificateBoundSiteForDomainWithCredentials(s Site, domain string, creds []Credential) Site {
	out := s
	out.Domain = domain
	out.CertificateMode = ""
	out.CertificateBindings = nil
	binding, ok := certificateBindingForDomain(s.CertificateBindings, domain)
	if !ok {
		return autoCertificateSiteForDomain(out, domain)
	}
	if cfg, ok := wildcardSiteCertificateConfig(s, domain, binding, creds); ok {
		out.Issuer = cfg.Issuer
		out.CredentialID = cfg.CredentialID
		out.ChallengePref = cfg.ChallengePref
		out.Wildcard = true
		out.CustomCertFile = ""
		out.CustomKeyFile = ""
		return out
	}
	if binding.Mode == "auto" || binding.CertificateID == 0 {
		if cfg, ok := certificateRequestConfigForDomain(domain, creds); ok {
			out.Issuer = cfg.Issuer
			out.CredentialID = cfg.CredentialID
			out.ChallengePref = "http"
			if cfg.SignMethod == "DNS-01" {
				out.ChallengePref = "dns"
			}
			out.Wildcard = strings.HasPrefix(certDomainName(domain), "*.")
			return out
		}
		return autoCertificateSiteForDomain(out, domain)
	}
	if cfg, ok := selectedBindingCertificateConfig(binding); ok {
		out.Issuer = cfg.Issuer
		out.CredentialID = cfg.CredentialID
		out.ChallengePref = cfg.ChallengePref
		out.Wildcard = strings.HasPrefix(certDomainName(cfg.Domain), "*.")
		out.CustomCertFile = cfg.CustomCertFile
		out.CustomKeyFile = cfg.CustomKeyFile
		return out
	}
	if cfg, ok := certificateConfigFromBinding(binding); ok {
		out.Issuer = cfg.Issuer
		out.CredentialID = cfg.CredentialID
		out.ChallengePref = cfg.ChallengePref
		out.Wildcard = strings.HasPrefix(certDomainName(cfg.Domain), "*.")
		if !out.Wildcard && binding.CertificateID > 0 {
			if certCfg, ok := certificateConfigForID(binding.CertificateID); ok {
				out.Wildcard = strings.HasPrefix(certDomainName(certCfg.Domain), "*.")
			}
		}
		return out
	}
	if cfg, ok := certificateConfigForID(binding.CertificateID); ok {
		out.Issuer = cfg.Issuer
		out.CredentialID = cfg.CredentialID
		out.ChallengePref = cfg.ChallengePref
		out.Wildcard = strings.HasPrefix(certDomainName(cfg.Domain), "*.")
		out.CustomCertFile = cfg.CustomCertFile
		out.CustomKeyFile = cfg.CustomKeyFile
		return out
	}
	return autoCertificateSiteForDomain(out, domain)
}

func wildcardSiteCertificateConfig(s Site, domain string, binding CertificateBinding, creds []Credential) (domainCertificateConfig, bool) {
	if !s.Wildcard {
		return domainCertificateConfig{}, false
	}
	wildcardDomain, ok := wildcardForDomain(certDomainName(domain))
	if !ok {
		return domainCertificateConfig{}, false
	}
	if cfg, ok := certificateConfigForDomain(wildcardDomain); ok && strings.HasPrefix(certDomainName(cfg.Domain), "*.") {
		if cfg.CredentialID == "" {
			if requestCfg, ok := certificateRequestConfigForDomain(wildcardDomain, creds); ok {
				cfg.CredentialID = requestCfg.CredentialID
				if cfg.Issuer.Provider == "" || cfg.Issuer.Provider == "auto" {
					cfg.Issuer = requestCfg.Issuer
				}
				cfg.ChallengePref = defaultString(requestCfg.SignMethod, cfg.ChallengePref)
				if cfg.ChallengePref == "DNS-01" {
					cfg.ChallengePref = "dns"
				}
			}
		}
		return cfg, true
	}
	issuer := s.Issuer
	if issuer.Provider == "" && binding.Issuer.Provider != "" {
		issuer = binding.Issuer
	} else if issuer.Provider == "" && binding.Provider != "" {
		issuer.Provider = binding.Provider
	}
	cfg := domainCertificateConfig{
		Domain:        wildcardDomain,
		ChallengePref: defaultString(firstNonEmpty(s.ChallengePref, binding.ChallengePref), "dns"),
		CredentialID:  firstNonEmpty(s.CredentialID, binding.CredentialID),
		Issuer:        issuer,
	}
	if cfg.CredentialID == "" {
		if requestCfg, ok := certificateRequestConfigForDomain(wildcardDomain, creds); ok {
			cfg.CredentialID = requestCfg.CredentialID
			if cfg.Issuer.Provider == "" || cfg.Issuer.Provider == "auto" {
				cfg.Issuer = requestCfg.Issuer
			}
		}
	}
	return cfg, true
}

func selectedBindingCertificateConfig(binding CertificateBinding) (domainCertificateConfig, bool) {
	if binding.Mode != "selected" || binding.CertificateID <= 0 {
		return domainCertificateConfig{}, false
	}
	if cfg, ok := certificateConfigFromSelectedBinding(binding); ok {
		return cfg, true
	}
	cfg, ok := certificateConfigForID(binding.CertificateID)
	if !ok {
		return domainCertificateConfig{}, false
	}
	return mergeBindingCertificateConfig(cfg, binding), true
}

func selectedBindingCertificateConfigFromRows(binding CertificateBinding, rows []CertOverview, sanMap map[string]certSAN) (domainCertificateConfig, bool) {
	if binding.Mode != "selected" || binding.CertificateID <= 0 {
		return domainCertificateConfig{}, false
	}
	if cfg, ok := certificateConfigFromSelectedBinding(binding); ok {
		return cfg, true
	}
	for _, row := range rows {
		if stableID("cert:"+certDomainName(row.Domain)) != binding.CertificateID {
			continue
		}
		return mergeBindingCertificateConfig(domainCertificateConfigFromRow(row), binding), true
	}
	for domain, info := range sanMap {
		if stableID("cert:"+certDomainName(domain)) != binding.CertificateID {
			continue
		}
		cfg := domainCertificateConfig{
			Domain:        domain,
			ChallengePref: defaultCertificateChallengePref(domain, ""),
			Issuer:        IssuerConfig{Provider: info.Provider},
		}
		return mergeBindingCertificateConfig(cfg, binding), true
	}
	return domainCertificateConfig{}, false
}

func certificateConfigFromSelectedBinding(binding CertificateBinding) (domainCertificateConfig, bool) {
	domain := certDomainName(binding.CertificateDomain)
	if domain == "" && binding.NiceName != "" && stableID("cert:"+certDomainName(binding.NiceName)) == binding.CertificateID {
		domain = certDomainName(binding.NiceName)
	}
	if domain == "" || stableID("cert:"+domain) != binding.CertificateID {
		return domainCertificateConfig{}, false
	}
	return mergeBindingCertificateConfig(domainCertificateConfig{
		Domain:        domain,
		ChallengePref: defaultCertificateChallengePref(domain, binding.ChallengePref),
		CredentialID:  binding.CredentialID,
		Issuer:        binding.Issuer,
	}, binding), true
}

func defaultCertificateChallengePref(domain string, preferred string) string {
	if strings.TrimSpace(preferred) != "" {
		return preferred
	}
	if strings.HasPrefix(certDomainName(domain), "*.") {
		return "dns"
	}
	return "http"
}

func mergeBindingCertificateConfig(cfg domainCertificateConfig, binding CertificateBinding) domainCertificateConfig {
	if cfg.ChallengePref == "" || cfg.ChallengePref == "http" {
		if strings.TrimSpace(binding.ChallengePref) != "" {
			cfg.ChallengePref = binding.ChallengePref
		}
	}
	if cfg.CredentialID == "" {
		cfg.CredentialID = binding.CredentialID
	}
	if cfg.Issuer.Provider == "" || cfg.Issuer.Provider == "auto" {
		if binding.Issuer.Provider != "" {
			cfg.Issuer = binding.Issuer
		} else if binding.Provider != "" {
			cfg.Issuer.Provider = binding.Provider
		}
	}
	return cfg
}

func certificateRequestConfigForDomain(domain string, creds []Credential) (certRequestConfig, bool) {
	if creds == nil {
		creds, _ = loadCredentials()
	}
	cfg, ok := loadCertificateRequestConfigs(creds)[stableID("cert:"+certDomainName(domain))]
	return cfg, ok
}

func certificateConfigFromBinding(binding CertificateBinding) (domainCertificateConfig, bool) {
	issuer := binding.Issuer
	if issuer.Provider == "" {
		issuer.Provider = binding.Provider
	}
	hasConfig := issuer.Provider != "" || binding.CredentialID != "" || binding.ChallengePref != ""
	if !hasConfig {
		return domainCertificateConfig{}, false
	}
	return domainCertificateConfig{
		Domain:        binding.Domain,
		ChallengePref: defaultString(binding.ChallengePref, "http"),
		CredentialID:  binding.CredentialID,
		Issuer:        issuer,
	}, true
}

func certificateBindingForDomain(bindings []CertificateBinding, domain string) (CertificateBinding, bool) {
	key := certDomainKey(domain)
	for _, binding := range bindings {
		if certDomainKey(binding.Domain) == key {
			return binding, true
		}
	}
	return CertificateBinding{}, false
}

type domainCertificateConfig struct {
	Domain         string
	ChallengePref  string
	CredentialID   string
	Issuer         IssuerConfig
	CustomCertFile string
	CustomKeyFile  string
}

func certificateConfigForID(id int) (domainCertificateConfig, bool) {
	cert, ok := npmCertificateByID(id)
	if !ok {
		return domainCertificateConfig{}, false
	}
	if cert.Provider == "other" {
		for _, rec := range mustLoadCustomCertRecords() {
			if rec.ID == id && rec.CertFile != "" && rec.KeyFile != "" {
				return domainCertificateConfig{
					Domain:         firstDomain(cert.DomainNames),
					ChallengePref:  "http",
					CustomCertFile: rec.CertFile,
					CustomKeyFile:  rec.KeyFile,
				}, true
			}
		}
		return domainCertificateConfig{}, false
	}
	issuer, err := issuerFromNPM(cert.Provider, cert.Meta)
	if err != nil {
		issuer = IssuerConfig{Provider: cert.Provider}
	}
	challenge := "http"
	if v, ok := cert.Meta["sign_method"].(string); ok && v == "DNS-01" {
		challenge = "dns"
	}
	credentialID := ""
	if v, ok := cert.Meta["credential_id"].(string); ok {
		credentialID = v
	}
	return domainCertificateConfig{
		Domain:        firstDomain(cert.DomainNames),
		ChallengePref: challenge,
		CredentialID:  credentialID,
		Issuer:        issuer,
	}, true
}

func certificateConfigForDomain(domain string) (domainCertificateConfig, bool) {
	domain = certDomainName(domain)
	rows := certOverviewRows()
	var exact *CertOverview
	var wildcard *CertOverview
	for i := range rows {
		rowDomain := certDomainName(rows[i].Domain)
		if strings.EqualFold(rowDomain, domain) {
			exact = &rows[i]
			continue
		}
		if !strings.HasPrefix(rowDomain, "*.") || !certCoversHost(rowDomain, domain) {
			continue
		}
		if wildcard == nil || len(rows[i].LinkedSites) > len(wildcard.LinkedSites) {
			wildcard = &rows[i]
		}
	}
	if exact != nil {
		if wildcard != nil && !strings.HasPrefix(domain, "*.") && len(exact.LinkedSites) == 0 {
			return domainCertificateConfigFromRow(*wildcard), true
		}
		return domainCertificateConfigFromRow(*exact), true
	}
	if wildcard != nil {
		return domainCertificateConfigFromRow(*wildcard), true
	}
	return domainCertificateConfig{}, false
}

func domainCertificateConfigFromRow(row CertOverview) domainCertificateConfig {
	challenge := "http"
	if row.SignMethod == "DNS-01" {
		challenge = "dns"
	}
	issuer := row.IssuerConfig
	if issuer.Provider == "" {
		issuer.Provider = row.Provider
	}
	return domainCertificateConfig{
		Domain:        row.Domain,
		ChallengePref: challenge,
		CredentialID:  row.CredentialID,
		Issuer:        issuer,
	}
}

func certificateOverviewDomainsForSite(s Site) []string {
	if s.CustomCertFile != "" || s.CustomKeyFile != "" {
		for _, rec := range mustLoadCustomCertRecords() {
			if rec.CertFile == s.CustomCertFile && rec.KeyFile == s.CustomKeyFile {
				return certDomainNames(rec.DomainNames)
			}
		}
		return nil
	}
	if s.Wildcard {
		out := []string{}
		seen := map[string]bool{}
		for _, domain := range splitDomains(s.Domain) {
			wildcardDomain, ok := wildcardForDomain(domain)
			if !ok {
				continue
			}
			key := certDomainKey(wildcardDomain)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, wildcardDomain)
		}
		if len(out) > 0 {
			return out
		}
	}
	return splitDomains(s.Domain)
}

func certificateConfigForSite(s Site) (domainCertificateConfig, bool) {
	if s.CustomCertFile != "" || s.CustomKeyFile != "" {
		for _, rec := range mustLoadCustomCertRecords() {
			if rec.CertFile == s.CustomCertFile && rec.KeyFile == s.CustomKeyFile {
				return domainCertificateConfig{
					Domain:         firstDomain(rec.DomainNames),
					ChallengePref:  defaultString(s.ChallengePref, "http"),
					CustomCertFile: rec.CertFile,
					CustomKeyFile:  rec.KeyFile,
				}, true
			}
		}
		return domainCertificateConfig{}, false
	}
	return domainCertificateConfig{
		Domain:        firstDomain(splitDomains(s.Domain)),
		ChallengePref: s.ChallengePref,
		CredentialID:  s.CredentialID,
		Issuer:        s.Issuer,
	}, true
}

func issuerNeedsTLSBlock(issuer IssuerConfig) bool {
	normalized, _ := normalizeIssuerConfig(issuer)
	return normalized.Provider != "auto" || normalized.CADirectory != "" || normalized.EABKeyID != "" || normalized.ZeroSSLAPIKey != ""
}

func renderTLSBlock(c Credential, issuer IssuerConfig, indent string) string {
	issuer, _ = normalizeIssuerConfig(issuer)
	var b strings.Builder
	if issuerNeedsContactEmail(issuer) {
		settings, _ := loadSystemSettings()
		fmt.Fprintf(&b, "%stls %s {\n", indent, caddyQuote(settings.ACMEContactEmail))
	} else {
		fmt.Fprintf(&b, "%stls {\n", indent)
	}
	hasDNSProvider := c.Provider == "alidns" || c.Provider == "cloudflare" || c.Provider == "dnspod" || c.Provider == "he"
	if issuer.Provider == "internal" {
		fmt.Fprintf(&b, "%s    issuer internal\n", indent)
	} else if issuer.Provider == "auto" {
		// Use Caddy's default automatic HTTPS issuer chain.
	} else if issuer.Provider == "zerossl" && !hasDNSProvider && issuer.ZeroSSLAPIKey != "" {
		fmt.Fprintf(&b, "%s    issuer zerossl %s\n", indent, caddyQuote(issuer.ZeroSSLAPIKey))
	} else {
		if issuer.CADirectory != "" {
			fmt.Fprintf(&b, "%s    ca %s\n", indent, caddyQuote(issuer.CADirectory))
		}
		if issuer.EABKeyID != "" && issuer.EABMACKey != "" {
			fmt.Fprintf(&b, "%s    eab %s %s\n", indent, caddyQuote(issuer.EABKeyID), caddyQuote(issuer.EABMACKey))
		}
	}
	switch c.Provider {
	case "alidns":
		fmt.Fprintf(&b, "%s    dns alidns {\n", indent)
		fmt.Fprintf(&b, "%s        access_key_id %s\n", indent, caddyQuote(c.AliyunKey))
		fmt.Fprintf(&b, "%s        access_key_secret %s\n", indent, caddyQuote(c.AliyunSecret))
		fmt.Fprintf(&b, "%s    }\n", indent)
	case "cloudflare":
		fmt.Fprintf(&b, "%s    dns cloudflare %s\n", indent, caddyQuote(c.CFToken))
	case "dnspod":
		fmt.Fprintf(&b, "%s    dns dnspod %s\n", indent, caddyQuote(c.DNSPodToken))
	case "he":
		fmt.Fprintf(&b, "%s    dns he {\n", indent)
		fmt.Fprintf(&b, "%s        api_key %s\n", indent, caddyQuote(c.HEAPIKey))
		fmt.Fprintf(&b, "%s    }\n", indent)
	}
	if hasDNSProvider {
		// 强制走公共递归 DNS 做 propagation check，避免 Docker embedded DNS 影响 SOA/NS 查询。
		fmt.Fprintf(&b, "%s    resolvers 1.1.1.1 8.8.8.8 223.5.5.5\n", indent)
		// certmagic 默认 2min 超时偏短，慢 NS 给到 10min。
		fmt.Fprintf(&b, "%s    propagation_timeout 10m\n", indent)
	}
	fmt.Fprintf(&b, "%s}\n", indent)
	return b.String()
}

func issuerNeedsContactEmail(issuer IssuerConfig) bool {
	issuer, _ = normalizeIssuerConfig(issuer)
	return issuer.Provider == "google" || issuer.Provider == "zerossl" || issuer.Provider == "custom"
}

func caddyQuote(value string) string {
	return strconv.Quote(value)
}

func renderPlaceholder(wildcardDomain string, cred Credential, issuer IssuerConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s {\n", wildcardDomain)
	b.WriteString("    respond 404\n")
	b.WriteString(renderTLSBlock(cred, issuer, "    "))
	b.WriteString("}\n")
	return b.String()
}

func saveRenderedSite(s Site, creds []Credential) error {
	if s.Name == "" {
		return errors.New("站点名称不能为空")
	}
	conf, err := renderSite(s, creds)
	if err != nil {
		return err
	}
	confPath := filepath.Join(sitesDir, s.Name+confSuffix)
	metaPath := filepath.Join(sitesDir, s.Name+metaSuffix)
	if err := os.WriteFile(confPath, []byte(conf), 0644); err != nil {
		return err
	}
	stampSiteForSave(&s, metaPath)
	meta, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(metaPath, meta, 0644)
}

func findTLSManagedSite(domain string) (Site, bool, error) {
	sites, err := readAllSites()
	if err != nil {
		return Site{}, false, err
	}
	for _, s := range sites {
		if s.NoTLS {
			continue
		}
		if certDomainMatchesSite(domain, s) {
			return s, true, nil
		}
	}
	return Site{}, false, nil
}

func removeManagedCertPlaceholder(domain string) {
	baseName := managedCertPrefix + strings.TrimPrefix(certDomainName(domain), "*.")
	metaPath := filepath.Join(sitesDir, baseName+metaSuffix)
	data, err := os.ReadFile(metaPath)
	if err == nil {
		var p placeholderMeta
		if json.Unmarshal(data, &p) == nil {
			p.Disabled = true
			meta, _ := json.MarshalIndent(p, "", "  ")
			_ = os.WriteFile(metaPath, meta, 0644)
		}
	}
	_ = os.Remove(filepath.Join(sitesDir, baseName+confSuffix))
}

func deleteManagedCertPlaceholder(domain string) {
	baseName := managedCertPrefix + strings.TrimPrefix(certDomainName(domain), "*.")
	_ = os.Remove(filepath.Join(sitesDir, baseName+confSuffix))
	_ = os.Remove(filepath.Join(sitesDir, baseName+metaSuffix))
}

func cleanupManagedCertConflicts() error {
	sites, err := readAllSites()
	if err != nil {
		return err
	}
	covered := map[string]bool{}
	for _, s := range sites {
		if s.Disabled || s.NoTLS {
			continue
		}
		if s.CustomCertFile != "" || s.CustomKeyFile != "" {
			continue
		}
		for _, d := range splitDomains(s.Domain) {
			covered[certDomainKey(d)] = true
		}
		for _, d := range renderedWildcardDomainsForSite(s) {
			covered[certDomainKey(d)] = true
		}
	}
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if !strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		metaPath := filepath.Join(sitesDir, e.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var p placeholderMeta
		if json.Unmarshal(data, &p) != nil {
			continue
		}
		if covered[certDomainKey(p.Domain)] {
			p.Disabled = true
			meta, _ := json.MarshalIndent(p, "", "  ")
			_ = os.WriteFile(metaPath, meta, 0644)
			_ = os.Remove(filepath.Join(sitesDir, baseName+confSuffix))
		}
	}
	return nil
}

func cleanupLegacyExactManagedCertPlaceholders() error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	acmeExactCreds := map[string]bool{}
	for _, cred := range creds {
		if !strings.HasPrefix(cred.Name, "ACME ") {
			continue
		}
		domain := certDomainName(strings.TrimPrefix(cred.Name, "ACME "))
		if domain != "" && !strings.HasPrefix(domain, "*.") {
			acmeExactCreds[strings.ToLower(domain)] = true
		}
	}
	if len(acmeExactCreds) == 0 {
		return nil
	}
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if !strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		metaPath := filepath.Join(sitesDir, e.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var p placeholderMeta
		if json.Unmarshal(data, &p) != nil {
			continue
		}
		if strings.HasPrefix(certDomainName(p.Domain), "*.") {
			continue
		}
		domainKey := strings.ToLower(certDomainName(p.Domain))
		if !acmeExactCreds[domainKey] {
			continue
		}
		linked, err := domainLinkedBy(p.Domain)
		if err != nil || len(linked) > 0 {
			continue
		}
		_ = os.Remove(filepath.Join(sitesDir, baseName+confSuffix))
		_ = os.Remove(metaPath)
	}
	return nil
}

// 将所有 site .json 用当前凭据重新渲染为 .conf
func renderAllSiteConfs(creds []Credential) error {
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), wildcardGroupPrefix) && strings.HasSuffix(e.Name(), confSuffix) {
			_ = os.Remove(filepath.Join(sitesDir, e.Name()))
		}
	}
	wildcardGroups, groupedBaseNames := wildcardRouteGroupsForEntries(entries, creds)
	wildcardAddressCounts := map[string]int{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sitesDir, e.Name()))
		if err != nil {
			continue
		}
		var s Site
		if json.Unmarshal(data, &s) != nil || s.Disabled {
			continue
		}
		for _, key := range renderedWildcardSiteAddressKeys(s, creds) {
			wildcardAddressCounts[key]++
		}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		metaPath := filepath.Join(sitesDir, e.Name())
		confPath := filepath.Join(sitesDir, baseName+confSuffix)
		if groupedBaseNames[baseName] {
			_ = os.Remove(confPath)
			continue
		}
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		if strings.HasPrefix(baseName, managedCertPrefix) {
			var p placeholderMeta
			if json.Unmarshal(data, &p) != nil {
				continue
			}
			if p.Disabled {
				_ = os.Remove(confPath)
				continue
			}
			cred := Credential{}
			if strings.TrimSpace(p.CredentialID) != "" {
				var ok bool
				cred, ok = findCredential(p.CredentialID, creds)
				if !ok {
					log.Printf("warn: placeholder %s 凭据 %s 不存在，跳过渲染", baseName, p.CredentialID)
					continue
				}
				if !isDNSCredentialProvider(cred.Provider) {
					log.Printf("warn: placeholder %s 凭据 %s 不是 ACME DNS 凭据，跳过渲染", baseName, p.CredentialID)
					continue
				}
			}
			os.WriteFile(confPath, []byte(renderPlaceholder(p.Domain, cred, p.Issuer)), 0644)
		} else {
			var s Site
			if json.Unmarshal(data, &s) != nil {
				continue
			}
			if s.Disabled {
				_ = os.Remove(confPath)
				continue
			}
			useWildcardSiteAddress := true
			for _, key := range renderedWildcardSiteAddressKeys(s, creds) {
				if wildcardAddressCounts[key] > 1 {
					useWildcardSiteAddress = false
					break
				}
			}
			conf, err := renderSiteWithOptions(s, creds, renderSiteOptions{UseWildcardSiteAddress: useWildcardSiteAddress})
			if err != nil {
				log.Printf("warn: render site %s: %v", baseName, err)
				continue
			}
			os.WriteFile(confPath, []byte(conf), 0644)
		}
	}
	for _, group := range wildcardGroups {
		conf, err := renderWildcardRouteGroup(group, creds)
		if err != nil {
			log.Printf("warn: render wildcard group %s: %v", group.Address, err)
			continue
		}
		name := wildcardGroupPrefix + strconv.Itoa(stableID(group.Address)) + confSuffix
		os.WriteFile(filepath.Join(sitesDir, name), []byte(conf), 0644)
	}
	return nil
}

func wildcardRouteGroupsForEntries(entries []os.DirEntry, creds []Credential) (map[string]wildcardRouteGroup, map[string]bool) {
	type candidate struct {
		address string
		items   []namedSite
	}
	candidates := map[string]candidate{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sitesDir, e.Name()))
		if err != nil {
			continue
		}
		var s Site
		if json.Unmarshal(data, &s) != nil || s.Disabled {
			continue
		}
		leaves := expandSiteRenderLeaves(s, creds)
		if len(leaves) != 1 {
			continue
		}
		leaf := leaves[0]
		if strings.TrimSpace(leaf.Name) == "" {
			leaf.Name = baseName
		}
		address, ok := wildcardRouteGroupAddress(leaf)
		if !ok {
			continue
		}
		key := address + "\x00" + wildcardTLSCompatibilityKey(leaf)
		item := namedSite{BaseName: baseName, Site: leaf}
		current := candidates[key]
		current.address = address
		current.items = append(current.items, item)
		candidates[key] = current
	}
	groupKeysByAddress := map[string][]string{}
	for key, item := range candidates {
		if len(item.items) <= 1 {
			continue
		}
		groupKeysByAddress[item.address] = append(groupKeysByAddress[item.address], key)
	}
	groups := map[string]wildcardRouteGroup{}
	groupedBaseNames := map[string]bool{}
	for address, keys := range groupKeysByAddress {
		if len(keys) != 1 {
			continue
		}
		item := candidates[keys[0]]
		groups[address] = wildcardRouteGroup{Address: address, Items: item.items}
		for _, named := range item.items {
			groupedBaseNames[named.BaseName] = true
		}
	}
	return groups, groupedBaseNames
}

func recoverOrphanCertificateRequests(creds []Credential) error {
	issued := scanIssuedCerts()
	for _, cred := range creds {
		if !isDNSCredentialProvider(cred.Provider) {
			continue
		}
		if !strings.HasPrefix(cred.Name, "ACME ") {
			continue
		}
		domain := certDomainName(strings.TrimPrefix(cred.Name, "ACME "))
		if domain == "" || !strings.HasPrefix(domain, "*.") {
			continue
		}
		if _, ok := issued[strings.ToLower(domain)]; ok {
			continue
		}
		baseName := managedCertPrefix + strings.TrimPrefix(domain, "*.")
		metaPath := filepath.Join(sitesDir, baseName+metaSuffix)
		if _, err := os.Stat(metaPath); err == nil {
			continue
		}
		issuer := cred.Issuer
		if issuer.Provider == "" {
			issuer = inferIssuerForDomain(domain)
		}
		meta := placeholderMeta{Domain: domain, CredentialID: cred.ID, Issuer: issuer}
		stampPlaceholderForSave(&meta, metaPath)
		metaBytes, _ := json.MarshalIndent(meta, "", "  ")
		if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
			return err
		}
	}
	return nil
}

func inferIssuerForDomain(domain string) IssuerConfig {
	data, err := os.ReadFile(logFile)
	if err == nil && strings.Contains(string(data), domain) && strings.Contains(string(data), "dv.acme-v02.api.pki.goog") {
		if cfg, ok := loadSavedIssuerConfig("google"); ok {
			return cfg
		}
		return IssuerConfig{Provider: "google"}
	}
	return IssuerConfig{}
}

// ============================================================
// 通配符占位同步
// ============================================================

// collectNeededWildcards 返回仍需要独立占位站点管理的 parent wildcard -> credential_id。
// active 代理/重定向/404 站点会直接用 *.example.com 作为 Caddy site address，
// 不再额外生成 __cert_ 占位块，否则 Caddy 会同时看到两个 wildcard 站点。
func collectNeededWildcards(creds []Credential) (map[string]string, error) {
	_ = creds
	return map[string]string{}, nil
}

func renderedWildcardDomainsForSite(s Site) []string {
	if s.Disabled || s.NoTLS {
		return nil
	}
	if s.CustomCertFile != "" || s.CustomKeyFile != "" {
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	add := func(domain string) {
		domain = certDomainName(domain)
		if !strings.HasPrefix(domain, "*.") {
			return
		}
		key := strings.ToLower(domain)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, domain)
	}
	for _, domain := range splitDomains(s.Domain) {
		host := certDomainName(domain)
		if host == "" {
			continue
		}
		if strings.HasPrefix(host, "*.") {
			add(host)
			continue
		}
		if wildcardDomain, ok := wildcardDomainRenderedForHost(s, host); ok {
			add(wildcardDomain)
		}
	}
	return out
}

func wildcardDomainRenderedForHost(s Site, host string) (string, bool) {
	if s.Wildcard {
		return wildcardForDomain(host)
	}
	binding, ok := certificateBindingForDomain(s.CertificateBindings, host)
	if !ok || binding.Mode != "selected" || binding.CertificateID <= 0 {
		return "", false
	}
	for _, candidate := range []string{binding.CertificateDomain, binding.NiceName} {
		candidate = certDomainName(candidate)
		if !strings.HasPrefix(candidate, "*.") || !certCoversHost(candidate, host) {
			continue
		}
		if stableID("cert:"+candidate) == binding.CertificateID {
			return candidate, true
		}
	}
	if wildcardDomain, ok := wildcardForDomain(host); ok && stableID("cert:"+wildcardDomain) == binding.CertificateID {
		return wildcardDomain, true
	}
	return "", false
}

func collectExistingPlaceholders() (map[string]string, error) {
	out := map[string]string{}
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if !strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(sitesDir, e.Name()))
		var p placeholderMeta
		if json.Unmarshal(data, &p) != nil {
			continue
		}
		if p.Disabled {
			continue
		}
		out[p.Domain] = baseName
	}
	return out, nil
}

type fileBackup struct {
	path string
	data []byte // nil 表示原本不存在
}

func restoreBackups(backups []fileBackup) {
	for _, b := range backups {
		if b.data == nil {
			os.Remove(b.path)
		} else {
			os.WriteFile(b.path, b.data, 0644)
		}
	}
}

func snapshotAll() ([]fileBackup, error) {
	var backups []fileBackup
	entries, err := os.ReadDir(sitesDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(sitesDir, e.Name())
			data, err := os.ReadFile(path)
			if err == nil {
				backups = append(backups, fileBackup{path: path, data: data})
			}
		}
	}
	if data, err := os.ReadFile(credentialPath); err == nil {
		backups = append(backups, fileBackup{path: credentialPath, data: data})
	}
	return backups, nil
}

func syncWildcardPlaceholders(creds []Credential) ([]fileBackup, error) {
	needed, err := collectNeededWildcards(creds)
	if err != nil {
		return nil, err
	}
	existing, err := collectExistingPlaceholders()
	if err != nil {
		return nil, err
	}
	renderedWildcards, err := collectRenderedWildcardDomains()
	if err != nil {
		return nil, err
	}
	var backups []fileBackup
	for w, credID := range needed {
		if _, ok := existing[w]; ok {
			continue
		}
		cred, okC := findCredential(credID, creds)
		if !okC {
			return backups, fmt.Errorf("通配符 %s 关联的凭据 %s 不存在", w, credID)
		}
		if !isDNSCredentialProvider(cred.Provider) {
			return backups, fmt.Errorf("通配符 %s 关联的凭据 %s 不是 ACME DNS 凭据", w, credID)
		}
		baseName := managedCertPrefix + strings.TrimPrefix(w, "*.")
		confPath := filepath.Join(sitesDir, baseName+confSuffix)
		metaPath := filepath.Join(sitesDir, baseName+metaSuffix)
		backups = append(backups, fileBackup{path: confPath, data: nil}, fileBackup{path: metaPath, data: nil})
		conf := renderPlaceholder(w, cred, IssuerConfig{})
		p := placeholderMeta{Domain: w, CredentialID: credID}
		stampPlaceholderForSave(&p, metaPath)
		meta, _ := json.MarshalIndent(p, "", "  ")
		if err := os.WriteFile(confPath, []byte(conf), 0644); err != nil {
			return backups, err
		}
		if err := os.WriteFile(metaPath, meta, 0644); err != nil {
			return backups, err
		}
	}
	for w, baseName := range existing {
		if _, ok := needed[w]; ok {
			continue
		}
		if !renderedWildcards[certDomainKey(w)] {
			continue
		}
		confPath := filepath.Join(sitesDir, baseName+confSuffix)
		metaPath := filepath.Join(sitesDir, baseName+metaSuffix)
		oldConf, _ := os.ReadFile(confPath)
		oldMeta, _ := os.ReadFile(metaPath)
		backups = append(backups, fileBackup{path: confPath, data: oldConf}, fileBackup{path: metaPath, data: oldMeta})
		os.Remove(confPath)
		os.Remove(metaPath)
	}
	return backups, nil
}

func collectRenderedWildcardDomains() (map[string]bool, error) {
	out := map[string]bool{}
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sitesDir, e.Name()))
		if err != nil {
			continue
		}
		var s Site
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		for _, domain := range renderedWildcardDomainsForSite(s) {
			out[certDomainKey(domain)] = true
		}
	}
	return out, nil
}

func wildcardForDomain(d string) (string, bool) {
	if strings.HasPrefix(d, "*.") {
		return "", false
	}
	parts := strings.Split(d, ".")
	if len(parts) < 2 {
		return "", false
	}
	return "*." + strings.Join(parts[1:], "."), true
}

func rollback(path string, old []byte) {
	if old == nil {
		os.Remove(path)
		return
	}
	os.WriteFile(path, old, 0644)
}

// ============================================================
// 证书全景
// ============================================================

func certsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	out := certOverviewRows()
	sort.Slice(out, func(i, j int) bool { return out[i].Domain < out[j].Domain })
	writeJSON(w, http.StatusOK, out)
}

func scanIssuedCerts() map[string]certSAN {
	out := map[string]certSAN{}
	filepath.Walk(caddyCertsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".crt") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		block, _ := pem.Decode(data)
		if block == nil {
			return nil
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil
		}
		provider := "auto"
		normalizedPath := strings.ToLower(filepath.ToSlash(path))
		switch {
		case strings.Contains(normalizedPath, "dv.acme-v02.api.pki.goog"):
			provider = "google"
		case strings.Contains(normalizedPath, "acme-staging-v02.api.letsencrypt.org"):
			provider = "letsencrypt-staging"
		case strings.Contains(normalizedPath, "acme-v02.api.letsencrypt.org"):
			provider = "letsencrypt"
		case strings.Contains(normalizedPath, "acme.zerossl.com"):
			provider = "zerossl"
		}
		names := append([]string{}, cert.DNSNames...)
		if cert.Subject.CommonName != "" {
			names = append(names, cert.Subject.CommonName)
		}
		seenNames := map[string]bool{}
		for _, d := range names {
			d = certDomainName(d)
			if d == "" {
				continue
			}
			d = strings.ToLower(d)
			if seenNames[d] {
				continue
			}
			seenNames[d] = true
			existing, ok := out[d]
			if !ok || cert.NotAfter.After(existing.NotAfter) {
				out[d] = certSAN{NotAfter: cert.NotAfter, NotBefore: cert.NotBefore, Issuer: certificateIssuerName(cert), Provider: provider}
			}
		}
		return nil
	})
	return out
}

// certFolderName Caddy 把 *.example.com 落到 wildcard_.example.com
func certFolderName(domain string) string {
	if strings.HasPrefix(domain, "*.") {
		return "wildcard_" + strings.TrimPrefix(domain, "*")
	}
	return domain
}

func clearCertFiles(domain string) (int, error) {
	target := strings.ToLower(certFolderName(domain))
	removed := 0
	if _, err := os.Stat(caddyCertsDir); os.IsNotExist(err) {
		return 0, nil
	}
	var toRemove []string
	err := filepath.Walk(caddyCertsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() && strings.EqualFold(info.Name(), target) {
			toRemove = append(toRemove, path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, p := range toRemove {
		if err := os.RemoveAll(p); err == nil {
			removed++
		}
	}
	return removed, nil
}

func certIssueLockExists(domain string) bool {
	name := "issue_cert_" + strings.ToLower(certFolderName(domain)) + ".lock"
	if _, err := os.Stat(filepath.Join(caddyDataDir, "locks", name)); err == nil {
		return true
	}
	return false
}

func certReissueHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	body.Domain = strings.TrimSpace(body.Domain)
	if body.Domain == "" {
		http.Error(w, "domain required", http.StatusBadRequest)
		return
	}
	removed, err := clearCertFiles(body.Domain)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 触发 Caddy 重新加载，让自动 HTTPS 引擎重新评估
	if err := reloadCaddy(); err != nil {
		http.Error(w, "reload failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"removed_dirs": removed})
}

func certDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	body.Domain = strings.TrimSpace(body.Domain)
	if body.Domain == "" {
		http.Error(w, "domain required", http.StatusBadRequest)
		return
	}
	// 检查是否仍被反代 / 占位引用
	linked, err := domainLinkedBy(body.Domain)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(linked) > 0 {
		http.Error(w, fmt.Sprintf("仍被反代引用：%s。请先删除对应反代或取消通配符勾选。", strings.Join(linked, ", ")), http.StatusBadRequest)
		return
	}
	removed, err := clearCertFiles(body.Domain)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	deleteManagedCertPlaceholder(body.Domain)
	_ = deleteACMECredentialForDomain(body.Domain, "")
	writeJSON(w, http.StatusOK, map[string]int{"removed_dirs": removed})
}

func domainLinkedBy(domain string) ([]string, error) {
	var out []string
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return nil, err
	}
	target := certDomainName(domain)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		data, err := os.ReadFile(filepath.Join(sitesDir, e.Name()))
		if err != nil {
			continue
		}
		if strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		var s Site
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		if siteReferencesCertificateDomain(s, target) {
			out = append(out, s.Name)
		}
	}
	return out, nil
}

func siteReferencesCertificateDomain(s Site, certDomain string) bool {
	if s.NoTLS || s.Disabled {
		return false
	}
	if s.CustomCertFile != "" || s.CustomKeyFile != "" {
		for _, domain := range certificateOverviewDomainsForSite(s) {
			if strings.EqualFold(certDomainName(domain), certDomain) {
				return true
			}
		}
		return false
	}
	for _, siteDomain := range splitDomains(s.Domain) {
		siteDomain = certDomainName(siteDomain)
		if siteDomain == "" {
			continue
		}
		if binding, ok := certificateBindingForDomain(s.CertificateBindings, siteDomain); ok {
			if binding.Mode == "selected" && binding.CertificateID > 0 {
				if cert, ok := npmCertificateByID(binding.CertificateID); ok && certificateHasDomain(cert, certDomain) {
					return true
				}
				continue
			}
			if cfg, ok := certificateConfigForDomain(siteDomain); ok && strings.EqualFold(certDomainName(cfg.Domain), certDomain) {
				return true
			}
			continue
		}
		if s.CertificateMode == "auto" {
			if cfg, ok := certificateConfigForDomain(siteDomain); ok && strings.EqualFold(certDomainName(cfg.Domain), certDomain) {
				return true
			}
			continue
		}
		if s.Wildcard {
			if wildcardDomain, ok := wildcardForDomain(siteDomain); ok && strings.EqualFold(wildcardDomain, certDomain) {
				return true
			}
			continue
		}
		if strings.EqualFold(siteDomain, certDomain) {
			return true
		}
	}
	return false
}

func certificateHasDomain(cert npmCertificate, domain string) bool {
	domain = certDomainName(domain)
	for _, certDomain := range cert.DomainNames {
		if strings.EqualFold(certDomainName(certDomain), domain) {
			return true
		}
	}
	return false
}

// ============================================================
// 探活：DNS 解析 + TCP :80
// ============================================================

func probeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Domains []string `json:"domains"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	type result struct {
		Domain          string   `json:"domain"`
		Resolvable      bool     `json:"resolvable"`
		ResolvedIPs     []string `json:"resolved_ips,omitempty"`
		Port80Reachable bool     `json:"port80_reachable"`
		Note            string   `json:"note,omitempty"`
	}
	results := []result{}
	for _, d := range req.Domains {
		d = strings.TrimSpace(d)
		if d == "" || strings.HasPrefix(d, "*.") {
			continue
		}
		res := result{Domain: d}
		resolver := &net.Resolver{}
		ctxIPs, _ := resolver.LookupHost(r.Context(), d)
		if len(ctxIPs) > 0 {
			res.Resolvable = true
			res.ResolvedIPs = ctxIPs
			for _, ip := range ctxIPs {
				conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "80"), probeTimeout)
				if err == nil {
					conn.Close()
					res.Port80Reachable = true
					break
				}
			}
			if !res.Port80Reachable {
				res.Note = "80 端口外部不可达（IP 已解析但 TCP 连接超时/被拒）"
			}
		} else {
			res.Note = "DNS 解析失败"
		}
		results = append(results, res)
	}
	writeJSON(w, http.StatusOK, results)
}

// ============================================================
// reload / logs
// ============================================================

func reloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := reloadCaddy(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func reloadCaddy() error {
	cf, err := os.ReadFile(caddyfile)
	if err != nil {
		return fmt.Errorf("read Caddyfile: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, caddyAdmin+"/load", bytes.NewReader(cf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/caddyfile")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("call admin /load: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("admin /load returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// ============================================================
// Nginx Proxy Manager UI compatibility
// ============================================================

type npmUser struct {
	ID         int      `json:"id"`
	CreatedOn  string   `json:"created_on"`
	ModifiedOn string   `json:"modified_on"`
	IsDisabled bool     `json:"is_disabled"`
	Email      string   `json:"email"`
	Name       string   `json:"name"`
	Nickname   string   `json:"nickname"`
	Avatar     string   `json:"avatar"`
	Roles      []string `json:"roles"`
}

type auditEntry struct {
	ID         int            `json:"id"`
	CreatedOn  string         `json:"created_on"`
	ModifiedOn string         `json:"modified_on"`
	UserID     int            `json:"user_id"`
	ObjectType string         `json:"object_type"`
	ObjectID   int            `json:"object_id"`
	Action     string         `json:"action"`
	Meta       map[string]any `json:"meta"`
	User       npmUser        `json:"user,omitempty"`
}

type npmProxyLocation struct {
	Path          string `json:"path"`
	ForwardScheme string `json:"forward_scheme"`
	ForwardHost   string `json:"forward_host"`
	ForwardPort   int    `json:"forward_port"`
	ForwardPath   string `json:"forward_path,omitempty"`
}

type npmProxyHost struct {
	ID                         int               `json:"id"`
	CreatedOn                  string            `json:"created_on"`
	ModifiedOn                 string            `json:"modified_on"`
	OwnerUserID                int               `json:"owner_user_id"`
	ServiceName                string            `json:"service_name,omitempty"`
	DomainNames                []string          `json:"domain_names"`
	ListenPort                 int               `json:"listen_port"`
	ListenPorts                []int             `json:"listen_ports,omitempty"`
	ForwardScheme              string            `json:"forward_scheme"`
	ForwardHost                string            `json:"forward_host"`
	ForwardPort                int                `json:"forward_port"`
	Locations                  []npmProxyLocation `json:"locations,omitempty"`
	AccessListID               int                `json:"access_list_id"`
	CertificateID              any               `json:"certificate_id"`
	SSLForced                  bool              `json:"ssl_forced"`
	CachingEnabled             bool              `json:"caching_enabled"`
	BlockExploits              bool              `json:"block_exploits"`
	AdvancedConfig             string            `json:"advanced_config"`
	Meta                       map[string]any    `json:"meta"`
	AllowWebsocketUpgrade      bool              `json:"allow_websocket_upgrade"`
	HTTP2Support               bool              `json:"http2_support"`
	Enabled                    bool              `json:"enabled"`
	HSTSEnabled                bool              `json:"hsts_enabled"`
	HSTSSubdomains             bool              `json:"hsts_subdomains"`
	TrustForwardedProto        bool              `json:"trust_forwarded_proto"`
	ForwardAuth                ForwardAuthConfig `json:"forward_auth,omitempty"`
	UpstreamInsecureSkipVerify bool              `json:"upstream_insecure_skip_verify,omitempty"`
	Owner                      npmUser           `json:"owner,omitempty"`
	Certificate                *npmCertificate   `json:"certificate,omitempty"`
	AccessList                 *npmAccessList    `json:"access_list,omitempty"`
}

type npmCertificate struct {
	ID               int                  `json:"id"`
	CreatedOn        string               `json:"created_on"`
	ModifiedOn       string               `json:"modified_on"`
	OwnerUserID      int                  `json:"owner_user_id"`
	Provider         string               `json:"provider"`
	NiceName         string               `json:"nice_name"`
	DomainNames      []string             `json:"domain_names"`
	ExpiresOn        string               `json:"expires_on"`
	Meta             map[string]any       `json:"meta"`
	Owner            npmUser              `json:"owner,omitempty"`
	ProxyHosts       []npmProxyHost       `json:"proxy_hosts,omitempty"`
	RedirectionHosts []npmRedirectionHost `json:"redirection_hosts,omitempty"`
	DeadHosts        []npmDeadHost        `json:"dead_hosts,omitempty"`
	Streams          []npmStream          `json:"streams,omitempty"`
}

type npmRedirectionHost struct {
	ID                int             `json:"id"`
	CreatedOn         string          `json:"created_on"`
	ModifiedOn        string          `json:"modified_on"`
	OwnerUserID       int             `json:"owner_user_id"`
	DomainNames       []string        `json:"domain_names"`
	ForwardDomainName string          `json:"forward_domain_name"`
	PreservePath      bool            `json:"preserve_path"`
	CertificateID     any             `json:"certificate_id"`
	SSLForced         bool            `json:"ssl_forced"`
	BlockExploits     bool            `json:"block_exploits"`
	AdvancedConfig    string          `json:"advanced_config"`
	Meta              map[string]any  `json:"meta"`
	HTTP2Support      bool            `json:"http2_support"`
	ForwardScheme     string          `json:"forward_scheme"`
	ForwardHTTPCode   int             `json:"forward_http_code"`
	Enabled           bool            `json:"enabled"`
	HSTSEnabled       bool            `json:"hsts_enabled"`
	HSTSSubdomains    bool            `json:"hsts_subdomains"`
	Owner             npmUser         `json:"owner,omitempty"`
	Certificate       *npmCertificate `json:"certificate,omitempty"`
}

type npmDeadHost struct {
	ID             int             `json:"id"`
	CreatedOn      string          `json:"created_on"`
	ModifiedOn     string          `json:"modified_on"`
	OwnerUserID    int             `json:"owner_user_id"`
	DomainNames    []string        `json:"domain_names"`
	CertificateID  any             `json:"certificate_id"`
	SSLForced      bool            `json:"ssl_forced"`
	AdvancedConfig string          `json:"advanced_config"`
	Meta           map[string]any  `json:"meta"`
	HTTP2Support   bool            `json:"http2_support"`
	Enabled        bool            `json:"enabled"`
	HSTSEnabled    bool            `json:"hsts_enabled"`
	HSTSSubdomains bool            `json:"hsts_subdomains"`
	Owner          npmUser         `json:"owner,omitempty"`
	Certificate    *npmCertificate `json:"certificate,omitempty"`
}

type npmStream struct {
	ID             int            `json:"id"`
	CreatedOn      string         `json:"created_on"`
	ModifiedOn     string         `json:"modified_on"`
	OwnerUserID    int            `json:"owner_user_id"`
	IncomingPort   int            `json:"incoming_port"`
	ForwardingHost string         `json:"forwarding_host"`
	ForwardingPort int            `json:"forwarding_port"`
	TCPForwarding  bool           `json:"tcp_forwarding"`
	UDPForwarding  bool           `json:"udp_forwarding"`
	Meta           map[string]any `json:"meta"`
	Enabled        bool           `json:"enabled"`
	CertificateID  any            `json:"certificate_id"`
	Owner          npmUser        `json:"owner,omitempty"`
}

type npmDynamicDNS struct {
	ID            int            `json:"id"`
	CreatedOn     string         `json:"created_on"`
	ModifiedOn    string         `json:"modified_on"`
	OwnerUserID   int            `json:"owner_user_id"`
	Name          string         `json:"name"`
	DomainNames   []string       `json:"domain_names"`
	CredentialID  string         `json:"credential_id"`
	IPv4          bool           `json:"ipv4"`
	IPv6          bool           `json:"ipv6"`
	CheckInterval string         `json:"check_interval"`
	TTL           string         `json:"ttl"`
	Resolvers     []string       `json:"resolvers"`
	IPServiceURL  string         `json:"ip_service_url"`
	DNSProvider   string         `json:"dns_provider,omitempty"`
	Meta          map[string]any `json:"meta"`
	Enabled       bool           `json:"enabled"`
	Owner         npmUser        `json:"owner,omitempty"`
}

type cloudflareListResponse struct {
	Success bool                  `json:"success"`
	Errors  []cloudflareAPIError  `json:"errors"`
	Result  []cloudflareDNSRecord `json:"result"`
	Info    cloudflareResultInfo  `json:"result_info"`
}

type cloudflareRecordResponse struct {
	Success bool                 `json:"success"`
	Errors  []cloudflareAPIError `json:"errors"`
	Result  cloudflareDNSRecord  `json:"result"`
}

type cloudflareDNSRecord struct {
	ID      string `json:"id"`
	ZoneID  string `json:"zone_id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl,omitempty"`
	Proxied bool   `json:"proxied,omitempty"`
}

type cloudflareAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareResultInfo struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalPages int `json:"total_pages"`
	Count      int `json:"count"`
	TotalCount int `json:"total_count"`
}

type dynamicDNSRecordTarget struct {
	Zone string
	RR   string
	Host string
}

type aliyunDNSRecordsResponse struct {
	RequestID    string `json:"RequestId"`
	TotalCount   int    `json:"TotalCount"`
	PageNumber   int    `json:"PageNumber"`
	PageSize     int    `json:"PageSize"`
	DomainRecord struct {
		Records []aliyunDNSRecord `json:"Record"`
	} `json:"DomainRecords"`
}

type aliyunDNSRecordResponse struct {
	RequestID string `json:"RequestId"`
	RecordID  string `json:"RecordId"`
}

type aliyunDNSRecord struct {
	RecordID   string `json:"RecordId"`
	RR         string `json:"RR"`
	Type       string `json:"Type"`
	Value      string `json:"Value"`
	TTL        int64  `json:"TTL,omitempty"`
	Line       string `json:"Line,omitempty"`
	Priority   int64  `json:"Priority,omitempty"`
	DomainName string `json:"DomainName,omitempty"`
	Status     string `json:"Status,omitempty"`
}

type npmDomainMonitor struct {
	ID                int            `json:"id"`
	CreatedOn         string         `json:"created_on"`
	ModifiedOn        string         `json:"modified_on"`
	OwnerUserID       int            `json:"owner_user_id"`
	Name              string         `json:"name"`
	DomainNames       []string       `json:"domain_names"`
	CheckSSL          bool           `json:"check_ssl"`
	CheckDNS          bool           `json:"check_dns"`
	CheckDomain       bool           `json:"check_domain"`
	CredentialID      string         `json:"credential_id,omitempty"`
	RegistrarProvider string         `json:"registrar_provider,omitempty"`
	ReminderDays      []int          `json:"reminder_days,omitempty"`
	AutoRenew         bool           `json:"auto_renew,omitempty"`
	RenewBefore       int            `json:"renew_before_days,omitempty"`
	CheckInterval     string         `json:"check_interval"`
	ThresholdDays     int            `json:"threshold_days"`
	Resolvers         []string       `json:"resolvers"`
	Meta              map[string]any `json:"meta"`
	Enabled           bool           `json:"enabled"`
	Owner             npmUser        `json:"owner,omitempty"`
}

type npmWakeDevice struct {
	ID               int            `json:"id"`
	CreatedOn        string         `json:"created_on"`
	ModifiedOn       string         `json:"modified_on"`
	OwnerUserID      int            `json:"owner_user_id"`
	Name             string         `json:"name"`
	MACAddress       string         `json:"mac_address"`
	BroadcastAddress string         `json:"broadcast_address"`
	Port             int            `json:"port"`
	SecureOn         string         `json:"secure_on,omitempty"`
	Host             string         `json:"host,omitempty"`
	Description      string         `json:"description,omitempty"`
	Meta             map[string]any `json:"meta"`
	Enabled          bool           `json:"enabled"`
	Owner            npmUser        `json:"owner,omitempty"`
}

type domainMonitorResult struct {
	Domain                  string   `json:"domain"`
	Status                  string   `json:"status"`
	ResolvedIPs             []string `json:"resolved_ips,omitempty"`
	DomainName              string   `json:"domain_name,omitempty"`
	DomainExpiresOn         string   `json:"domain_expires_on,omitempty"`
	DomainDaysLeft          int      `json:"domain_days_left,omitempty"`
	DomainExpiryUnavailable bool     `json:"domain_expiry_unavailable,omitempty"`
	DomainExpiryReason      string   `json:"domain_expiry_unavailable_reason,omitempty"`
	SSLExpiresOn            string   `json:"ssl_expires_on,omitempty"`
	SSLDaysLeft             int      `json:"ssl_days_left,omitempty"`
	SSLIssuer               string   `json:"ssl_issuer,omitempty"`
	SSLSource               string   `json:"ssl_source,omitempty"`
	Error                   string   `json:"error,omitempty"`
}

type npmAccessList struct {
	ID             int                 `json:"id"`
	CreatedOn      string              `json:"created_on"`
	ModifiedOn     string              `json:"modified_on"`
	OwnerUserID    int                 `json:"owner_user_id"`
	Name           string              `json:"name"`
	Meta           map[string]any      `json:"meta"`
	SatisfyAny     bool                `json:"satisfy_any"`
	PassAuth       bool                `json:"pass_auth"`
	ProxyHostCount int                 `json:"proxy_host_count"`
	Owner          npmUser             `json:"owner,omitempty"`
	Items          []npmAccessListItem `json:"items"`
	Clients        []npmAccessClient   `json:"clients"`
}

type npmAccessListItem struct {
	ID           int            `json:"id,omitempty"`
	CreatedOn    string         `json:"created_on,omitempty"`
	ModifiedOn   string         `json:"modified_on,omitempty"`
	AccessListID int            `json:"access_list_id,omitempty"`
	Username     string         `json:"username"`
	Password     string         `json:"password"`
	Meta         map[string]any `json:"meta,omitempty"`
	Hint         string         `json:"hint,omitempty"`
}

type npmAccessClient struct {
	ID           int            `json:"id,omitempty"`
	CreatedOn    string         `json:"created_on,omitempty"`
	ModifiedOn   string         `json:"modified_on,omitempty"`
	AccessListID int            `json:"access_list_id,omitempty"`
	Address      string         `json:"address"`
	Directive    string         `json:"directive"`
	Meta         map[string]any `json:"meta,omitempty"`
}

type customCertRecord struct {
	ID          int            `json:"id"`
	CreatedOn   string         `json:"created_on,omitempty"`
	ModifiedOn  string         `json:"modified_on,omitempty"`
	Name        string         `json:"name"`
	DomainNames []string       `json:"domain_names"`
	CertFile    string         `json:"cert_file"`
	KeyFile     string         `json:"key_file"`
	Meta        map[string]any `json:"meta,omitempty"`
}

var localNPMUser = npmUser{
	ID:         1,
	CreatedOn:  seedTimestamp(),
	ModifiedOn: seedTimestamp(),
	Email:      "local@my-caddy-ui",
	Name:       "本地管理员",
	Nickname:   "本地管理员",
	Avatar:     "/images/default-avatar.jpg",
	Roles:      []string{"admin"},
}

func npmHealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "OK",
		"setup":  true,
		"version": map[string]int{
			"major":    2,
			"minor":    0,
			"revision": 0,
		},
	})
}

func localTimestamp(t time.Time) string {
	loc := time.FixedZone("CST", 8*60*60)
	if t.IsZero() {
		t = time.Now()
	}
	return t.In(loc).Format(time.RFC3339)
}

func seedTimestamp() string {
	return localTimestamp(time.Now())
}

func timestampFromFile(path string) string {
	if info, err := os.Stat(path); err == nil {
		return localTimestamp(info.ModTime())
	}
	return seedTimestamp()
}

func normalizePersistentTimestamps(createdOn, modifiedOn, fallbackPath string) (string, string) {
	if strings.TrimSpace(createdOn) == "" {
		createdOn = timestampFromFile(fallbackPath)
	}
	if strings.TrimSpace(modifiedOn) == "" {
		modifiedOn = createdOn
	}
	return createdOn, modifiedOn
}

func stampSiteForSave(s *Site, existingMetaPath string) {
	if strings.TrimSpace(s.CreatedOn) == "" {
		if existingMetaPath != "" {
			var old Site
			if data, err := os.ReadFile(existingMetaPath); err == nil && json.Unmarshal(data, &old) == nil {
				s.CreatedOn = old.CreatedOn
			}
		}
		if strings.TrimSpace(s.CreatedOn) == "" {
			s.CreatedOn = timestampFromFile(existingMetaPath)
		}
	}
	s.ModifiedOn = localTimestamp(time.Now())
}

func stampPlaceholderForSave(p *placeholderMeta, existingMetaPath string) {
	if strings.TrimSpace(p.CreatedOn) == "" {
		if existingMetaPath != "" {
			var old placeholderMeta
			if data, err := os.ReadFile(existingMetaPath); err == nil && json.Unmarshal(data, &old) == nil {
				p.CreatedOn = old.CreatedOn
			}
		}
		if strings.TrimSpace(p.CreatedOn) == "" {
			p.CreatedOn = timestampFromFile(existingMetaPath)
		}
	}
	p.ModifiedOn = localTimestamp(time.Now())
}

func stampCreatedModified(createdOn *string, modifiedOn *string, fallback string, touchModified bool) {
	if strings.TrimSpace(*createdOn) == "" {
		if strings.TrimSpace(fallback) != "" {
			*createdOn = fallback
		} else {
			*createdOn = seedTimestamp()
		}
	}
	if touchModified {
		*modifiedOn = localTimestamp(time.Now())
		return
	}
	if strings.TrimSpace(*modifiedOn) == "" {
		*modifiedOn = *createdOn
	}
}

func trimAnyPrefix(path string, prefixes ...string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return strings.TrimPrefix(path, prefix)
		}
	}
	return path
}

func npmHostReportHandler(w http.ResponseWriter, r *http.Request) {
	sites, err := readAllSites()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	streams, _ := loadNPMStreams()
	counts := map[string]int{"proxy": 0, "redirection": 0, "stream": len(streams), "dead": 0}
	for _, s := range sites {
		switch s.Kind {
		case "redirection":
			counts["redirection"]++
		case "dead":
			counts["dead"]++
		default:
			counts["proxy"]++
		}
	}
	writeJSON(w, http.StatusOK, counts)
}

func npmEmptyListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	logs, err := loadAuditLogs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func npmAuditLogItemHandler(w http.ResponseWriter, r *http.Request) {
	idText := strings.TrimPrefix(r.URL.Path, "/audit-log/")
	id, _ := strconv.Atoi(idText)
	logs, err := loadAuditLogs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, item := range logs {
		if item.ID == id {
			writeJSON(w, http.StatusOK, item)
			return
		}
	}
	http.Error(w, "audit log not found", http.StatusNotFound)
}

func npmVersionCheckHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"current":          "v2.0.0",
		"latest":           "v2.0.0",
		"update_available": false,
	})
}

func npmAutheliaSettingsHandler(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = "/caddy/settings/forward-auth/authelia"
	npmForwardAuthSettingsHandler(w, r)
}

func npmAuthentikSettingsHandler(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = "/caddy/settings/forward-auth/authentik"
	npmForwardAuthSettingsHandler(w, r)
}

func npmForwardAuthSettingsHandler(w http.ResponseWriter, r *http.Request) {
	provider := trimAnyPrefix(r.URL.Path, "/caddy/settings/forward-auth/", "/nginx/settings/forward-auth/")
	switch r.Method {
	case http.MethodGet:
		switch provider {
		case "authelia":
			cfg, err := loadAutheliaConfig()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, cfg)
		case "authentik":
			cfg, err := loadAuthentikConfig()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, cfg)
		default:
			http.Error(w, "unknown forward_auth provider", http.StatusNotFound)
		}
	case http.MethodPut:
		var err error
		switch provider {
		case "authelia":
			var cfg AutheliaConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
				return
			}
			err = saveAutheliaConfig(cfg)
		case "authentik":
			var cfg AuthentikConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
				return
			}
			err = saveAuthentikConfig(cfg)
		default:
			http.Error(w, "unknown forward_auth provider", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := renderSitesAndReload(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Method = http.MethodGet
		npmForwardAuthSettingsHandler(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func npmProxyHostsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sites, err := readSitesByKind("proxy")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		certByDomain := npmCertificatesByDomain()
		out := make([]npmProxyHost, 0, len(sites))
		for _, s := range sites {
			out = append(out, siteToNPMProxyHost(s, certByDomain))
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var p npmProxyHost
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		p.Enabled = true
		s, err := npmProxyHostToSite(p, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Name = availableSiteName(s.Name)
		if err := saveSiteFromNPM(s); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		appendAuditLog("proxy-host", stableID(s.Name), "created", siteAuditMeta(s))
		writeJSON(w, http.StatusCreated, siteToNPMProxyHost(s, npmCertificatesByDomain()))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func npmProxyHostItemHandler(w http.ResponseWriter, r *http.Request) {
	path := trimAnyPrefix(r.URL.Path, "/caddy/proxy-hosts/", "/nginx/proxy-hosts/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	site, ok, err := siteByNPMID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "proxy host not found", http.StatusNotFound)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPost {
		switch parts[1] {
		case "enable", "disable":
			next := site
			next.Disabled = parts[1] == "disable"
			if err := saveSiteFromNPM(next); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			appendAuditLog("proxy-host", stableID(next.Name), parts[1]+"d", siteAuditMeta(next))
			writeJSON(w, http.StatusOK, true)
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, siteToNPMProxyHost(site, npmCertificatesByDomain()))
	case http.MethodPut:
		var p npmProxyHost
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		p.Enabled = !site.Disabled
		next, err := npmProxyHostToSite(p, site.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		next.CreatedOn = site.CreatedOn
		if err := saveSiteFromNPM(next); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		appendAuditLog("proxy-host", stableID(next.Name), "updated", siteAuditMeta(next))
		writeJSON(w, http.StatusOK, siteToNPMProxyHost(next, npmCertificatesByDomain()))
	case http.MethodDelete:
		deleteSite(w, site.Name)
		appendAuditLog("proxy-host", stableID(site.Name), "deleted", siteAuditMeta(site))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func npmRedirectionHostsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sites, err := readSitesByKind("redirection")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		certByDomain := npmCertificatesByDomain()
		out := make([]npmRedirectionHost, 0, len(sites))
		for _, s := range sites {
			out = append(out, siteToNPMRedirectionHost(s, certByDomain))
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var item npmRedirectionHost
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		item.Enabled = true
		s, err := npmRedirectionHostToSite(item, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := saveSiteFromNPM(s); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		appendAuditLog("redirection-host", stableID(s.Name), "created", siteAuditMeta(s))
		writeJSON(w, http.StatusCreated, siteToNPMRedirectionHost(s, npmCertificatesByDomain()))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func npmRedirectionHostItemHandler(w http.ResponseWriter, r *http.Request) {
	handleSiteBackedItem(w, r, trimAnyPrefix(r.URL.Path, "/caddy/redirection-hosts/", "/nginx/redirection-hosts/"), "redirection", func(s Site) any {
		return siteToNPMRedirectionHost(s, npmCertificatesByDomain())
	}, func(r *http.Request, existing string) (Site, error) {
		var item npmRedirectionHost
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			return Site{}, err
		}
		return npmRedirectionHostToSite(item, existing)
	})
}

func npmDeadHostsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sites, err := readSitesByKind("dead")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		certByDomain := npmCertificatesByDomain()
		out := make([]npmDeadHost, 0, len(sites))
		for _, s := range sites {
			out = append(out, siteToNPMDeadHost(s, certByDomain))
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var item npmDeadHost
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		item.Enabled = true
		s, err := npmDeadHostToSite(item, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := saveSiteFromNPM(s); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		appendAuditLog("dead-host", stableID(s.Name), "created", siteAuditMeta(s))
		writeJSON(w, http.StatusCreated, siteToNPMDeadHost(s, npmCertificatesByDomain()))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func npmDeadHostItemHandler(w http.ResponseWriter, r *http.Request) {
	handleSiteBackedItem(w, r, trimAnyPrefix(r.URL.Path, "/caddy/dead-hosts/", "/nginx/dead-hosts/"), "dead", func(s Site) any {
		return siteToNPMDeadHost(s, npmCertificatesByDomain())
	}, func(r *http.Request, existing string) (Site, error) {
		var item npmDeadHost
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			return Site{}, err
		}
		return npmDeadHostToSite(item, existing)
	})
}

func npmStreamsHandler(w http.ResponseWriter, r *http.Request) {
	items, err := loadNPMStreams()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var item npmStream
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		item.ID = stableID(fmt.Sprintf("stream:%d:%s:%d", item.IncomingPort, item.ForwardingHost, time.Now().UnixNano()))
		item.Enabled = true
		stampNPMStream(&item, "", true)
		items = append(items, item)
		if err := saveJSONFile(streamPath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := syncStreamsAndReload(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		appendAuditLog("stream", item.ID, "created", auditMetaForJSONItem(item))
		writeJSON(w, http.StatusCreated, item)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func npmDynamicDNSHandler(w http.ResponseWriter, r *http.Request) {
	items, err := loadNPMDynamicDNS()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var item npmDynamicDNS
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		item.ID = stableID(fmt.Sprintf("dynamic-dns:%s:%d", item.Name, time.Now().UnixNano()))
		stampNPMDynamicDNS(&item, "", true)
		if err := validateDynamicDNS(item); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items = append(items, item)
		if err := saveJSONFile(dynamicDNSPath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := syncDynamicDNSAndReload(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if creds, err := loadCredentials(); err == nil {
			item.DNSProvider = effectiveDynamicDNSProvider(item, creds)
		}
		appendAuditLog("dynamic-dns", item.ID, "created", auditMetaForJSONItem(item))
		writeJSON(w, http.StatusCreated, item)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func npmDynamicDNSItemHandler(w http.ResponseWriter, r *http.Request) {
	prefix := trimAnyPrefix(r.URL.Path, "/caddy/dynamic-dns/", "/nginx/dynamic-dns/")
	if strings.HasSuffix(strings.Trim(prefix, "/"), "/check") || strings.Contains(strings.Trim(prefix, "/"), "/check/") {
		id, action, ok := parseNPMItemPath(w, r, prefix)
		if !ok {
			return
		}
		if action != "check" {
			http.NotFound(w, r)
			return
		}
		checkDynamicDNSItem(w, r, id)
		return
	}
	handleJSONBackedItem[npmDynamicDNS](w, r, prefix, dynamicDNSPath, loadNPMDynamicDNS, func(item *npmDynamicDNS) {
		stampNPMDynamicDNS(item, "", true)
	}, syncDynamicDNSAndReload)
}

func checkDynamicDNSItem(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := loadNPMDynamicDNS()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	creds, err := loadCredentials()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range items {
		if items[i].ID != id {
			continue
		}
		if err := validateDynamicDNS(items[i]); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if items[i].Meta == nil {
			items[i].Meta = map[string]any{}
		}
		errText, meta := checkDynamicDNSItemStatus(items[i], creds, map[string][]string{})
		now := time.Now().Format(time.RFC3339)
		for key, value := range meta {
			items[i].Meta[key] = value
		}
		items[i].Meta["last_checked"] = now
		if errText == "" {
			dynamicDNSClearStatusError(&items[i])
		} else {
			items[i].Meta["last_error"] = errText
			items[i].Meta["last_error_at"] = now
		}
		items[i].DNSProvider = effectiveDynamicDNSProvider(items[i], creds)
		if err := saveJSONFile(dynamicDNSPath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		appendAuditLog("dynamic-dns", id, "checked", auditMetaForJSONItem(items[i]))
		if errText != "" {
			http.Error(w, errText, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, items[i])
		return
	}
	http.Error(w, "item not found", http.StatusNotFound)
}

func npmDomainMonitorsHandler(w http.ResponseWriter, r *http.Request) {
	items, err := loadNPMDomainMonitors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var item npmDomainMonitor
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		item.ID = stableID(fmt.Sprintf("domain-monitor:%s:%d", item.Name, time.Now().UnixNano()))
		item.Enabled = true
		stampNPMDomainMonitor(&item, "", true)
		if err := validateDomainMonitor(item); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items = append(items, item)
		if err := saveJSONFile(domainMonitorPath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		appendAuditLog("domain-monitor", item.ID, "created", auditMetaForJSONItem(item))
		writeJSON(w, http.StatusCreated, item)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func npmDomainMonitorItemHandler(w http.ResponseWriter, r *http.Request) {
	prefix := trimAnyPrefix(r.URL.Path, "/caddy/domain-monitor/", "/nginx/domain-monitor/")
	if strings.HasSuffix(strings.Trim(prefix, "/"), "/check") || strings.Contains(strings.Trim(prefix, "/"), "/check/") {
		id, action, ok := parseNPMItemPath(w, r, prefix)
		if !ok {
			return
		}
		if action != "check" {
			http.NotFound(w, r)
			return
		}
		checkDomainMonitorItem(w, r, id)
		return
	}
	if strings.HasSuffix(strings.Trim(prefix, "/"), "/renew") || strings.Contains(strings.Trim(prefix, "/"), "/renew/") {
		id, action, ok := parseNPMItemPath(w, r, prefix)
		if !ok {
			return
		}
		if action != "renew" {
			http.NotFound(w, r)
			return
		}
		renewDomainMonitorItem(w, r, id)
		return
	}
	handleJSONBackedItem[npmDomainMonitor](w, r, prefix, domainMonitorPath, loadNPMDomainMonitors, func(item *npmDomainMonitor) {
		stampNPMDomainMonitor(item, "", true)
	}, nil, validateDomainMonitor)
}

func checkDomainMonitorItem(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := loadNPMDomainMonitors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range items {
		if items[i].ID != id {
			continue
		}
		if err := validateDomainMonitor(items[i]); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		runDomainMonitorCheck(&items[i])
		if err := saveJSONFile(domainMonitorPath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		appendAuditLog("domain-monitor", id, "checked", auditMetaForJSONItem(items[i]))
		writeJSON(w, http.StatusOK, items[i])
		return
	}
	http.Error(w, "item not found", http.StatusNotFound)
}

func npmWakeDevicesHandler(w http.ResponseWriter, r *http.Request) {
	items, err := loadNPMWakeDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
			return
		}
		var item npmWakeDevice
		if err := json.Unmarshal(body, &item); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		var raw map[string]json.RawMessage
		if json.Unmarshal(body, &raw) == nil {
			if _, ok := raw["enabled"]; !ok {
				item.Enabled = true
			}
		}
		item.ID = stableID(fmt.Sprintf("wake-device:%s:%s:%d", item.Name, item.MACAddress, time.Now().UnixNano()))
		stampNPMWakeDevice(&item, "", true)
		if err := validateWakeDevice(item); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items = append(items, item)
		if err := saveJSONFile(wakeDevicePath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		appendAuditLog("wake-device", item.ID, "created", auditMetaForJSONItem(item))
		writeJSON(w, http.StatusCreated, item)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func npmWakeDeviceItemHandler(w http.ResponseWriter, r *http.Request) {
	prefix := trimAnyPrefix(r.URL.Path, "/caddy/wake-devices/", "/nginx/wake-devices/")
	if strings.Contains(strings.Trim(prefix, "/"), "/wake") {
		id, action, ok := parseNPMItemPath(w, r, prefix)
		if !ok {
			return
		}
		if action != "wake" {
			http.NotFound(w, r)
			return
		}
		wakeDeviceItem(w, r, id)
		return
	}
	handleJSONBackedItem[npmWakeDevice](w, r, prefix, wakeDevicePath, loadNPMWakeDevices, func(item *npmWakeDevice) {
		stampNPMWakeDevice(item, "", true)
	}, nil, validateWakeDevice)
}

func wakeDeviceItem(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := loadNPMWakeDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range items {
		if items[i].ID != id {
			continue
		}
		var err error
		if !items[i].Enabled {
			err = errors.New("网络唤醒设备已禁用")
		} else if validateErr := validateWakeDevice(items[i]); validateErr != nil {
			err = validateErr
		} else {
			err = sendWakePacket(items[i])
		}
		if items[i].Meta == nil {
			items[i].Meta = map[string]any{}
		}
		now := localTimestamp(time.Now())
		if err != nil {
			items[i].Meta["last_error"] = err.Error()
			items[i].Meta["last_error_at"] = now
			stampNPMWakeDevice(&items[i], "", true)
			if saveErr := saveJSONFile(wakeDevicePath, items); saveErr != nil {
				http.Error(w, saveErr.Error(), http.StatusInternalServerError)
				return
			}
			appendAuditLog("wake-device", id, "wake-failed", auditMetaForJSONItem(items[i]))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items[i].Meta["last_woken_at"] = now
		delete(items[i].Meta, "last_error")
		delete(items[i].Meta, "last_error_at")
		stampNPMWakeDevice(&items[i], "", true)
		if err := saveJSONFile(wakeDevicePath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		appendAuditLog("wake-device", id, "woken", auditMetaForJSONItem(items[i]))
		writeJSON(w, http.StatusOK, items[i])
		return
	}
	http.Error(w, "item not found", http.StatusNotFound)
}

func npmStreamItemHandler(w http.ResponseWriter, r *http.Request) {
	handleJSONBackedItem[npmStream](w, r, trimAnyPrefix(r.URL.Path, "/caddy/streams/", "/nginx/streams/"), streamPath, loadNPMStreams, func(item *npmStream) {
		stampNPMStream(item, "", true)
	}, syncStreamsAndReload)
}

func npmAccessListsHandler(w http.ResponseWriter, r *http.Request) {
	items, err := loadNPMAccessLists()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var item npmAccessList
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		item.ID = stableID(fmt.Sprintf("access:%s:%d", item.Name, time.Now().UnixNano()))
		stampNPMAccessList(&item, "", true)
		items = append(items, item)
		if err := saveJSONFile(accessListPath, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := renderSitesAndReload(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		appendAuditLog("access-list", item.ID, "created", auditMetaForJSONItem(item))
		writeJSON(w, http.StatusCreated, item)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func npmAccessListItemHandler(w http.ResponseWriter, r *http.Request) {
	handleJSONBackedItem[npmAccessList](w, r, trimAnyPrefix(r.URL.Path, "/caddy/access-lists/", "/nginx/access-lists/"), accessListPath, loadNPMAccessLists, func(item *npmAccessList) {
		stampNPMAccessList(item, "", true)
	}, renderSitesAndReload)
}

func npmCertificatesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		certs := npmCertificates()
		writeJSON(w, http.StatusOK, certs)
	case http.MethodPost:
		var item npmCertificate
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		cert, err := createCertificateFromNPM(item)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, cert)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func createCertificateFromNPM(item npmCertificate) (npmCertificate, error) {
	item.DomainNames = certDomainNames(item.DomainNames)
	if item.Provider == "other" {
		if item.NiceName == "" {
			item.NiceName = "自定义证书"
		}
		now := localTimestamp(time.Now())
		rec := customCertRecord{
			ID:          stableID(fmt.Sprintf("custom:%s:%d", item.NiceName, time.Now().UnixNano())),
			CreatedOn:   now,
			ModifiedOn:  now,
			Name:        item.NiceName,
			DomainNames: item.DomainNames,
			Meta:        map[string]any{},
		}
		records, _ := loadCustomCertRecords()
		records = append(records, rec)
		if err := saveJSONFile(customCertMeta, records); err != nil {
			return npmCertificate{}, err
		}
		return customCertToNPM(rec), nil
	}
	if item.Meta != nil {
		if dnsChallenge, _ := item.Meta["dns_challenge"].(bool); dnsChallenge {
			return createACMECertificateFromNPM(item)
		}
		if dnsChallenge, _ := item.Meta["dnsChallenge"].(bool); dnsChallenge {
			return createACMECertificateFromNPM(item)
		}
	}
	return createHTTPACMECertificateFromNPM(item)
}

func createHTTPACMECertificateFromNPM(item npmCertificate) (npmCertificate, error) {
	if len(item.DomainNames) == 0 {
		return npmCertificate{}, errors.New("域名不能为空")
	}
	issuer, err := issuerFromNPM(item.Provider, item.Meta)
	if err != nil {
		return npmCertificate{}, err
	}
	rememberIssuerConfig(issuer)
	for _, domain := range item.DomainNames {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}
		baseName := managedCertPrefix + strings.TrimPrefix(domain, "*.")
		if strings.HasPrefix(domain, "*.") {
			return npmCertificate{}, errors.New("HTTP-01 不支持通配符证书，请使用 DNS-01")
		}
		if site, ok, err := findTLSManagedSite(domain); err != nil {
			return npmCertificate{}, err
		} else if ok {
			site.Issuer = issuer
			if site.ChallengePref == "" {
				site.ChallengePref = "http"
			}
			applyCertificateRequestToSite(&site, domain, issuer, "http", "")
			site.NoTLS = false
			site.LastError = ""
			site.LastErrorAt = time.Time{}
			creds, _ := loadCredentials()
			if err := saveRenderedSite(site, creds); err != nil {
				return npmCertificate{}, err
			}
			removeManagedCertPlaceholder(domain)
			continue
		}
		confPath := filepath.Join(sitesDir, baseName+confSuffix)
		metaPath := filepath.Join(sitesDir, baseName+metaSuffix)
		var b strings.Builder
		fmt.Fprintf(&b, "%s {\n    respond 404\n", domain)
		if issuerNeedsTLSBlock(issuer) {
			b.WriteString(renderTLSBlock(Credential{}, issuer, "    "))
		}
		b.WriteString("}\n")
		meta := placeholderMeta{Domain: domain, Issuer: issuer}
		stampPlaceholderForSave(&meta, metaPath)
		metaBytes, _ := json.MarshalIndent(meta, "", "  ")
		if err := os.WriteFile(confPath, []byte(b.String()), 0644); err != nil {
			return npmCertificate{}, err
		}
		if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
			return npmCertificate{}, err
		}
	}
	if err := cleanupManagedCertConflicts(); err != nil {
		return npmCertificate{}, err
	}
	if err := reloadCaddy(); err != nil {
		return npmCertificate{}, fmt.Errorf("reload failed: %w", err)
	}
	return certOverviewToNPM(CertOverview{
		Domain:       item.DomainNames[0],
		Status:       "pending",
		Provider:     issuer.Provider,
		SignMethod:   "HTTP-01",
		IssuerConfig: issuer,
	}), nil
}

func npmDNSProvidersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	out := []map[string]any{
		{
			"id":          "alidns",
			"name":        "阿里云 DNS",
			"credentials": "ALICLOUD_ACCESS_KEY=<AccessKey ID>\nALICLOUD_SECRET_KEY=<AccessKey Secret>",
			"saved":       false,
		},
		{
			"id":          "cloudflare",
			"name":        "Cloudflare",
			"credentials": "CLOUDFLARE_API_TOKEN=<API Token>",
			"saved":       false,
		},
		{
			"id":          "dnspod",
			"name":        "DNSPod.cn",
			"credentials": "DNSPOD_TOKEN=<APP_ID,APP_TOKEN>",
			"saved":       false,
		},
		{
			"id":          "he",
			"name":        "Hurricane Electric",
			"credentials": "HE_API_KEY=<API Key>",
			"saved":       false,
		},
	}
	if creds, err := loadCredentials(); err == nil {
		seen := map[string]int{}
		counts := map[string]int{}
		names := map[string][]string{}
		for _, c := range creds {
			if !isDNSCredentialProvider(c.Provider) {
				continue
			}
			key := credentialReuseKey(c)
			if key == "" {
				continue
			}
			if idx, ok := seen[key]; ok {
				counts[key]++
				names[key] = append(names[key], c.Name)
				out[idx]["name"] = savedCredentialLabel(fmt.Sprint(out[idx]["provider_name"]), counts[key], names[key], fmt.Sprint(out[idx]["credential_id"]))
				continue
			}
			providerName := dnsProviderDisplayName(c.Provider)
			seen[key] = len(out)
			counts[key] = 1
			names[key] = []string{c.Name}
			out = append(out, map[string]any{
				"id":            "saved:" + c.ID,
				"name":          savedCredentialLabel(providerName, 1, names[key], c.ID),
				"provider_name": providerName,
				"provider":      c.Provider,
				"credentials":   "",
				"credential_id": c.ID,
				"saved":         true,
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func savedCredentialLabel(providerName string, count int, names []string, id string) string {
	if len(id) > 6 {
		id = id[len(id)-6:]
	}
	shown := names
	if len(shown) > 2 {
		shown = shown[:2]
	}
	suffix := strings.Join(shown, "、")
	if count > len(shown) {
		suffix = fmt.Sprintf("%s 等 %d 条", suffix, count)
	}
	return fmt.Sprintf("复用：%s #%s（%s）", providerName, id, suffix)
}

func credentialReuseKey(c Credential) string {
	switch c.Provider {
	case "cloudflare":
		if c.CFToken == "" {
			return ""
		}
		return "cloudflare:" + c.CFToken
	case "alidns":
		if c.AliyunKey == "" || c.AliyunSecret == "" {
			return ""
		}
		return "alidns:" + c.AliyunKey + ":" + c.AliyunSecret
	case "dnspod":
		if c.DNSPodToken == "" {
			return ""
		}
		return "dnspod:" + c.DNSPodToken
	case "he":
		if c.HEAPIKey == "" {
			return ""
		}
		return "he:" + c.HEAPIKey
	default:
		return ""
	}
}

func npmCertificateAuthoritiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	saved, err := loadIssuerCredentials()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	order := []string{"auto", "letsencrypt", "letsencrypt-staging", "zerossl", "google", "custom", "internal"}
	out := make([]map[string]any, 0, len(order))
	for _, id := range order {
		spec, ok := certificateAuthorities[id]
		if !ok {
			continue
		}
		cfg := IssuerConfig{Provider: id}
		_, hasSaved := saved[id]
		if savedCfg, ok := saved[id]; ok {
			cfg = savedCfg
			cfg.Provider = id
		}
		item := publicIssuerMeta(cfg)
		item["id"] = spec.ID
		item["name"] = spec.Name
		if strings.TrimSpace(fmt.Sprint(item["ca_directory"])) == "" || item["ca_directory"] == nil {
			item["ca_directory"] = spec.CADirectory
		}
		item["needs_url"] = spec.NeedsURL
		item["supports_eab"] = spec.SupportsEAB
		item["internal"] = spec.Internal
		item["saved"] = hasSaved
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

func certCredentialID(cert npmCertificate) string {
	if cert.Meta == nil {
		return ""
	}
	if v, ok := cert.Meta["credential_id"].(string); ok {
		return strings.TrimSpace(v)
	}
	if v, ok := cert.Meta["credentialId"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func deleteACMECredentialForDomain(domain string, credentialID string) error {
	domain = certDomainName(domain)
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	next := make([]Credential, 0, len(creds))
	for _, cred := range creds {
		remove := false
		if credentialID != "" && cred.ID == credentialID {
			remove = true
		}
		if strings.EqualFold(strings.TrimSpace(cred.Name), "ACME "+domain) {
			remove = true
		}
		if !remove {
			next = append(next, cred)
		}
	}
	if len(next) == len(creds) {
		return nil
	}
	return saveCredentials(next)
}

func npmTestHTTPCertificateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Domains []string `json:"domains"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	out := map[string]string{}
	for _, d := range req.Domains {
		d = certDomainName(d)
		if d == "" {
			continue
		}
		ips, err := net.LookupHost(d)
		if err != nil || len(ips) == 0 {
			out[d] = "no-host"
			continue
		}
		out[d] = "ok"
	}
	writeJSON(w, http.StatusOK, out)
}

func npmValidateCertificateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad form: "+err.Error(), http.StatusBadRequest)
		return
	}
	_, certHeader, err := r.FormFile("certificate")
	if err != nil || certHeader == nil {
		http.Error(w, "certificate required", http.StatusBadRequest)
		return
	}
	_, keyHeader, err := r.FormFile("certificate_key")
	if err != nil || keyHeader == nil {
		http.Error(w, "certificate key required", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"certificate":     map[string]any{},
		"certificate_key": true,
	})
}

func createACMECertificateFromNPM(item npmCertificate) (npmCertificate, error) {
	if len(item.DomainNames) == 0 {
		return npmCertificate{}, errors.New("域名不能为空")
	}
	if item.Meta == nil {
		item.Meta = map[string]any{}
	}
	provider, _ := item.Meta["dns_provider"].(string)
	if provider == "" {
		provider, _ = item.Meta["dnsProvider"].(string)
	}
	rawCreds, _ := item.Meta["dns_provider_credentials"].(string)
	if rawCreds == "" {
		rawCreds, _ = item.Meta["dnsProviderCredentials"].(string)
	}
	creds, err := loadCredentials()
	if err != nil {
		return npmCertificate{}, err
	}
	issuer, err := issuerFromNPM(item.Provider, item.Meta)
	if err != nil {
		return npmCertificate{}, err
	}
	rememberIssuerConfig(issuer)
	var cred Credential
	credID := ""
	if v, ok := item.Meta["credential_id"].(string); ok {
		credID = strings.TrimSpace(v)
	}
	if credID == "" {
		if v, ok := item.Meta["credentialId"].(string); ok {
			credID = strings.TrimSpace(v)
		}
	}
	if strings.TrimSpace(rawCreds) == "" && credID != "" {
		var ok bool
		cred, ok = findCredential(credID, creds)
		if !ok {
			return npmCertificate{}, errors.New("原 DNS 凭据不存在，请重新选择 DNS 提供商并填写凭据")
		}
	} else {
		var err error
		cred, err = credentialFromDNSProvider(provider, rawCreds)
		if err != nil {
			return npmCertificate{}, err
		}
		cred.ID = newCredID()
		cred.Name = "ACME " + strings.Join(item.DomainNames, ", ")
		cred.Issuer = issuer
		creds = append(creds, cred)
		if err := saveCredentials(creds); err != nil {
			return npmCertificate{}, err
		}
	}
	for _, domain := range item.DomainNames {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}
		baseName := managedCertPrefix + strings.TrimPrefix(domain, "*.")
		if site, ok, err := findTLSManagedSite(domain); err != nil {
			return npmCertificate{}, err
		} else if ok {
			site.Issuer = issuer
			site.ChallengePref = "dns"
			site.CredentialID = cred.ID
			applyCertificateRequestToSite(&site, domain, issuer, "dns", cred.ID)
			site.NoTLS = false
			site.LastError = ""
			site.LastErrorAt = time.Time{}
			if strings.HasPrefix(domain, "*.") {
				site.Wildcard = true
			}
			if err := saveRenderedSite(site, creds); err != nil {
				return npmCertificate{}, err
			}
			removeManagedCertPlaceholder(domain)
			continue
		}
		confPath := filepath.Join(sitesDir, baseName+confSuffix)
		metaPath := filepath.Join(sitesDir, baseName+metaSuffix)
		meta := placeholderMeta{Domain: domain, CredentialID: cred.ID, Issuer: issuer}
		stampPlaceholderForSave(&meta, metaPath)
		metaBytes, _ := json.MarshalIndent(meta, "", "  ")
		if err := os.WriteFile(confPath, []byte(renderPlaceholder(domain, cred, issuer)), 0644); err != nil {
			return npmCertificate{}, err
		}
		if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
			return npmCertificate{}, err
		}
	}
	if err := cleanupManagedCertConflicts(); err != nil {
		return npmCertificate{}, err
	}
	if err := reloadCaddy(); err != nil {
		return npmCertificate{}, fmt.Errorf("reload failed: %w", err)
	}
	return certOverviewToNPM(CertOverview{
		Domain:         item.DomainNames[0],
		IsWildcard:     strings.HasPrefix(item.DomainNames[0], "*."),
		Status:         "pending",
		Provider:       issuer.Provider,
		SignMethod:     "DNS-01",
		CredentialID:   cred.ID,
		CredentialName: cred.Name,
		IssuerConfig:   issuer,
	}), nil
}

func credentialFromDNSProvider(provider, raw string) (Credential, error) {
	values := parseCredentialLines(raw)
	switch provider {
	case "alidns":
		key := firstNonEmpty(values["ALICLOUD_ACCESS_KEY"], values["ALICLOUD_ACCESS_KEY_ID"], values["ALIYUN_ACCESS_KEY_ID"])
		secret := firstNonEmpty(values["ALICLOUD_SECRET_KEY"], values["ALICLOUD_ACCESS_KEY_SECRET"], values["ALIYUN_ACCESS_KEY_SECRET"])
		if key == "" || secret == "" {
			return Credential{}, errors.New("阿里云 DNS 需要 ALICLOUD_ACCESS_KEY 和 ALICLOUD_SECRET_KEY")
		}
		return Credential{Provider: "alidns", AliyunKey: key, AliyunSecret: secret}, nil
	case "cloudflare":
		token := firstNonEmpty(values["CLOUDFLARE_API_TOKEN"], values["CF_API_TOKEN"])
		if token == "" {
			return Credential{}, errors.New("Cloudflare 需要 CLOUDFLARE_API_TOKEN")
		}
		return Credential{Provider: "cloudflare", CFToken: token}, nil
	case "dnspod":
		token := firstNonEmpty(values["DNSPOD_TOKEN"], values["DNSPOD_API_TOKEN"], values["DNSPOD_LOGIN_TOKEN"])
		if token == "" {
			return Credential{}, errors.New("DNSPod 需要 DNSPOD_TOKEN，格式为 APP_ID,APP_TOKEN")
		}
		return Credential{Provider: "dnspod", DNSPodToken: token}, nil
	case "he":
		key := firstNonEmpty(values["HE_API_KEY"], values["HEDNS_API_KEY"], values["HURRICANE_ELECTRIC_API_KEY"])
		if key == "" {
			return Credential{}, errors.New("Hurricane Electric 需要 HE_API_KEY")
		}
		return Credential{Provider: "he", HEAPIKey: key}, nil
	default:
		return Credential{}, fmt.Errorf("不支持的 DNS 提供商：%s", provider)
	}
}

func parseCredentialLines(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		out[key] = value
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func npmCertificateItemHandler(w http.ResponseWriter, r *http.Request) {
	path := trimAnyPrefix(r.URL.Path, "/caddy/certificates/", "/nginx/certificates/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	cert, ok := npmCertificateByID(id)
	if !ok {
		http.Error(w, "certificate not found", http.StatusNotFound)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPost && parts[1] == "upload" {
		updated, err := uploadCustomCertificate(id, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodGet && parts[1] == "download" {
		downloadCertificateFiles(w, cert)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPost && parts[1] == "renew" {
		removed := 0
		hasActiveIssue := false
		for _, d := range cert.DomainNames {
			if certIssueLockExists(d) {
				hasActiveIssue = true
			}
			n, err := clearCertFiles(d)
			if err != nil {
				http.Error(w, "clear certificate files failed: "+err.Error(), http.StatusBadRequest)
				return
			}
			removed += n
			if err := clearCertError(d); err != nil {
				http.Error(w, "clear certificate error failed: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
		// If Caddy is already issuing this certificate, reloading cancels the in-flight
		// ACME job and turns a retry into "context canceled". Clear the visible error
		// and let the existing automation continue.
		if removed > 0 || !hasActiveIssue {
			if err := reloadCaddy(); err != nil {
				http.Error(w, "reload failed: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
		if updated, ok := npmCertificateByID(id); ok {
			writeJSON(w, http.StatusOK, updated)
		} else {
			writeJSON(w, http.StatusOK, cert)
		}
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, cert)
	case http.MethodDelete:
		for _, d := range cert.DomainNames {
			if linked, err := domainLinkedBy(d); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else if len(linked) > 0 {
				http.Error(w, fmt.Sprintf("仍被反代引用：%s", strings.Join(linked, ", ")), http.StatusBadRequest)
				return
			}
		}
		for _, d := range cert.DomainNames {
			_, _ = clearCertFiles(d)
			deleteManagedCertPlaceholder(d)
			_ = deleteACMECredentialForDomain(d, certCredentialID(cert))
		}
		writeJSON(w, http.StatusOK, true)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func uploadCustomCertificate(id int, r *http.Request) (npmCertificate, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return npmCertificate{}, err
	}
	records, err := loadCustomCertRecords()
	if err != nil {
		return npmCertificate{}, err
	}
	idx := -1
	for i := range records {
		if records[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return npmCertificate{}, errors.New("custom certificate not found")
	}
	dir := filepath.Join(customCertsDir, strconv.Itoa(id))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return npmCertificate{}, err
	}
	certPath := filepath.Join(dir, "certificate.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := saveUploadedFile(r, "certificate", certPath); err != nil {
		return npmCertificate{}, err
	}
	if err := saveUploadedFile(r, "certificate_key", keyPath); err != nil {
		return npmCertificate{}, err
	}
	if _, header, err := r.FormFile("intermediate_certificate"); err == nil && header != nil {
		_ = saveUploadedFile(r, "intermediate_certificate", filepath.Join(dir, "intermediate.pem"))
	}
	if names, err := certificateDomainNamesFromFile(certPath); err == nil && len(names) > 0 {
		records[idx].DomainNames = names
	}
	records[idx].CertFile = certPath
	records[idx].KeyFile = keyPath
	if err := saveJSONFile(customCertMeta, records); err != nil {
		return npmCertificate{}, err
	}
	return customCertToNPM(records[idx]), nil
}

func certificateDomainNamesFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := []string{}
	seen := map[string]bool{}
	for {
		block, rest := pem.Decode(data)
		if block == nil {
			break
		}
		data = rest
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		for _, name := range append(cert.DNSNames, cert.Subject.CommonName) {
			name = certDomainName(name)
			if name == "" {
				continue
			}
			key := strings.ToLower(name)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, name)
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return out, nil
}

func saveUploadedFile(r *http.Request, field string, target string) error {
	file, _, err := r.FormFile(field)
	if err != nil {
		return err
	}
	defer file.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	return err
}

func downloadCertificateFiles(w http.ResponseWriter, cert npmCertificate) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="certificate-%d.pem"`, cert.ID))
	for _, rec := range mustLoadCustomCertRecords() {
		if rec.ID == cert.ID {
			if rec.CertFile != "" {
				data, _ := os.ReadFile(rec.CertFile)
				_, _ = w.Write(data)
			}
			if rec.KeyFile != "" {
				data, _ := os.ReadFile(rec.KeyFile)
				_, _ = w.Write([]byte("\n"))
				_, _ = w.Write(data)
			}
			return
		}
	}
	_, _ = w.Write([]byte("# Managed by Caddy UI\n"))
	for _, d := range cert.DomainNames {
		_, _ = w.Write([]byte("# " + d + "\n"))
	}
}

func readAllSites() ([]Site, error) {
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Site{}, nil
		}
		return nil, err
	}
	sites := []Site{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if strings.HasPrefix(baseName, managedCertPrefix) {
			continue
		}
		metaPath := filepath.Join(sitesDir, e.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var s Site
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		if s.Name == "" {
			s.Name = baseName
		}
		s.CreatedOn, s.ModifiedOn = normalizePersistentTimestamps(s.CreatedOn, s.ModifiedOn, metaPath)
		if s.ChallengePref == "" {
			s.ChallengePref = "http"
		}
		sites = append(sites, s)
	}
	sort.Slice(sites, func(i, j int) bool { return sites[i].Name < sites[j].Name })
	return sites, nil
}

func siteByNPMID(id int) (Site, bool, error) {
	sites, err := readAllSites()
	if err != nil {
		return Site{}, false, err
	}
	for _, s := range sites {
		if stableID(s.Name) == id {
			return s, true, nil
		}
	}
	return Site{}, false, nil
}

func readSitesByKind(kind string) ([]Site, error) {
	sites, err := readAllSites()
	if err != nil {
		return nil, err
	}
	out := []Site{}
	for _, s := range sites {
		if kind == "proxy" && s.Kind == "" {
			out = append(out, s)
			continue
		}
		if s.Kind == kind {
			out = append(out, s)
		}
	}
	return out, nil
}

func siteByNPMIDAndKind(id int, kind string) (Site, bool, error) {
	sites, err := readSitesByKind(kind)
	if err != nil {
		return Site{}, false, err
	}
	for _, s := range sites {
		if stableID(s.Name) == id {
			return s, true, nil
		}
	}
	return Site{}, false, nil
}

func handleSiteBackedItem(w http.ResponseWriter, r *http.Request, prefix string, kind string, toNPM func(Site) any, fromNPM func(*http.Request, string) (Site, error)) {
	path := prefix
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	site, ok, err := siteByNPMIDAndKind(id, kind)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPost {
		switch parts[1] {
		case "enable", "disable":
			next := site
			next.Disabled = parts[1] == "disable"
			if err := saveSiteFromNPM(next); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			appendAuditLog(kind+"-host", stableID(next.Name), parts[1]+"d", siteAuditMeta(next))
			writeJSON(w, http.StatusOK, true)
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, toNPM(site))
	case http.MethodPut:
		next, err := fromNPM(r, site.Name)
		if err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		next.CreatedOn = site.CreatedOn
		next.Disabled = site.Disabled
		if err := saveSiteFromNPM(next); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		appendAuditLog(kind+"-host", stableID(next.Name), "updated", siteAuditMeta(next))
		writeJSON(w, http.StatusOK, toNPM(next))
	case http.MethodDelete:
		deleteSite(w, site.Name)
		appendAuditLog(kind+"-host", stableID(site.Name), "deleted", siteAuditMeta(site))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func saveSiteFromNPM(s Site) error {
	body, _ := json.Marshal(s)
	req, _ := http.NewRequest(http.MethodPut, "/sites/"+s.Name, bytes.NewReader(body))
	rr := &captureResponse{header: http.Header{}}
	putSite(rr, req, s.Name)
	if rr.status >= 400 {
		return errors.New(strings.TrimSpace(rr.body.String()))
	}
	return nil
}

type captureResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (r *captureResponse) Header() http.Header { return r.header }
func (r *captureResponse) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}
func (r *captureResponse) WriteHeader(statusCode int) { r.status = statusCode }

func siteToNPMProxyHost(s Site, certByDomain map[string]npmCertificate) npmProxyHost {
	scheme, host, port := splitBackend(s.Backend)
	defaultListenPort := 443
	if s.NoTLS {
		defaultListenPort = 80
	}
	domains, listenPort, listenPorts := splitNPMListenDomains(s.Domain, defaultListenPort)
	bindings := canonicalizeWildcardCertificateBindings(s, domains, s.CertificateBindings, certByDomain)
	certID := 0
	var cert *npmCertificate
	if !s.NoTLS {
		if s.CertificateMode == "auto" {
			if len(bindings) == 0 {
				bindings = inferredCertificateBindings(domains, certByDomain)
			}
			bindings = enrichCertificateBindings(bindings, certByDomain)
			if len(domains) == 1 {
				if binding, ok := certificateBindingForDomain(bindings, domains[0]); ok && binding.Mode == "selected" {
					if selected, ok := certByIDFromDomainMap(certByDomain, binding.CertificateID); ok {
						cert = &selected
					}
				}
			}
			certID = -1
		} else if s.Wildcard {
			if c, ok := wildcardCertificateCoveringAllDomains(domains, certByDomain); ok {
				certID = c.ID
				cert = &c
			} else if c, ok := certCoveringAllDomains(domains, certByDomain); ok {
				certID = c.ID
				cert = &c
			}
		} else if c, ok := certCoveringAllDomains(domains, certByDomain); ok {
			certID = c.ID
			cert = &c
		} else {
			if len(bindings) == 0 {
				bindings = inferredCertificateBindings(domains, certByDomain)
			}
			certID = -1
		}
	}
	var accessList *npmAccessList
	if s.AccessListID > 0 {
		if item, ok := accessListByID(s.AccessListID); ok {
			accessList = &item
		}
	}
	forwardAuth := s.ForwardAuth
	if !forwardAuth.Enabled {
		forwardAuth = ForwardAuthConfig{}
	}
	lastError, lastErrorAt := proxyHostVisibleCertError(s)
	var locations []npmProxyLocation
	if len(s.Locations) > 0 {
		locations = make([]npmProxyLocation, 0, len(s.Locations))
		for _, loc := range s.Locations {
			locations = append(locations, npmProxyLocation{
				Path:          loc.Path,
				ForwardScheme: loc.ForwardScheme,
				ForwardHost:   loc.ForwardHost,
				ForwardPort:   loc.ForwardPort,
				ForwardPath:   loc.ForwardPath,
			})
		}
	}
	return npmProxyHost{
		ID:                         stableID(s.Name),
		CreatedOn:                  defaultString(s.CreatedOn, seedTimestamp()),
		ModifiedOn:                 defaultString(s.ModifiedOn, defaultString(s.CreatedOn, seedTimestamp())),
		OwnerUserID:                1,
		ServiceName:                s.ServiceName,
		DomainNames:                domains,
		ListenPort:                 listenPort,
		ListenPorts:                listenPorts,
		ForwardScheme:              scheme,
		ForwardHost:                host,
		ForwardPort:                port,
		Locations:                  locations,
		AccessListID:               s.AccessListID,
		CertificateID:              certID,
		SSLForced:                  !s.NoTLS,
		CachingEnabled:             false,
		BlockExploits:              false,
		AdvancedConfig:             "",
		Meta:                       mergeMeta(map[string]any{"caddy_name": s.Name, "challenge_pref": s.ChallengePref, "wildcard": s.Wildcard, "credential_id": s.CredentialID, "certificate_mode": s.CertificateMode, "certificate_bindings": bindings, "last_error": lastError, "last_error_at": lastErrorAt}, issuerMeta(s.Issuer)),
		AllowWebsocketUpgrade:      true,
		HTTP2Support:               true,
		Enabled:                    !s.Disabled,
		HSTSEnabled:                false,
		HSTSSubdomains:             false,
		TrustForwardedProto:        false,
		ForwardAuth:                forwardAuth,
		UpstreamInsecureSkipVerify: s.UpstreamInsecureSkipVerify,
		Owner:                      localNPMUser,
		Certificate:                cert,
		AccessList:                 accessList,
	}
}

func npmProxyHostToSite(p npmProxyHost, existingName string) (Site, error) {
	backend := fmt.Sprintf("%s://%s:%d", defaultString(p.ForwardScheme, "http"), strings.TrimSpace(p.ForwardHost), p.ForwardPort)
	challenge := "http"
	wildcard := false
	credentialID := ""
	customCertFile := ""
	customKeyFile := ""
	bindings := certificateBindingsFromMeta(p.Meta)
	if p.Meta != nil {
		if v, ok := p.Meta["challenge_pref"].(string); ok && v != "" {
			challenge = v
		}
		if v, ok := p.Meta["wildcard"].(bool); ok {
			wildcard = v
		}
		if v, ok := p.Meta["credential_id"].(string); ok {
			credentialID = v
		}
	}
	certificateMode := ""
	if npmNewCertificateRequested(p.CertificateID) {
		if dnsChallenge, provider, rawCreds := dnsCertificateMeta(p.Meta); dnsChallenge {
			cred, err := ensureACMECredential(p.DomainNames, provider, rawCreds)
			if err != nil {
				return Site{}, err
			}
			challenge = "dns"
			credentialID = cred.ID
		} else {
			challenge = "http"
			credentialID = ""
		}
	} else if asInt(p.CertificateID) == -1 {
		challenge = "http"
		credentialID = ""
		certificateMode = "auto"
		if len(bindings) == 0 {
			bindings = certificateBindingsFromDomains(p.DomainNames)
		}
		bindings = hydrateAutoCertificateBindings(bindings)
	}
	issuer, err := issuerFromNPM("", p.Meta)
	if err != nil {
		return Site{}, err
	}
	forwardAuth, err := normalizeProxyHostForwardAuthConfig(p.ForwardAuth)
	if err != nil {
		return Site{}, err
	}
	var locations []ProxyLocation
	if len(p.Locations) > 0 {
		locations = make([]ProxyLocation, 0, len(p.Locations))
		for _, loc := range p.Locations {
			scheme := defaultString(loc.ForwardScheme, "http")
			hostRaw := strings.TrimSpace(loc.ForwardHost)
			port := loc.ForwardPort
			if port <= 0 {
				port = 80
			}
			// 分离 host 和 path（兼容用户在 host 字段中输入了 path 的情况）
			host, hostPath := splitHostAndPath(hostRaw)
			// ForwardPath 优先；其次用 host 字段中提取的 path
			upstreamPath := strings.TrimSpace(loc.ForwardPath)
			if upstreamPath == "" && hostPath != "" {
				upstreamPath = hostPath
			}
			locBackend := fmt.Sprintf("%s://%s:%d", scheme, host, port)
			if upstreamPath != "" {
				locBackend += upstreamPath
			}
			locations = append(locations, ProxyLocation{
				Path:          strings.TrimSpace(loc.Path),
				ForwardScheme: scheme,
				ForwardHost:   host,
				ForwardPort:   port,
				ForwardPath:   upstreamPath,
				Backend:       locBackend,
			})
		}
	}
	site := Site{
		CreatedOn:                  strings.TrimSpace(p.CreatedOn),
		ModifiedOn:                 strings.TrimSpace(p.ModifiedOn),
		Name:                       defaultString(existingName, uniqueSiteName(firstDomain(p.DomainNames))),
		ServiceName:                strings.TrimSpace(p.ServiceName),
		Domain:                     joinListenDomainsWithPorts(p.DomainNames, p.ListenPort, p.ListenPorts),
		Backend:                    backend,
		Locations:                  locations,
		AccessListID:               p.AccessListID,
		Wildcard:                   wildcard,
		NoTLS:                      !p.SSLForced,
		ChallengePref:              challenge,
		CredentialID:               credentialID,
		CertificateMode:            certificateMode,
		CertificateBindings:        bindings,
		Issuer:                     issuer,
		CustomCertFile:             customCertFile,
		CustomKeyFile:              customKeyFile,
		ForwardAuth:                forwardAuth,
		UpstreamInsecureSkipVerify: p.UpstreamInsecureSkipVerify,
		Disabled:                   !p.Enabled,
	}
	if !npmNewCertificateRequested(p.CertificateID) && asInt(p.CertificateID) > 0 {
		if !site.Wildcard || !applyWildcardCertificateConfigToSite(&site) {
			applyCertificateConfigToSite(&site, p.CertificateID)
		}
	}
	if site.Wildcard {
		applyWildcardCertificateConfigToSite(&site)
		site.CertificateBindings = canonicalizeWildcardCertificateBindings(site, p.DomainNames, site.CertificateBindings, npmCertificatesByDomain())
	}
	return site, nil
}

func hydrateAutoCertificateBindings(bindings []CertificateBinding) []CertificateBinding {
	if len(bindings) == 0 {
		return bindings
	}
	out := make([]CertificateBinding, len(bindings))
	copy(out, bindings)
	for i := range out {
		if out[i].Mode != "auto" {
			continue
		}
		cfg, ok := certificateConfigForDomain(out[i].Domain)
		if !ok || cfg.Issuer.Provider == "" || cfg.Issuer.Provider == "auto" {
			continue
		}
		out[i].Mode = "selected"
		out[i].CertificateID = stableID("cert:" + certDomainName(cfg.Domain))
		out[i].ChallengePref = cfg.ChallengePref
		out[i].CredentialID = cfg.CredentialID
		out[i].Issuer = cfg.Issuer
		out[i].Provider = cfg.Issuer.Provider
		out[i].NiceName = certDomainName(cfg.Domain)
	}
	return out
}

func siteToNPMHostCertificateState(s Site, domains []string, certByDomain map[string]npmCertificate) (any, *npmCertificate, []CertificateBinding) {
	bindings := canonicalizeWildcardCertificateBindings(s, domains, s.CertificateBindings, certByDomain)
	certID := any(0)
	var cert *npmCertificate
	if s.NoTLS {
		return certID, cert, bindings
	}
	if s.CertificateMode == "auto" {
		if len(bindings) == 0 {
			bindings = inferredCertificateBindings(domains, certByDomain)
		}
		bindings = enrichCertificateBindings(bindings, certByDomain)
		if len(domains) == 1 {
			if binding, ok := certificateBindingForDomain(bindings, domains[0]); ok && binding.Mode == "selected" {
				if selected, ok := certByIDFromDomainMap(certByDomain, binding.CertificateID); ok {
					cert = &selected
				}
			}
		}
		certID = -1
	} else if s.Wildcard {
		if c, ok := wildcardCertificateCoveringAllDomains(domains, certByDomain); ok {
			certID = c.ID
			cert = &c
		} else if c, ok := certCoveringAllDomains(domains, certByDomain); ok {
			certID = c.ID
			cert = &c
		}
	} else if c, ok := certCoveringAllDomains(domains, certByDomain); ok {
		certID = c.ID
		cert = &c
	} else {
		if len(bindings) == 0 {
			bindings = inferredCertificateBindings(domains, certByDomain)
		}
		certID = -1
	}
	return certID, cert, bindings
}

func npmHostCertificateStateToSite(certificateID any, domainNames []string, meta map[string]any) (string, string, string, []CertificateBinding, error) {
	challenge, credentialID := metaChallenge(meta)
	bindings := certificateBindingsFromMeta(meta)
	certificateMode := ""
	if npmNewCertificateRequested(certificateID) {
		if dnsChallenge, provider, rawCreds := dnsCertificateMeta(meta); dnsChallenge {
			cred, err := ensureACMECredential(domainNames, provider, rawCreds)
			if err != nil {
				return "", "", "", nil, err
			}
			challenge = "dns"
			credentialID = cred.ID
		} else {
			challenge = "http"
			credentialID = ""
		}
	} else if asInt(certificateID) == -1 {
		challenge = "http"
		credentialID = ""
		certificateMode = "auto"
		if len(bindings) == 0 {
			bindings = certificateBindingsFromDomains(domainNames)
		}
		bindings = hydrateAutoCertificateBindings(bindings)
	} else {
		bindings = nil
	}
	return challenge, credentialID, certificateMode, bindings, nil
}

func siteToNPMRedirectionHost(s Site, certByDomain map[string]npmCertificate) npmRedirectionHost {
	domains := splitDomains(s.Domain)
	certID, cert, bindings := siteToNPMHostCertificateState(s, domains, certByDomain)
	scheme := "http"
	target := s.RedirectURL
	if strings.Contains(target, "://") {
		parts := strings.SplitN(target, "://", 2)
		scheme = parts[0]
		target = parts[1]
	}
	return npmRedirectionHost{
		ID:                stableID(s.Name),
		CreatedOn:         defaultString(s.CreatedOn, seedTimestamp()),
		ModifiedOn:        defaultString(s.ModifiedOn, defaultString(s.CreatedOn, seedTimestamp())),
		OwnerUserID:       1,
		DomainNames:       domains,
		ForwardDomainName: target,
		PreservePath:      s.PreservePath,
		CertificateID:     certID,
		SSLForced:         !s.NoTLS,
		BlockExploits:     false,
		AdvancedConfig:    "",
		Meta:              mergeMeta(map[string]any{"caddy_name": s.Name, "challenge_pref": s.ChallengePref, "wildcard": s.Wildcard, "credential_id": s.CredentialID, "certificate_mode": s.CertificateMode, "certificate_bindings": bindings}, issuerMeta(s.Issuer)),
		HTTP2Support:      true,
		ForwardScheme:     scheme,
		ForwardHTTPCode:   defaultInt(s.RedirectCode, http.StatusMovedPermanently),
		Enabled:           !s.Disabled,
		HSTSEnabled:       false,
		HSTSSubdomains:    false,
		Owner:             localNPMUser,
		Certificate:       cert,
	}
}

func npmRedirectionHostToSite(item npmRedirectionHost, existingName string) (Site, error) {
	challenge, credentialID, certificateMode, bindings, err := npmHostCertificateStateToSite(item.CertificateID, item.DomainNames, item.Meta)
	if err != nil {
		return Site{}, err
	}
	issuer, _ := issuerFromNPM("", item.Meta)
	wildcard := metaBool(item.Meta, "wildcard", "Wildcard")
	target := strings.TrimSpace(item.ForwardDomainName)
	if target != "" && !strings.Contains(target, "://") {
		target = defaultString(item.ForwardScheme, "https") + "://" + target
	}
	site := Site{
		CreatedOn:           strings.TrimSpace(item.CreatedOn),
		ModifiedOn:          strings.TrimSpace(item.ModifiedOn),
		Name:                defaultString(existingName, "redir-"+uniqueSiteName(firstDomain(item.DomainNames))),
		Kind:                "redirection",
		Domain:              strings.Join(item.DomainNames, ", "),
		RedirectURL:         target,
		RedirectCode:        defaultInt(item.ForwardHTTPCode, http.StatusMovedPermanently),
		PreservePath:        item.PreservePath,
		NoTLS:               !item.SSLForced,
		Wildcard:            wildcard,
		ChallengePref:       challenge,
		CredentialID:        credentialID,
		CertificateMode:     certificateMode,
		CertificateBindings: bindings,
		Issuer:              issuer,
		Disabled:            !item.Enabled,
	}
	if site.Wildcard {
		applyWildcardCertificateConfigToSite(&site)
		site.CertificateBindings = canonicalizeWildcardCertificateBindings(site, item.DomainNames, site.CertificateBindings, npmCertificatesByDomain())
	} else {
		applyCertificateConfigToSite(&site, item.CertificateID)
	}
	return site, nil
}

func siteToNPMDeadHost(s Site, certByDomain map[string]npmCertificate) npmDeadHost {
	domains := splitDomains(s.Domain)
	certID, cert, bindings := siteToNPMHostCertificateState(s, domains, certByDomain)
	return npmDeadHost{
		ID:             stableID(s.Name),
		CreatedOn:      defaultString(s.CreatedOn, seedTimestamp()),
		ModifiedOn:     defaultString(s.ModifiedOn, defaultString(s.CreatedOn, seedTimestamp())),
		OwnerUserID:    1,
		DomainNames:    domains,
		CertificateID:  certID,
		SSLForced:      !s.NoTLS,
		AdvancedConfig: "",
		Meta:           mergeMeta(map[string]any{"caddy_name": s.Name, "challenge_pref": s.ChallengePref, "wildcard": s.Wildcard, "credential_id": s.CredentialID, "certificate_mode": s.CertificateMode, "certificate_bindings": bindings}, issuerMeta(s.Issuer)),
		HTTP2Support:   true,
		Enabled:        !s.Disabled,
		HSTSEnabled:    false,
		HSTSSubdomains: false,
		Owner:          localNPMUser,
		Certificate:    cert,
	}
}

func npmDeadHostToSite(item npmDeadHost, existingName string) (Site, error) {
	challenge, credentialID, certificateMode, bindings, err := npmHostCertificateStateToSite(item.CertificateID, item.DomainNames, item.Meta)
	if err != nil {
		return Site{}, err
	}
	issuer, _ := issuerFromNPM("", item.Meta)
	wildcard := metaBool(item.Meta, "wildcard", "Wildcard")
	site := Site{
		CreatedOn:           strings.TrimSpace(item.CreatedOn),
		ModifiedOn:          strings.TrimSpace(item.ModifiedOn),
		Name:                defaultString(existingName, "dead-"+uniqueSiteName(firstDomain(item.DomainNames))),
		Kind:                "dead",
		Domain:              strings.Join(item.DomainNames, ", "),
		NoTLS:               !item.SSLForced,
		Wildcard:            wildcard,
		ChallengePref:       challenge,
		CredentialID:        credentialID,
		CertificateMode:     certificateMode,
		CertificateBindings: bindings,
		Issuer:              issuer,
		Disabled:            !item.Enabled,
	}
	if site.Wildcard {
		applyWildcardCertificateConfigToSite(&site)
		site.CertificateBindings = canonicalizeWildcardCertificateBindings(site, item.DomainNames, site.CertificateBindings, npmCertificatesByDomain())
	} else {
		applyCertificateConfigToSite(&site, item.CertificateID)
	}
	return site, nil
}

func certForSite(s Site, certByDomain map[string]npmCertificate) (int, *npmCertificate) {
	if s.CustomCertFile != "" || s.CustomKeyFile != "" {
		for _, rec := range mustLoadCustomCertRecords() {
			if rec.CertFile == s.CustomCertFile && rec.KeyFile == s.CustomKeyFile {
				cert := customCertToNPM(rec)
				return cert.ID, &cert
			}
		}
	}
	for _, d := range splitDomains(s.Domain) {
		if c, ok := certByDomain[certDomainKey(d)]; ok {
			return c.ID, &c
		}
	}
	return 0, nil
}

func applyCertificateConfigToSite(s *Site, certificateID any) bool {
	cfg, ok := certificateConfigForID(asInt(certificateID))
	if !ok {
		s.CustomCertFile = ""
		s.CustomKeyFile = ""
		return false
	}
	s.ChallengePref = cfg.ChallengePref
	s.CredentialID = cfg.CredentialID
	s.Issuer = cfg.Issuer
	s.Wildcard = strings.HasPrefix(certDomainName(cfg.Domain), "*.")
	s.CustomCertFile = cfg.CustomCertFile
	s.CustomKeyFile = cfg.CustomKeyFile
	if cfg.CustomCertFile != "" || cfg.CustomKeyFile != "" {
		s.CredentialID = ""
		s.Issuer = IssuerConfig{}
		s.Wildcard = false
	}
	return true
}

func applyWildcardCertificateConfigToSite(s *Site) bool {
	creds, _ := loadCredentials()
	for _, domain := range splitDomains(s.Domain) {
		wildcardDomain, ok := wildcardForDomain(certDomainName(domain))
		if !ok {
			continue
		}
		if cfg, ok := certificateConfigForDomain(wildcardDomain); ok && strings.HasPrefix(certDomainName(cfg.Domain), "*.") {
			if cfg.CredentialID == "" {
				if requestCfg, ok := certificateRequestConfigForDomain(wildcardDomain, creds); ok {
					cfg.CredentialID = requestCfg.CredentialID
					if cfg.Issuer.Provider == "" || cfg.Issuer.Provider == "auto" {
						cfg.Issuer = requestCfg.Issuer
					}
					if requestCfg.SignMethod == "DNS-01" {
						cfg.ChallengePref = "dns"
					}
				}
			}
			s.ChallengePref = cfg.ChallengePref
			s.CredentialID = cfg.CredentialID
			s.Issuer = cfg.Issuer
			s.Wildcard = true
			s.CustomCertFile = ""
			s.CustomKeyFile = ""
			return true
		}
		if s.CredentialID != "" || s.ChallengePref == "dns" || s.Issuer.Provider != "" {
			if s.ChallengePref == "" || s.ChallengePref == "http" {
				s.ChallengePref = "dns"
			}
			s.Wildcard = true
			s.CustomCertFile = ""
			s.CustomKeyFile = ""
			return true
		}
	}
	return false
}

func applyCertificateRequestToSite(s *Site, domain string, issuer IssuerConfig, challengePref string, credentialID string) {
	domain = certDomainName(domain)
	if domain == "" {
		return
	}
	if challengePref == "" {
		challengePref = "http"
	}
	if len(s.CertificateBindings) == 0 {
		return
	}
	next := make([]CertificateBinding, len(s.CertificateBindings))
	copy(next, s.CertificateBindings)
	for i := range next {
		if certDomainKey(next[i].Domain) != certDomainKey(domain) {
			continue
		}
		next[i].Mode = "selected"
		next[i].CertificateID = stableID("cert:" + domain)
		next[i].CertificateDomain = domain
		next[i].ChallengePref = challengePref
		next[i].CredentialID = credentialID
		next[i].Issuer = issuer
		next[i].Provider = defaultString(issuer.Provider, "auto")
		next[i].NiceName = domain
		next[i].LastError = ""
		next[i].LastErrorAt = time.Time{}
	}
	s.CertificateBindings = next
	s.CertificateMode = "auto"
}

func metaChallenge(meta map[string]any) (string, string) {
	challenge := "http"
	credentialID := ""
	if meta != nil {
		if v, ok := meta["challenge_pref"].(string); ok && v != "" {
			challenge = v
		}
		if v, ok := meta["credential_id"].(string); ok {
			credentialID = v
		}
	}
	return challenge, credentialID
}

func npmCertificateSelected(v any) bool {
	switch x := v.(type) {
	case string:
		return x != "" && x != "0"
	case float64:
		return x > 0
	case int:
		return x > 0
	default:
		return false
	}
}

func npmNewCertificateRequested(v any) bool {
	s, ok := v.(string)
	return ok && s == "new"
}

func dnsCertificateMeta(meta map[string]any) (bool, string, string) {
	if meta == nil {
		return false, "", ""
	}
	dnsChallenge, _ := meta["dns_challenge"].(bool)
	if !dnsChallenge {
		dnsChallenge, _ = meta["dnsChallenge"].(bool)
	}
	provider, _ := meta["dns_provider"].(string)
	if provider == "" {
		provider, _ = meta["dnsProvider"].(string)
	}
	rawCreds, _ := meta["dns_provider_credentials"].(string)
	if rawCreds == "" {
		rawCreds, _ = meta["dnsProviderCredentials"].(string)
	}
	return dnsChallenge, provider, rawCreds
}

func ensureACMECredential(domains []string, provider, rawCreds string) (Credential, error) {
	cred, err := credentialFromDNSProvider(provider, rawCreds)
	if err != nil {
		return Credential{}, err
	}
	creds, err := loadCredentials()
	if err != nil {
		return Credential{}, err
	}
	cred.ID = newCredID()
	cred.Name = "ACME " + strings.Join(certDomainNames(domains), ", ")
	creds = append(creds, cred)
	if err := saveCredentials(creds); err != nil {
		return Credential{}, err
	}
	return cred, nil
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	default:
		return 0
	}
}

func npmCertificates() []npmCertificate {
	certs := []npmCertificate{}
	for _, c := range certOverviewRows() {
		certs = append(certs, certOverviewToNPM(c))
	}
	for _, rec := range mustLoadCustomCertRecords() {
		certs = append(certs, customCertToNPM(rec))
	}
	sort.Slice(certs, func(i, j int) bool { return certs[i].NiceName < certs[j].NiceName })
	populateCertificateUsage(certs)
	return certs
}

func populateCertificateUsage(certs []npmCertificate) {
	if len(certs) == 0 {
		return
	}
	byID := map[int]*npmCertificate{}
	for i := range certs {
		certs[i].ProxyHosts = []npmProxyHost{}
		certs[i].RedirectionHosts = []npmRedirectionHost{}
		certs[i].DeadHosts = []npmDeadHost{}
		certs[i].Streams = []npmStream{}
		byID[certs[i].ID] = &certs[i]
	}
	certByDomain := map[string]npmCertificate{}
	for _, c := range certs {
		for _, d := range c.DomainNames {
			certByDomain[certDomainKey(d)] = c
		}
	}
	if sites, err := readSitesByKind("proxy"); err == nil {
		for _, s := range sites {
			host := siteToNPMProxyHost(s, certByDomain)
			for _, id := range certificateUsageIDs(host.CertificateID, host.Certificate, host.Meta) {
				cert, ok := byID[id]
				if !ok {
					continue
				}
				hostForCert := host
				hostForCert.Certificate = nil
				cert.ProxyHosts = append(cert.ProxyHosts, hostForCert)
			}
		}
	}
	if sites, err := readSitesByKind("redirection"); err == nil {
		for _, s := range sites {
			host := siteToNPMRedirectionHost(s, certByDomain)
			for _, id := range certificateUsageIDs(host.CertificateID, host.Certificate, host.Meta) {
				cert, ok := byID[id]
				if !ok {
					continue
				}
				hostForCert := host
				hostForCert.Certificate = nil
				cert.RedirectionHosts = append(cert.RedirectionHosts, hostForCert)
			}
		}
	}
	if sites, err := readSitesByKind("dead"); err == nil {
		for _, s := range sites {
			host := siteToNPMDeadHost(s, certByDomain)
			for _, id := range certificateUsageIDs(host.CertificateID, host.Certificate, host.Meta) {
				cert, ok := byID[id]
				if !ok {
					continue
				}
				hostForCert := host
				hostForCert.Certificate = nil
				cert.DeadHosts = append(cert.DeadHosts, hostForCert)
			}
		}
	}
}

func certificateUsageIDs(certificateID any, certificate *npmCertificate, meta map[string]any) []int {
	out := []int{}
	seen := map[int]bool{}
	add := func(id int) {
		if id <= 0 || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}

	add(asInt(certificateID))
	if certificate != nil {
		add(certificate.ID)
	}
	for _, binding := range certificateBindingsFromMeta(meta) {
		if binding.Mode == "selected" {
			add(binding.CertificateID)
			continue
		}
		if binding.Mode == "auto" {
			add(stableID("cert:" + certDomainName(binding.Domain)))
		}
	}
	return out
}

func certCoversHost(certDomain string, host string) bool {
	certDomain = strings.ToLower(strings.TrimSpace(certDomain))
	host = strings.ToLower(strings.TrimSpace(certDomainName(host)))
	if certDomain == "" || host == "" {
		return false
	}
	if strings.HasPrefix(certDomain, "*.") {
		parent := strings.TrimPrefix(certDomain, "*.")
		return strings.HasSuffix(host, "."+parent) && strings.Count(host, ".") == strings.Count(parent, ".")+1
	}
	return certDomain == host
}

func npmCertCoversDomain(cert npmCertificate, domain string) bool {
	for _, certDomain := range cert.DomainNames {
		if certCoversHost(certDomain, domain) {
			return true
		}
	}
	return false
}

func certCoveringAllDomains(domains []string, certByDomain map[string]npmCertificate) (npmCertificate, bool) {
	seen := map[int]bool{}
	for _, domain := range domains {
		for _, candidateDomain := range []string{domain, wildcardParentDomain(domain)} {
			cert, ok := certByDomain[certDomainKey(candidateDomain)]
			if !ok || seen[cert.ID] {
				continue
			}
			seen[cert.ID] = true
			coversAll := true
			for _, requiredDomain := range domains {
				if !npmCertCoversDomain(cert, requiredDomain) {
					coversAll = false
					break
				}
			}
			if coversAll {
				return cert, true
			}
		}
	}
	return npmCertificate{}, false
}

func wildcardCertificateCoveringAllDomains(domains []string, certByDomain map[string]npmCertificate) (npmCertificate, bool) {
	var selected npmCertificate
	for _, domain := range domains {
		wildcardDomain, ok := wildcardForDomain(certDomainName(domain))
		if !ok {
			return npmCertificate{}, false
		}
		cert, ok := certByDomain[certDomainKey(wildcardDomain)]
		if !ok || !npmCertCoversDomain(cert, domain) {
			return npmCertificate{}, false
		}
		if selected.ID == 0 {
			selected = cert
			continue
		}
		if selected.ID != cert.ID {
			return npmCertificate{}, false
		}
	}
	if selected.ID == 0 {
		return npmCertificate{}, false
	}
	return selected, true
}

func inferredCertificateBindings(domains []string, certByDomain map[string]npmCertificate) []CertificateBinding {
	if len(domains) <= 1 {
		return nil
	}
	out := make([]CertificateBinding, 0, len(domains))
	for _, domain := range domains {
		binding := CertificateBinding{Domain: certDomainName(domain), Mode: "auto"}
		if cert, ok := bestCertificateForDomain(domain, certByDomain); ok {
			binding.Mode = "selected"
			binding.CertificateID = cert.ID
			binding.CertificateDomain = firstDomain(cert.DomainNames)
		}
		out = append(out, binding)
	}
	return out
}

func canonicalizeWildcardCertificateBindings(s Site, domains []string, bindings []CertificateBinding, certByDomain map[string]npmCertificate) []CertificateBinding {
	if !s.Wildcard {
		return bindings
	}
	if len(domains) == 0 {
		domains = splitDomains(s.Domain)
	}
	existing := map[string]CertificateBinding{}
	for _, binding := range bindings {
		existing[certDomainKey(binding.Domain)] = binding
	}
	out := make([]CertificateBinding, 0, len(domains))
	for _, domain := range domains {
		domain = certDomainName(domain)
		if domain == "" {
			continue
		}
		binding := existing[certDomainKey(domain)]
		wildcardDomain, ok := wildcardForDomain(domain)
		if !ok {
			if binding.Domain == "" {
				binding = CertificateBinding{Domain: domain, Mode: "auto"}
			}
			out = append(out, binding)
			continue
		}
		cfg := domainCertificateConfig{
			Domain:        wildcardDomain,
			ChallengePref: defaultString(firstNonEmpty(s.ChallengePref, binding.ChallengePref), "dns"),
			CredentialID:  firstNonEmpty(s.CredentialID, binding.CredentialID),
			Issuer:        s.Issuer,
		}
		if cfg.Issuer.Provider == "" && binding.Issuer.Provider != "" {
			cfg.Issuer = binding.Issuer
		} else if cfg.Issuer.Provider == "" && binding.Provider != "" {
			cfg.Issuer.Provider = binding.Provider
		}
		cert, hasCert := certByDomain[certDomainKey(wildcardDomain)]
		if hasCert {
			if certProviderLabel(cert) != "" {
				cfg.Issuer.Provider = certProviderLabel(cert)
			}
			if v := metaString(cert.Meta, "credential_id", "credentialId"); v != "" {
				cfg.CredentialID = v
			}
			if v := metaString(cert.Meta, "sign_method", "signMethod"); v == "DNS-01" {
				cfg.ChallengePref = "dns"
			}
		}
		if cfg.ChallengePref == "" || cfg.ChallengePref == "http" {
			cfg.ChallengePref = "dns"
		}
		binding.Domain = domain
		binding.Mode = "selected"
		binding.CertificateID = stableID("cert:" + wildcardDomain)
		binding.CertificateDomain = wildcardDomain
		binding.ChallengePref = cfg.ChallengePref
		binding.CredentialID = cfg.CredentialID
		binding.Issuer = cfg.Issuer
		binding.Provider = defaultString(cfg.Issuer.Provider, binding.Provider)
		binding.NiceName = wildcardDomain
		out = append(out, binding)
	}
	return out
}

func certificateBindingsFromDomains(domains []string) []CertificateBinding {
	out := make([]CertificateBinding, 0, len(domains))
	for _, domain := range domains {
		domain = certDomainName(domain)
		if domain == "" {
			continue
		}
		out = append(out, CertificateBinding{Domain: domain, Mode: "auto"})
	}
	return out
}

func bestCertificateForDomain(domain string, certByDomain map[string]npmCertificate) (npmCertificate, bool) {
	if cert, ok := certByDomain[certDomainKey(domain)]; ok && npmCertCoversDomain(cert, domain) {
		return cert, true
	}
	if cert, ok := certByDomain[certDomainKey(wildcardParentDomain(domain))]; ok && npmCertCoversDomain(cert, domain) {
		return cert, true
	}
	return npmCertificate{}, false
}

func certificateBindingsFromMeta(meta map[string]any) []CertificateBinding {
	if meta == nil || meta["certificate_bindings"] == nil {
		return nil
	}
	data, err := json.Marshal(meta["certificate_bindings"])
	if err != nil {
		return nil
	}
	var bindings []CertificateBinding
	if err := json.Unmarshal(data, &bindings); err != nil {
		return nil
	}
	out := make([]CertificateBinding, 0, len(bindings))
	for _, binding := range bindings {
		binding.Domain = certDomainName(binding.Domain)
		if binding.Domain == "" {
			continue
		}
		if binding.Mode == "" {
			if binding.CertificateID > 0 {
				binding.Mode = "selected"
			} else {
				binding.Mode = "auto"
			}
		}
		out = append(out, binding)
	}
	return out
}

func enrichCertificateBindings(bindings []CertificateBinding, certByDomain map[string]npmCertificate) []CertificateBinding {
	if len(bindings) == 0 {
		return bindings
	}
	out := make([]CertificateBinding, len(bindings))
	copy(out, bindings)
	for i := range out {
		out[i].LastError, out[i].LastErrorAt = visibleCertError(out[i].LastError, out[i].LastErrorAt)
		if out[i].Mode == "auto" {
			if cfg, ok := certificateConfigForDomain(out[i].Domain); ok {
				out[i].ChallengePref = cfg.ChallengePref
				out[i].CredentialID = cfg.CredentialID
				out[i].Issuer = cfg.Issuer
				out[i].Provider = defaultString(cfg.Issuer.Provider, out[i].Provider)
				if out[i].NiceName == "" {
					out[i].NiceName = certDomainName(out[i].Domain)
				}
				if cfg.Issuer.Provider != "" && cfg.Issuer.Provider != "auto" {
					out[i].Mode = "selected"
					out[i].CertificateID = stableID("cert:" + certDomainName(cfg.Domain))
					out[i].CertificateDomain = certDomainName(cfg.Domain)
				}
			} else if issuer, ok := inferIssuerFromText(out[i].LastError); ok {
				out[i].ChallengePref = "http"
				if strings.Contains(out[i].LastError, "_acme-challenge") {
					out[i].ChallengePref = "dns"
				}
				out[i].Issuer = issuer
				out[i].Provider = issuer.Provider
				out[i].CertificateID = stableID("cert:" + certDomainName(out[i].Domain))
				out[i].CertificateDomain = certDomainName(out[i].Domain)
				if out[i].NiceName == "" {
					out[i].NiceName = certDomainName(out[i].Domain)
				}
			}
		}
		if out[i].CertificateID <= 0 {
			continue
		}
		cert, ok := certByIDFromDomainMap(certByDomain, out[i].CertificateID)
		if !ok {
			continue
		}
		if out[i].Provider == "" {
			out[i].Provider = certProviderLabel(cert)
		}
		if out[i].CertificateDomain == "" {
			out[i].CertificateDomain = firstDomain(cert.DomainNames)
		}
		if out[i].NiceName == "" {
			out[i].NiceName = cert.NiceName
		}
	}
	return out
}

func proxyHostVisibleCertError(s Site) (string, time.Time) {
	if msg, at := visibleCertError(s.LastError, s.LastErrorAt); msg != "" {
		return msg, at
	}
	for _, binding := range s.CertificateBindings {
		if binding.Mode == "selected" && binding.CertificateID > 0 {
			if _, ok := certificateConfigFromSelectedBinding(binding); ok {
				continue
			}
		}
		if msg, at := visibleCertError(binding.LastError, binding.LastErrorAt); msg != "" {
			return msg, at
		}
	}
	return "", time.Time{}
}

func certProviderLabel(cert npmCertificate) string {
	provider := cert.Provider
	if cert.Meta != nil {
		if v := metaString(cert.Meta, "issuer_provider", "issuerProvider"); v != "" {
			provider = v
		}
	}
	return provider
}

func certByIDFromDomainMap(certByDomain map[string]npmCertificate, id int) (npmCertificate, bool) {
	if id <= 0 {
		return npmCertificate{}, false
	}
	for _, cert := range certByDomain {
		if cert.ID == id {
			return cert, true
		}
	}
	return npmCertificate{}, false
}

func wildcardParentDomain(domain string) string {
	host := certDomainName(domain)
	parts := strings.Split(host, ".")
	if len(parts) < 3 {
		return ""
	}
	return "*." + strings.Join(parts[1:], ".")
}

func npmCertificatesByDomain() map[string]npmCertificate {
	out := map[string]npmCertificate{}
	for _, c := range npmCertificates() {
		for _, d := range c.DomainNames {
			out[certDomainKey(d)] = c
		}
	}
	return out
}

func loadNPMStreams() ([]npmStream, error) {
	items := []npmStream{}
	if err := loadJSONFile(streamPath, &items); err != nil {
		return nil, err
	}
	fallback := timestampFromFile(streamPath)
	for i := range items {
		stampNPMStream(&items[i], fallback, false)
	}
	return items, nil
}

func loadNPMDynamicDNS() ([]npmDynamicDNS, error) {
	items := []npmDynamicDNS{}
	if err := loadJSONFile(dynamicDNSPath, &items); err != nil {
		return nil, err
	}
	creds, _ := loadCredentials()
	fallback := timestampFromFile(dynamicDNSPath)
	for i := range items {
		stampNPMDynamicDNS(&items[i], fallback, false)
		items[i].DNSProvider = effectiveDynamicDNSProvider(items[i], creds)
	}
	return items, nil
}

func loadNPMDomainMonitors() ([]npmDomainMonitor, error) {
	items := []npmDomainMonitor{}
	if err := loadJSONFile(domainMonitorPath, &items); err != nil {
		return nil, err
	}
	fallback := timestampFromFile(domainMonitorPath)
	for i := range items {
		stampNPMDomainMonitor(&items[i], fallback, false)
	}
	return items, nil
}

func loadNPMWakeDevices() ([]npmWakeDevice, error) {
	items := []npmWakeDevice{}
	if err := loadJSONFile(wakeDevicePath, &items); err != nil {
		return nil, err
	}
	fallback := timestampFromFile(wakeDevicePath)
	for i := range items {
		stampNPMWakeDevice(&items[i], fallback, false)
	}
	return items, nil
}

func loadNPMAccessLists() ([]npmAccessList, error) {
	items := []npmAccessList{}
	if err := loadJSONFile(accessListPath, &items); err != nil {
		return nil, err
	}
	counts := accessListProxyHostCounts()
	fallback := timestampFromFile(accessListPath)
	for i := range items {
		stampNPMAccessList(&items[i], fallback, false)
		items[i].ProxyHostCount = counts[items[i].ID]
	}
	return items, nil
}

func accessListProxyHostCounts() map[int]int {
	counts := map[int]int{}
	sites, err := readSitesByKind("proxy")
	if err != nil {
		return counts
	}
	for _, site := range sites {
		if site.AccessListID > 0 {
			counts[site.AccessListID]++
		}
	}
	return counts
}

func accessListByID(id int) (npmAccessList, bool) {
	items, err := loadNPMAccessLists()
	if err != nil {
		return npmAccessList{}, false
	}
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return npmAccessList{}, false
}

func renderSitesAndReload() error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	if err := renderAllSiteConfs(creds); err != nil {
		return err
	}
	backups, err := syncWildcardPlaceholders(creds)
	if err != nil {
		restoreBackups(backups)
		return err
	}
	if err := reloadCaddy(); err != nil {
		restoreBackups(backups)
		return err
	}
	return nil
}

func syncStreamsAndReload() error {
	if err := syncStreamsConfig(); err != nil {
		return err
	}
	return reloadCaddy()
}

func syncDynamicDNSAndReload() error {
	if err := syncDynamicDNSConfig(); err != nil {
		return err
	}
	if err := syncDynamicDNSProviderRecordsBeforeReload(); err != nil {
		log.Printf("dynamic dns provider pre-sync before reload: %v", err)
	}
	if err := reloadCaddy(); err != nil {
		return err
	}
	if err := refreshDynamicDNSStatuses(); err != nil {
		log.Printf("dynamic dns status refresh after save: %v", err)
	}
	return nil
}

func syncDynamicDNSConfig() error {
	items, err := loadNPMDynamicDNS()
	if err != nil {
		return err
	}
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	conf, err := renderDynamicDNSConfig(items, creds)
	if err != nil {
		return err
	}
	if strings.TrimSpace(conf) == "" {
		if err := os.Remove(dynamicDNSConfPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dynamicDNSConfPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(dynamicDNSConfPath, []byte(conf), 0644)
}

func syncDynamicDNSProviderRecordsBeforeReload() error {
	items, err := loadNPMDynamicDNS()
	if err != nil {
		return err
	}
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	cache := map[string][]string{}
	errs := []string{}
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		if errText, _ := checkDynamicDNSItemStatus(item, creds, cache); errText != "" {
			errs = append(errs, fmt.Sprintf("%s: %s", item.Name, errText))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func syncStreamsConfig() error {
	streams, err := loadNPMStreams()
	if err != nil {
		return err
	}
	conf, err := renderLayer4Config(streams)
	if err != nil {
		return err
	}
	if strings.TrimSpace(conf) == "" {
		if err := os.Remove(layer4ConfPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(layer4ConfPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(layer4ConfPath, []byte(conf), 0644)
}

func renderLayer4Config(streams []npmStream) (string, error) {
	var b strings.Builder
	wroteHeader := false
	ports := map[string]bool{}
	for _, stream := range streams {
		if !stream.Enabled {
			continue
		}
		if !stream.TCPForwarding && !stream.UDPForwarding {
			continue
		}
		if stream.IncomingPort <= 0 || stream.IncomingPort > 65535 {
			return "", fmt.Errorf("端口转发入口端口无效: %d", stream.IncomingPort)
		}
		if stream.ForwardingPort <= 0 || stream.ForwardingPort > 65535 {
			return "", fmt.Errorf("端口转发目标端口无效: %d", stream.ForwardingPort)
		}
		host := strings.TrimSpace(stream.ForwardingHost)
		if host == "" {
			return "", fmt.Errorf("端口转发 %d 目标主机不能为空", stream.IncomingPort)
		}
		listeners := []string{}
		if stream.TCPForwarding {
			listeners = append(listeners, fmt.Sprintf("tcp/:%d", stream.IncomingPort))
		}
		if stream.UDPForwarding {
			listeners = append(listeners, fmt.Sprintf("udp/:%d", stream.IncomingPort))
		}
		for _, listener := range listeners {
			if ports[listener] {
				return "", fmt.Errorf("端口转发入口 %s 重复", listener)
			}
			ports[listener] = true
			if !wroteHeader {
				b.WriteString("layer4 {\n")
				wroteHeader = true
			}
			fmt.Fprintf(&b, "    %s {\n", listener)
			b.WriteString("        route {\n")
			fmt.Fprintf(&b, "            proxy %s:%d\n", host, stream.ForwardingPort)
			b.WriteString("        }\n")
			b.WriteString("    }\n")
		}
	}
	if !wroteHeader {
		return "", nil
	}
	b.WriteString("}\n")
	return b.String(), nil
}

func renderDynamicDNSConfig(items []npmDynamicDNS, creds []Credential) (string, error) {
	credByID := map[string]Credential{}
	for _, c := range creds {
		credByID[c.ID] = c
	}
	var b strings.Builder
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		if err := validateDynamicDNS(item); err != nil {
			return "", err
		}
		cred, ok := credByID[item.CredentialID]
		if !ok {
			return "", fmt.Errorf("动态 DNS %s 使用的 DNS 凭据不存在", item.Name)
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("dynamic_dns {\n")
		switch cred.Provider {
		case "alidns":
			fmt.Fprintf(&b, "    provider alidns {\n")
			fmt.Fprintf(&b, "        access_key_id %s\n", caddyQuote(cred.AliyunKey))
			fmt.Fprintf(&b, "        access_key_secret %s\n", caddyQuote(cred.AliyunSecret))
			b.WriteString("    }\n")
		case "cloudflare":
			fmt.Fprintf(&b, "    provider cloudflare %s\n", caddyQuote(cred.CFToken))
		case "dnspod":
			fmt.Fprintf(&b, "    provider dnspod %s\n", caddyQuote(cred.DNSPodToken))
		case "he":
			fmt.Fprintf(&b, "    provider he {\n")
			fmt.Fprintf(&b, "        api_key %s\n", caddyQuote(cred.HEAPIKey))
			b.WriteString("    }\n")
		default:
			return "", fmt.Errorf("动态 DNS %s 使用了不支持的 DNS 提供商：%s", item.Name, cred.Provider)
		}
		b.WriteString("    domains {\n")
		for _, domain := range dynamicDNSDomains(item.DomainNames) {
			line, err := dynamicDNSCaddyDomainLine(domain)
			if err != nil {
				return "", fmt.Errorf("动态 DNS %s 域名 %s 无效: %w", item.Name, domain, err)
			}
			fmt.Fprintf(&b, "        %s\n", line)
		}
		b.WriteString("    }\n")
		fmt.Fprintf(&b, "    check_interval %s\n", item.CheckInterval)
		if item.IPv4 && item.IPv6 {
			b.WriteString("    versions ipv4 ipv6\n")
		} else if item.IPv6 {
			b.WriteString("    versions ipv6\n")
		} else {
			b.WriteString("    versions ipv4\n")
		}
		if strings.TrimSpace(item.TTL) != "" {
			fmt.Fprintf(&b, "    ttl %s\n", strings.TrimSpace(item.TTL))
		}
		for _, resolver := range item.Resolvers {
			resolver = strings.TrimSpace(resolver)
			if resolver != "" {
				fmt.Fprintf(&b, "    resolver %s\n", caddyQuote(resolver))
			}
		}
		for _, ipServiceURL := range dynamicDNSIPSourceURLs(item) {
			fmt.Fprintf(&b, "    ip_source simple_http %s\n", caddyQuote(ipServiceURL))
		}
		b.WriteString("}\n")
	}
	return b.String(), nil
}

func dynamicDNSIPSourceURLs(item npmDynamicDNS) []string {
	if ipServiceURL := normalizeDynamicDNSIPServiceURL(item.IPServiceURL); ipServiceURL != "" {
		return []string{ipServiceURL}
	}
	urls := []string{}
	if item.IPv4 {
		urls = append(urls, publicIPv4Endpoint)
	}
	if item.IPv6 {
		urls = append(urls, publicIPv6Endpoint)
	}
	urls = append(urls, publicIPFallbackEndpoints...)
	return uniqueStringsPreserveOrder(urls)
}

func dynamicDNSCaddyDomainLine(value string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return "", errors.New("域名为空")
	}
	if len(fields) > 1 {
		return strings.Join(fields, " "), nil
	}
	host, wildcard := normalizeDynamicDNSHost(fields[0])
	if host == "" {
		return "", errors.New("域名为空")
	}
	if net.ParseIP(host) != nil {
		return "", errors.New("不能使用 IP 地址")
	}
	zone, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return "", fmt.Errorf("无法计算 DNS Zone，请改用“zone record”格式，例如 example.com www: %w", err)
	}
	if host == zone {
		if wildcard {
			return zone + " *", nil
		}
		return zone + " @", nil
	}
	record := strings.TrimSuffix(host, "."+zone)
	if record == "" || record == host {
		return "", errors.New("无法计算 DNS 记录名")
	}
	if wildcard {
		record = "*." + record
	}
	return zone + " " + record, nil
}

func normalizeDynamicDNSHost(value string) (string, bool) {
	host := strings.ToLower(strings.Trim(strings.TrimSpace(value), "."))
	wildcard := strings.HasPrefix(host, "*.")
	if wildcard {
		host = strings.TrimPrefix(host, "*.")
	}
	return host, wildcard
}

func dynamicDNSHostnames(value string) []string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return nil
	}
	if len(fields) == 1 {
		host, wildcard := normalizeDynamicDNSHost(fields[0])
		if host == "" || wildcard {
			return nil
		}
		return []string{host}
	}
	zone := normalizedDomainMonitorHost(fields[0])
	if zone == "" {
		return nil
	}
	out := []string{}
	for _, record := range fields[1:] {
		record = strings.Trim(strings.TrimSpace(record), ".")
		if record == "" || record == "@" {
			out = append(out, zone)
			continue
		}
		if strings.HasPrefix(record, "*") {
			continue
		}
		record = normalizedDomainMonitorHost(record)
		if record == "" {
			continue
		}
		if record == zone || strings.HasSuffix(record, "."+zone) {
			out = append(out, record)
			continue
		}
		out = append(out, record+"."+zone)
	}
	return uniqueSortedStrings(out)
}

func dynamicDNSRecordTargets(value string) ([]dynamicDNSRecordTarget, error) {
	line, err := dynamicDNSCaddyDomainLine(value)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 2 {
		return nil, errors.New("缺少 DNS 记录名")
	}
	zone := normalizedDomainMonitorHost(fields[0])
	if zone == "" {
		return nil, errors.New("DNS Zone 为空")
	}
	out := []dynamicDNSRecordTarget{}
	for _, rawRecord := range fields[1:] {
		rr := normalizeDynamicDNSRR(rawRecord)
		if rr == "" {
			continue
		}
		if rr != "@" {
			rrHost := normalizedDomainMonitorHost(strings.TrimPrefix(rr, "*."))
			if strings.HasPrefix(rr, "*.") && rrHost == "" {
				return nil, fmt.Errorf("DNS 记录名 %s 无效", rawRecord)
			}
			if rrHost == zone {
				rr = "@"
			} else if rrHost != "" && strings.HasSuffix(rrHost, "."+zone) {
				prefix := strings.TrimSuffix(rrHost, "."+zone)
				if strings.HasPrefix(rr, "*.") {
					rr = "*." + prefix
				} else {
					rr = prefix
				}
			}
		}
		host := zone
		if rr != "@" {
			host = rr + "." + zone
		}
		out = append(out, dynamicDNSRecordTarget{Zone: zone, RR: rr, Host: host})
	}
	return uniqueDynamicDNSRecordTargets(out), nil
}

func normalizeDynamicDNSRR(value string) string {
	rr := strings.ToLower(strings.Trim(strings.TrimSpace(value), "."))
	if rr == "" || rr == "@" {
		return "@"
	}
	return rr
}

func uniqueDynamicDNSRecordTargets(values []dynamicDNSRecordTarget) []dynamicDNSRecordTarget {
	out := []dynamicDNSRecordTarget{}
	seen := map[string]bool{}
	for _, value := range values {
		key := value.Zone + "\x00" + value.RR
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func dynamicDNSDomains(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		for _, domain := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
			domain = strings.TrimSpace(domain)
			if domain == "" {
				continue
			}
			key := strings.ToLower(strings.Join(strings.Fields(domain), " "))
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, strings.Join(strings.Fields(domain), " "))
		}
	}
	return out
}

func validateDynamicDNS(item npmDynamicDNS) error {
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("动态 DNS 名称不能为空")
	}
	if len(dynamicDNSDomains(item.DomainNames)) == 0 {
		return errors.New("动态 DNS 域名不能为空")
	}
	if strings.TrimSpace(item.CredentialID) == "" {
		return errors.New("动态 DNS 需要选择 DNS 凭据")
	}
	if !item.IPv4 && !item.IPv6 {
		return errors.New("动态 DNS 至少需要启用 IPv4 或 IPv6")
	}
	if strings.TrimSpace(item.CheckInterval) == "" {
		return errors.New("动态 DNS 检查间隔不能为空")
	}
	if _, err := validateDynamicDNSIPServiceURL(item.IPServiceURL); err != nil {
		return err
	}
	return nil
}

func normalizeDynamicDNSIPServiceURL(value string) string {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "auto") {
		return ""
	}
	base := strings.TrimRight(value, "/")
	if strings.EqualFold(base, "https://ifconfig.me") || strings.EqualFold(base, "http://ifconfig.me") {
		return base + "/ip"
	}
	return value
}

func validateDynamicDNSIPServiceURL(value string) (string, error) {
	value = normalizeDynamicDNSIPServiceURL(value)
	if value == "" {
		return "", nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("动态 DNS IP 检测接口必须是 http 或 https URL")
	}
	return value, nil
}

func effectiveDynamicDNSProvider(item npmDynamicDNS, creds []Credential) string {
	if provider := dnsProviderForCredentialID(item.CredentialID, creds); provider != "" {
		return provider
	}
	if item.Meta != nil {
		if provider := metaString(item.Meta, "dns_provider", "dnsProvider"); provider != "" {
			return dnsProviderDisplayName(provider)
		}
	}
	return dnsProviderDisplayName(item.DNSProvider)
}

func domainMonitorDomains(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		for _, domain := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
			domain = strings.TrimSpace(domain)
			if domain == "" {
				continue
			}
			key := strings.ToLower(domain)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, domain)
		}
	}
	return out
}

func validateDomainMonitor(item npmDomainMonitor) error {
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("域名监控名称不能为空")
	}
	if len(domainMonitorDomains(item.DomainNames)) == 0 {
		return errors.New("域名监控域名不能为空")
	}
	if !item.CheckSSL && !item.CheckDNS && !item.CheckDomain {
		return errors.New("域名监控至少需要启用域名到期、证书或 DNS 检查")
	}
	if strings.TrimSpace(item.CredentialID) != "" {
		creds, err := loadCredentials()
		if err != nil {
			return err
		}
		cred, ok := findCredential(strings.TrimSpace(item.CredentialID), creds)
		if !ok {
			return errors.New("选择的域名商凭据不存在")
		}
		if cred.Provider != "digitalplat" && cred.Provider != "dnshe" {
			return errors.New("域名监控只能选择 DigitalPlat 或 DNSHE 凭据")
		}
		if cred.Provider == "digitalplat" && strings.TrimSpace(cred.DigitalPlatAPIKey) == "" {
			return errors.New("DigitalPlat 凭据缺少 API Key")
		}
		if cred.Provider == "dnshe" && (strings.TrimSpace(cred.DNSHEAPIKey) == "" || strings.TrimSpace(cred.DNSHEAPISecret) == "") {
			return errors.New("DNSHE 凭据缺少 API Key 或 API Secret")
		}
	}
	for _, days := range item.ReminderDays {
		if days < 0 {
			return errors.New("域名提醒天数不能小于 0")
		}
	}
	if item.RenewBefore < 0 {
		return errors.New("自动续期提前天数不能小于 0")
	}
	if _, err := parseDomainMonitorInterval(item.CheckInterval); err != nil {
		return err
	}
	if item.ThresholdDays < 0 {
		return errors.New("域名监控告警阈值不能小于 0 天")
	}
	return nil
}

func validateWakeDevice(item npmWakeDevice) error {
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("网络唤醒设备名称不能为空")
	}
	if _, err := parseWakeMAC(item.MACAddress); err != nil {
		return errors.New("MAC 地址无效")
	}
	addr := defaultString(strings.TrimSpace(item.BroadcastAddress), "255.255.255.255")
	ip := net.ParseIP(addr)
	if ip == nil || ip.To4() == nil {
		return errors.New("广播地址必须是有效 IPv4 地址")
	}
	port := defaultInt(item.Port, 9)
	if port < 1 || port > 65535 {
		return errors.New("UDP 端口必须在 1 到 65535 之间")
	}
	if strings.TrimSpace(item.SecureOn) != "" {
		if _, err := parseWakeMAC(item.SecureOn); err != nil {
			return errors.New("SecureOn 密码必须是 6 字节十六进制")
		}
	}
	return nil
}

func parseWakeMAC(value string) (net.HardwareAddr, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("empty mac")
	}
	if hw, err := net.ParseMAC(value); err == nil && len(hw) == 6 {
		return hw, nil
	}
	compact := strings.NewReplacer(":", "", "-", "", ".", "", " ", "", "\t", "", "\n", "", "\r", "").Replace(value)
	if len(compact) != 12 {
		return nil, errors.New("invalid mac length")
	}
	decoded, err := hex.DecodeString(compact)
	if err != nil || len(decoded) != 6 {
		return nil, errors.New("invalid mac")
	}
	return net.HardwareAddr(decoded), nil
}

func normalizeWakeMAC(value string) string {
	hw, err := parseWakeMAC(value)
	if err != nil {
		return strings.TrimSpace(value)
	}
	return strings.ToUpper(hw.String())
}

func buildWakeMagicPacket(mac net.HardwareAddr, secureOn []byte) []byte {
	packet := make([]byte, 6, 6+16*len(mac)+len(secureOn))
	for i := range packet {
		packet[i] = 0xff
	}
	for i := 0; i < 16; i++ {
		packet = append(packet, mac...)
	}
	if len(secureOn) > 0 {
		packet = append(packet, secureOn...)
	}
	return packet
}

func sendWakePacket(item npmWakeDevice) error {
	mac, err := parseWakeMAC(item.MACAddress)
	if err != nil {
		return err
	}
	var secureOn []byte
	if strings.TrimSpace(item.SecureOn) != "" {
		secureOn, err = parseWakeMAC(item.SecureOn)
		if err != nil {
			return err
		}
	}
	packet := buildWakeMagicPacket(mac, secureOn)
	target := net.JoinHostPort(defaultString(strings.TrimSpace(item.BroadcastAddress), "255.255.255.255"), strconv.Itoa(defaultInt(item.Port, 9)))
	addr, err := net.ResolveUDPAddr("udp4", target)
	if err != nil {
		return err
	}
	conn, err := listenUDP4Broadcast(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	if udpConn, ok := conn.(*net.UDPConn); ok {
		_ = udpConn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	}
	n, err := conn.WriteTo(packet, addr)
	if err != nil {
		return err
	}
	if n != len(packet) {
		return io.ErrShortWrite
	}
	return nil
}

func runDomainMonitorLoop() {
	time.Sleep(10 * time.Second)
	if err := checkDueDomainMonitors(); err != nil {
		log.Printf("domain monitor: %v", err)
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if err := checkDueDomainMonitors(); err != nil {
			log.Printf("domain monitor: %v", err)
		}
	}
}

func checkDueDomainMonitors() error {
	items, err := loadNPMDomainMonitors()
	if err != nil {
		return err
	}
	now := time.Now()
	changed := false
	for i := range items {
		if !items[i].Enabled {
			continue
		}
		if !domainMonitorDue(items[i], now) {
			continue
		}
		if err := validateDomainMonitor(items[i]); err != nil {
			if items[i].Meta == nil {
				items[i].Meta = map[string]any{}
			}
			interval, parseErr := parseDomainMonitorInterval(items[i].CheckInterval)
			if parseErr != nil {
				interval = 24 * time.Hour
			}
			items[i].Meta["status"] = "error"
			items[i].Meta["last_checked"] = localTimestamp(now)
			items[i].Meta["next_check"] = localTimestamp(now.Add(interval))
			items[i].Meta["last_error"] = err.Error()
			changed = true
			continue
		}
		runDomainMonitorCheck(&items[i])
		changed = true
	}
	if !changed {
		return nil
	}
	return saveJSONFile(domainMonitorPath, items)
}

func domainMonitorDue(item npmDomainMonitor, now time.Time) bool {
	if item.Meta == nil {
		return true
	}
	nextText := metaString(item.Meta, "next_check", "nextCheck")
	if strings.TrimSpace(nextText) == "" {
		return true
	}
	next, err := time.Parse(time.RFC3339, nextText)
	if err != nil {
		return true
	}
	return !next.After(now)
}

func runDomainMonitorCheck(item *npmDomainMonitor) {
	stampNPMDomainMonitor(item, "", true)
	previousStatus := metaString(item.Meta, "status")
	now := time.Now()
	interval, err := parseDomainMonitorInterval(item.CheckInterval)
	if err != nil {
		interval = 24 * time.Hour
	}
	threshold := item.ThresholdDays
	if threshold <= 0 {
		threshold = 30
	}

	results := []domainMonitorResult{}
	allIPs := []string{}
	errorsOut := []string{}
	status := "ok"
	var earliestExpiry time.Time
	earliestDays := 0
	earliestIssuer := ""
	var earliestDomainExpiry time.Time
	earliestDomainDays := 0
	earliestDomainName := ""
	primaryDomainName := ""
	domainExpiryUnavailable := false
	domainExpiryUnavailableReasons := []string{}

	for _, domain := range domainMonitorDomains(item.DomainNames) {
		result := domainMonitorResult{Domain: domain, Status: "ok"}
		host, port := domainMonitorEndpoint(domain)
		if host == "" {
			result.Status = "error"
			result.Error = "域名格式无效"
			errorsOut = append(errorsOut, fmt.Sprintf("%s: %s", domain, result.Error))
			results = append(results, result)
			status = "error"
			continue
		}

		if item.CheckDNS {
			ips, lookupErr := resolveDomainIPs(host, item.Resolvers)
			if lookupErr != nil {
				result.Status = "error"
				result.Error = appendErrorText(result.Error, "DNS: "+lookupErr.Error())
				notifyDomainMonitorCheckFailed(notificationEventDNSCheckFailed, domain, "DNS 检查失败", lookupErr.Error())
			} else {
				result.ResolvedIPs = ips
				allIPs = append(allIPs, ips...)
			}
		}

		if item.CheckDomain {
			registeredDomain, expiresAt, expiryErr := fetchDomainRegistrationExpiry(host, item.CredentialID)
			if registeredDomain != "" {
				result.DomainName = registeredDomain
				if primaryDomainName == "" {
					primaryDomainName = registeredDomain
				}
			}
			if expiryErr != nil {
				if !errors.Is(expiryErr, errDomainRegistrationExpiryUnavailable) {
					result.Status = "error"
					result.Error = appendErrorText(result.Error, "域名到期: "+expiryErr.Error())
					notifyDomainMonitorCheckFailed(notificationEventDomainCheckFailed, registeredDomain, "域名到期检查失败", expiryErr.Error())
				} else {
					result.DomainExpiryUnavailable = true
					if reason := domainExpiryUnavailableReason(expiryErr); reason != "" {
						result.DomainExpiryReason = reason
						domainExpiryUnavailableReasons = append(domainExpiryUnavailableReasons, fmt.Sprintf("%s: %s", registeredDomain, reason))
					}
					domainExpiryUnavailable = true
				}
			} else {
				daysLeft := int(math.Ceil(time.Until(expiresAt).Hours() / 24))
				if item.AutoRenew && domainMonitorRegistrarProvider(registeredDomain) == "dnshe" && daysLeft <= effectiveRenewBeforeDays(item) {
					renewResult, renewErr := renewDNSHEDomain(registeredDomain, item.CredentialID)
					if renewErr != nil {
						notifyDomainRenewFailed(registeredDomain, renewErr.Error())
						result.Status = "error"
						result.Error = appendErrorText(result.Error, "域名续期: "+renewErr.Error())
					} else {
						notifyDomainRenewSuccess(registeredDomain, renewResult.NewExpiresOn)
						if renewedAt, parseErr := parseDigitalPlatExpiryDate(renewResult.NewExpiresOn); parseErr == nil {
							expiresAt = renewedAt
							daysLeft = int(math.Ceil(time.Until(expiresAt).Hours() / 24))
						} else if _, refreshedAt, refreshErr := fetchDomainRegistrationExpiry(host, item.CredentialID); refreshErr == nil {
							expiresAt = refreshedAt
							daysLeft = int(math.Ceil(time.Until(expiresAt).Hours() / 24))
						}
					}
				}
				result.DomainExpiresOn = localTimestamp(expiresAt)
				result.DomainDaysLeft = daysLeft
				notifyDomainExpiryIfNeeded(registeredDomain, daysLeft, expiresAt, *item)
				if earliestDomainExpiry.IsZero() || expiresAt.Before(earliestDomainExpiry) {
					earliestDomainExpiry = expiresAt
					earliestDomainDays = daysLeft
					earliestDomainName = registeredDomain
				}
				if daysLeft < 0 {
					result.Status = "error"
					result.Error = appendErrorText(result.Error, "域名到期: 域名已过期")
				} else if daysLeft <= threshold && result.Status != "error" {
					result.Status = "warning"
				}
			}
		}

		if item.CheckSSL {
			cert, tlsErr := fetchDomainCertificate(host, port)
			if tlsErr != nil {
				if caddyCert, ok := caddyManagedCertificateForDomain(host); ok {
					if err := applyNPMCertificateToDomainMonitorResult(&result, caddyCert, threshold); err != nil {
						result.Status = "error"
						result.Error = appendErrorText(result.Error, "SSL: "+err.Error())
					} else {
						result.SSLSource = "caddy"
						if earliestExpiry.IsZero() || mustParseRFC3339(caddyCert.ExpiresOn).Before(earliestExpiry) {
							earliestExpiry = mustParseRFC3339(caddyCert.ExpiresOn)
							earliestDays = result.SSLDaysLeft
							earliestIssuer = result.SSLIssuer
						}
					}
				} else {
					result.Status = "error"
					result.Error = appendErrorText(result.Error, "SSL: "+domainMonitorTLSError(tlsErr, port))
					notifyDomainMonitorCheckFailed(notificationEventSSLCheckFailed, host, "证书检查失败", domainMonitorTLSError(tlsErr, port))
				}
			} else {
				daysLeft := int(math.Ceil(time.Until(cert.NotAfter).Hours() / 24))
				result.SSLExpiresOn = localTimestamp(cert.NotAfter)
				result.SSLDaysLeft = daysLeft
				result.SSLIssuer = certificateIssuerName(cert)
				notifySSLExpiryIfNeeded(host, daysLeft, cert.NotAfter, threshold)
				if earliestExpiry.IsZero() || cert.NotAfter.Before(earliestExpiry) {
					earliestExpiry = cert.NotAfter
					earliestDays = daysLeft
					earliestIssuer = result.SSLIssuer
				}
				if err := cert.VerifyHostname(host); err != nil {
					result.Status = "error"
					result.Error = appendErrorText(result.Error, "SSL: "+err.Error())
					notifyDomainMonitorCheckFailed(notificationEventSSLCheckFailed, host, "证书检查失败", err.Error())
				} else if daysLeft < 0 {
					result.Status = "error"
					result.Error = appendErrorText(result.Error, "SSL: 证书已过期")
				} else if daysLeft <= threshold && result.Status != "error" {
					result.Status = "warning"
				}
			}
		}

		if result.Error != "" {
			errorsOut = append(errorsOut, fmt.Sprintf("%s: %s", domain, result.Error))
			status = "error"
		} else if result.Status == "warning" && status != "error" {
			status = "warning"
		}
		results = append(results, result)
	}

	item.Meta["status"] = status
	item.Meta["last_checked"] = localTimestamp(now)
	item.Meta["next_check"] = localTimestamp(now.Add(interval))
	item.Meta["results"] = results
	item.Meta["resolved_ips"] = uniqueSortedStrings(allIPs)
	item.Meta["last_error"] = strings.Join(errorsOut, "; ")
	if provider := effectiveDomainMonitorRegistrarProvider(*item); provider != "" {
		item.RegistrarProvider = provider
		item.Meta["registrar_provider"] = provider
	} else {
		delete(item.Meta, "registrar_provider")
	}
	if earliestDomainExpiry.IsZero() {
		if primaryDomainName == "" {
			delete(item.Meta, "domain_name")
		} else {
			item.Meta["domain_name"] = primaryDomainName
		}
		delete(item.Meta, "domain_expires_on")
		delete(item.Meta, "domain_days_left")
		if domainExpiryUnavailable {
			item.Meta["domain_expiry_unavailable"] = true
			if len(domainExpiryUnavailableReasons) > 0 {
				item.Meta["domain_expiry_unavailable_reason"] = strings.Join(domainExpiryUnavailableReasons, "; ")
			} else {
				delete(item.Meta, "domain_expiry_unavailable_reason")
			}
		} else {
			delete(item.Meta, "domain_expiry_unavailable")
			delete(item.Meta, "domain_expiry_unavailable_reason")
		}
	} else {
		item.Meta["domain_name"] = earliestDomainName
		item.Meta["domain_expires_on"] = localTimestamp(earliestDomainExpiry)
		item.Meta["domain_days_left"] = earliestDomainDays
		delete(item.Meta, "domain_expiry_unavailable")
		delete(item.Meta, "domain_expiry_unavailable_reason")
	}
	if earliestExpiry.IsZero() {
		delete(item.Meta, "ssl_expires_on")
		delete(item.Meta, "ssl_days_left")
		delete(item.Meta, "ssl_issuer")
	} else {
		item.Meta["ssl_expires_on"] = localTimestamp(earliestExpiry)
		item.Meta["ssl_days_left"] = earliestDays
		item.Meta["ssl_issuer"] = earliestIssuer
	}
	notifyMonitorStatusChange(item.Name, previousStatus, status, strings.Join(errorsOut, "; "))
}

func domainExpiryUnavailableReason(err error) string {
	if err == nil {
		return ""
	}
	reason := strings.TrimSpace(err.Error())
	if reason == "" || reason == errDomainRegistrationExpiryUnavailable.Error() {
		return "未配置域名商凭据"
	}
	reason = strings.ReplaceAll(reason, errDomainRegistrationExpiryUnavailable.Error()+": ", "")
	return reason
}

func domainMonitorEndpoint(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "443"
	}
	if strings.Contains(value, "://") {
		if u, err := url.Parse(value); err == nil {
			value = u.Host
		}
	}
	if idx := strings.Index(value, "/"); idx >= 0 {
		value = value[:idx]
	}
	host, port, ok := splitHostListenPort(value)
	if ok {
		if host == "" {
			return "", strconv.Itoa(port)
		}
		return strings.Trim(host, "[]"), strconv.Itoa(port)
	}
	return strings.Trim(value, "[]"), "443"
}

type rdapDomainResponse struct {
	Events []rdapEvent `json:"events"`
}

type rdapEvent struct {
	EventAction string `json:"eventAction"`
	EventDate   string `json:"eventDate"`
}

type digitalPlatDomainsResponse struct {
	Success bool                `json:"success"`
	Data    []digitalPlatDomain `json:"data"`
	Message string              `json:"message,omitempty"`
	Error   string              `json:"error,omitempty"`
}

type digitalPlatDomain struct {
	Name          string `json:"name"`
	Domain        string `json:"domain,omitempty"`
	Status        string `json:"status,omitempty"`
	SlotType      string `json:"slot_type,omitempty"`
	LifecycleType string `json:"lifecycle_type,omitempty"`
	ExpiryDate    string `json:"expiry_date"`
	ExpiresAt     string `json:"expires_at,omitempty"`
}

func normalizedDomainMonitorHost(host string) string {
	host = strings.Trim(strings.TrimSpace(host), ".")
	if strings.HasPrefix(host, "*.") {
		host = strings.TrimPrefix(host, "*.")
	}
	return strings.ToLower(host)
}

func domainMonitorRegisteredDomain(host string) (string, bool, error) {
	host = normalizedDomainMonitorHost(host)
	if host == "" {
		return "", false, errors.New("域名为空")
	}
	if net.ParseIP(host) != nil {
		return "", false, errors.New("IP 地址没有域名注册到期信息")
	}
	for _, suffix := range domainMonitorRegistrarHostedZones() {
		suffix = strings.Trim(strings.ToLower(strings.TrimSpace(suffix)), ".")
		if suffix == "" || host == suffix || !strings.HasSuffix(host, "."+suffix) {
			continue
		}
		prefix := strings.TrimSuffix(host, "."+suffix)
		labels := strings.Split(prefix, ".")
		for i := len(labels) - 1; i >= 0; i-- {
			label := strings.TrimSpace(labels[i])
			if label != "" {
				return label + "." + suffix, true, nil
			}
		}
	}
	registeredDomain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return "", false, fmt.Errorf("无法识别可注册域名: %w", err)
	}
	return registeredDomain, false, nil
}

func fetchDomainRegistrationExpiry(host string, credentialID string) (string, time.Time, error) {
	registeredDomain, hostedZone, err := domainMonitorRegisteredDomain(host)
	if err != nil {
		return "", time.Time{}, err
	}
	if hostedZone {
		provider := domainMonitorRegistrarProvider(registeredDomain)
		if provider == "" {
			return registeredDomain, time.Time{}, fmt.Errorf("%w: %s 属于免费/托管二级域名，当前没有可用的到期查询接口", errDomainRegistrationExpiryUnavailable, registeredDomain)
		}
		cred, ok, err := domainRegistrarCredentialForDomainMonitor(provider, credentialID)
		if err != nil {
			return registeredDomain, time.Time{}, err
		}
		if !ok {
			return registeredDomain, time.Time{}, errDomainRegistrationExpiryUnavailable
		}
		var expiresAt time.Time
		switch provider {
		case "digitalplat":
			expiresAt, err = fetchDigitalPlatDomainExpiry(registeredDomain, cred.DigitalPlatAPIKey)
		case "dnshe":
			expiresAt, err = fetchDNSHEDomainExpiry(registeredDomain, cred)
		default:
			err = errDomainRegistrationExpiryUnavailable
		}
		if err != nil {
			return registeredDomain, time.Time{}, err
		}
		return registeredDomain, expiresAt, nil
	}
	client := &http.Client{Timeout: domainMonitorRDAPTimeout}
	rdapURL := "https://rdap.org/domain/" + url.PathEscape(registeredDomain)
	res, err := client.Get(rdapURL)
	if err != nil {
		return registeredDomain, time.Time{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return registeredDomain, time.Time{}, fmt.Errorf("RDAP 返回 %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 1024*1024))
	if err != nil {
		return registeredDomain, time.Time{}, err
	}
	var payload rdapDomainResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return registeredDomain, time.Time{}, err
	}
	for _, event := range payload.Events {
		action := strings.ToLower(strings.TrimSpace(event.EventAction))
		if !strings.Contains(action, "expiration") && !strings.Contains(action, "expiry") {
			continue
		}
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(event.EventDate))
		if err != nil {
			return registeredDomain, time.Time{}, err
		}
		return registeredDomain, t, nil
	}
	return registeredDomain, time.Time{}, errors.New("RDAP 未返回域名到期时间")
}

func digitalPlatBaseURL() string {
	if base := strings.TrimRight(strings.TrimSpace(os.Getenv("DIGITALPLAT_API_BASE_URL")), "/"); base != "" {
		return base
	}
	return digitalPlatAPIBaseURL
}

func digitalPlatCredentialForDomainMonitor(credentialID string) (Credential, bool, error) {
	creds, err := loadCredentials()
	if err != nil {
		return Credential{}, false, err
	}
	credentialID = strings.TrimSpace(credentialID)
	if credentialID != "" {
		cred, ok := findCredential(credentialID, creds)
		if !ok {
			return Credential{}, false, errors.New("选择的 DigitalPlat 凭据不存在")
		}
		if cred.Provider != "digitalplat" {
			return Credential{}, false, errors.New("域名监控只能选择 DigitalPlat 凭据")
		}
		if strings.TrimSpace(cred.DigitalPlatAPIKey) == "" {
			return Credential{}, false, errors.New("DigitalPlat 凭据缺少 API Key")
		}
		return cred, true, nil
	}

	matches := []Credential{}
	for _, cred := range creds {
		if cred.Provider != "digitalplat" || strings.TrimSpace(cred.DigitalPlatAPIKey) == "" {
			continue
		}
		matches = append(matches, cred)
	}
	if len(matches) == 0 {
		return Credential{}, false, nil
	}
	if len(matches) > 1 {
		return Credential{}, false, errors.New("存在多个 DigitalPlat 凭据，请在域名监控中选择一个")
	}
	return matches[0], true, nil
}

func fetchDigitalPlatDomainExpiry(domain string, apiKey string) (time.Time, error) {
	domain = normalizedDomainMonitorHost(domain)
	if domain == "" {
		return time.Time{}, errors.New("域名为空")
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return time.Time{}, errDomainRegistrationExpiryUnavailable
	}

	req, err := http.NewRequest(http.MethodGet, digitalPlatBaseURL()+"/domains", nil)
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/125 Safari/537.36")
	client := &http.Client{Timeout: domainMonitorAPITimeout}
	res, err := client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1024*1024))
	if err != nil {
		return time.Time{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		detail := strings.TrimSpace(string(body))
		if len(detail) > 300 {
			detail = detail[:300] + "..."
		}
		message := fmt.Sprintf("DigitalPlat 返回 %s，请检查 API Key 是否有权访问该域名", res.Status)
		if looksLikeChallengePage(detail) {
			message = fmt.Sprintf("DigitalPlat API 返回 %s，疑似被防护页拦截，请检查当前服务器到 domain-api.digitalplat.org 的访问", res.Status)
			detail = ""
		}
		if detail != "" {
			message += ": " + detail
		}
		if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
			return time.Time{}, fmt.Errorf("%w: %s", errDomainRegistrationExpiryUnavailable, message)
		}
		return time.Time{}, errors.New(message)
	}

	var payload digitalPlatDomainsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return time.Time{}, err
	}
	if !payload.Success {
		msg := firstNonEmpty(payload.Message, payload.Error, "请求失败")
		return time.Time{}, fmt.Errorf("DigitalPlat 返回失败: %s", msg)
	}

	for _, item := range payload.Data {
		name := normalizedDomainMonitorHost(firstNonEmpty(item.Name, item.Domain))
		if name != domain {
			continue
		}
		expiryValue := firstNonEmpty(item.ExpiryDate, item.ExpiresAt)
		if strings.TrimSpace(expiryValue) == "" {
			return time.Time{}, fmt.Errorf("%w: DigitalPlat 未返回 %s 的到期时间", errDomainRegistrationExpiryUnavailable, domain)
		}
		expiresAt, err := parseDigitalPlatExpiryDate(expiryValue)
		if err != nil {
			return time.Time{}, fmt.Errorf("DigitalPlat 到期时间解析失败: %w", err)
		}
		return expiresAt, nil
	}

	return time.Time{}, fmt.Errorf("%w: DigitalPlat 未返回域名 %s，请确认该 API Key 属于此域名账号", errDomainRegistrationExpiryUnavailable, domain)
}

func looksLikeChallengePage(body string) bool {
	text := strings.ToLower(body)
	return strings.Contains(text, "challenge page") ||
		strings.Contains(text, "cloudflare") ||
		strings.Contains(text, "cf-chl") ||
		strings.Contains(text, "cf-ray")
}

func parseDigitalPlatExpiryDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("时间为空")
	}
	loc := time.FixedZone("CST", 8*60*60)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", value, loc); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("20060102", value, loc); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, loc), nil
	}
	t, err := time.ParseInLocation("2006-01-02", value, loc)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, loc), nil
}

func fetchDomainCertificate(host string, port string) (*x509.Certificate, error) {
	address := net.JoinHostPort(host, port)
	dialer := &net.Dialer{Timeout: domainMonitorTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, errors.New("服务器未返回证书")
	}
	return certs[0], nil
}

func caddyManagedCertificateForDomain(domain string) (npmCertificate, bool) {
	cert, ok := bestCertificateForDomain(domain, npmCertificatesByDomain())
	if !ok {
		return npmCertificate{}, false
	}
	status := metaString(cert.Meta, "status")
	if status == "pending" || status == "failed" || status == "disabled" {
		return npmCertificate{}, false
	}
	if strings.TrimSpace(cert.ExpiresOn) == "" {
		return npmCertificate{}, false
	}
	if _, err := time.Parse(time.RFC3339, cert.ExpiresOn); err != nil {
		return npmCertificate{}, false
	}
	return cert, true
}

func applyNPMCertificateToDomainMonitorResult(result *domainMonitorResult, cert npmCertificate, threshold int) error {
	expiresAt, err := time.Parse(time.RFC3339, cert.ExpiresOn)
	if err != nil {
		return err
	}
	daysLeft := int(math.Ceil(time.Until(expiresAt).Hours() / 24))
	result.SSLExpiresOn = localTimestamp(expiresAt)
	result.SSLDaysLeft = daysLeft
	result.SSLIssuer = firstNonEmpty(metaString(cert.Meta, "issuer"), certProviderLabel(cert), cert.Provider)
	if daysLeft < 0 {
		result.Status = "error"
		result.Error = appendErrorText(result.Error, "SSL: 证书已过期")
	} else if daysLeft <= threshold && result.Status != "error" {
		result.Status = "warning"
	}
	return nil
}

func mustParseRFC3339(value string) time.Time {
	t, _ := time.Parse(time.RFC3339, value)
	return t
}

func domainMonitorTLSError(err error, port string) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Sprintf("HTTPS 端口 %s 连接超时，请确认目标域名开放 HTTPS，或关闭 SSL 检查", port)
	}
	return err.Error()
}

func resolveDomainIPs(host string, resolvers []string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resolver := net.DefaultResolver
	if resolverAddr := firstResolverAddress(resolvers); resolverAddr != "" {
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, "udp", resolverAddr)
			},
		}
	}
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		ips = append(ips, addr.IP.String())
	}
	return uniqueSortedStrings(ips), nil
}

func resolveDomainIPsByVersion(host string, resolvers []string, wantIPv4 bool, wantIPv6 bool) ([]string, error) {
	ips, err := resolveDomainIPs(host, resolvers)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, value := range ips {
		ip := net.ParseIP(value)
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			if wantIPv4 {
				out = append(out, value)
			}
			continue
		}
		if wantIPv6 {
			out = append(out, value)
		}
	}
	return uniqueSortedStrings(out), nil
}

func firstResolverAddress(resolvers []string) string {
	for _, resolver := range resolvers {
		resolver = strings.TrimSpace(resolver)
		if resolver == "" {
			continue
		}
		if strings.Contains(resolver, "://") {
			if u, err := url.Parse(resolver); err == nil {
				resolver = u.Host
			}
		}
		if host, port, err := net.SplitHostPort(resolver); err == nil && host != "" && port != "" {
			return net.JoinHostPort(strings.Trim(host, "[]"), port)
		}
		if strings.Count(resolver, ":") > 1 {
			return net.JoinHostPort(strings.Trim(resolver, "[]"), "53")
		}
		return net.JoinHostPort(resolver, "53")
	}
	return ""
}

func parseDomainMonitorInterval(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 24 * time.Hour, nil
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days <= 0 {
			return 0, errors.New("域名监控检查间隔无效")
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, errors.New("域名监控检查间隔无效")
	}
	return d, nil
}

func certificateIssuerName(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	cn := strings.TrimSpace(cert.Issuer.CommonName)
	org := ""
	for _, value := range cert.Issuer.Organization {
		if strings.TrimSpace(value) != "" {
			org = strings.TrimSpace(value)
			break
		}
	}
	if org != "" && cn != "" && !strings.EqualFold(org, cn) {
		return org + "（" + cn + "）"
	}
	return readableCertificateIssuerLabel(firstNonEmpty(org, cn))
}

func readableCertificateIssuerLabel(issuer string) string {
	issuer = strings.TrimSpace(issuer)
	if issuer == "" || strings.Contains(issuer, "（") {
		return issuer
	}
	upper := strings.ToUpper(issuer)
	if issuerCodeWithDigits(upper, "YE") || issuerCodeWithDigits(upper, "R") || issuerCodeWithDigits(upper, "E") {
		return "Let's Encrypt（" + issuer + "）"
	}
	if issuerCodeWithDigits(upper, "WE") {
		return "Google Trust Services（" + issuer + "）"
	}
	return issuer
}

func issuerCodeWithDigits(value string, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return false
	}
	for _, r := range value[len(prefix):] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func appendErrorText(base string, extra string) string {
	extra = strings.TrimSpace(extra)
	if extra == "" {
		return base
	}
	if base == "" {
		return extra
	}
	return base + "; " + extra
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func loadJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

func saveJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(value, "", "  ")
	return os.WriteFile(path, data, 0644)
}

func loadSystemSettings() (SystemSettings, error) {
	cfg := SystemSettings{}
	if err := loadJSONFile(settingsPath, &cfg); err != nil {
		return SystemSettings{}, err
	}
	if cfg.ACMEContactEmail == "" {
		cfg.ACMEContactEmail = fmt.Sprintf("noreply-%s@localhost", randomHex(8))
	}
	return cfg, nil
}

func saveSystemSettings(cfg SystemSettings) error {
	return saveJSONFile(settingsPath, cfg)
}

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := loadSystemSettings()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	case http.MethodPut:
		var cfg SystemSettings
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := saveSystemSettings(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func loadAuditLogs() ([]auditEntry, error) {
	items := []auditEntry{}
	if err := loadJSONFile(auditLogPath, &items); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].User = localNPMUser
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	return items, nil
}

func appendAuditLog(objectType string, objectID int, action string, meta map[string]any) {
	items, err := loadAuditLogs()
	if err != nil {
		log.Printf("load audit log: %v", err)
		return
	}
	if len(items) > 0 && auditDuplicate(items[0], objectType, objectID, action, meta) {
		return
	}
	now := localTimestamp(time.Now())
	item := auditEntry{
		ID:         int(time.Now().UnixNano() / int64(time.Millisecond)),
		CreatedOn:  now,
		ModifiedOn: now,
		UserID:     1,
		ObjectType: objectType,
		ObjectID:   objectID,
		Action:     action,
		Meta:       meta,
		User:       localNPMUser,
	}
	items = append([]auditEntry{item}, items...)
	if len(items) > 500 {
		items = items[:500]
	}
	if err := saveJSONFile(auditLogPath, items); err != nil {
		log.Printf("save audit log: %v", err)
	}
}

func auditDuplicate(item auditEntry, objectType string, objectID int, action string, meta map[string]any) bool {
	if item.ObjectType != objectType || item.ObjectID != objectID || item.Action != action {
		return false
	}
	a, _ := json.Marshal(item.Meta)
	b, _ := json.Marshal(meta)
	return bytes.Equal(a, b)
}

type npmID interface {
	*npmStream | *npmAccessList | *npmDynamicDNS | *npmDomainMonitor | *npmWakeDevice
}

func handleJSONBackedItem[T any](w http.ResponseWriter, r *http.Request, prefix string, path string, loader func() ([]T, error), stamp func(*T), afterSave func() error, validators ...func(T) error) {
	id, action, ok := parseNPMItemPath(w, r, prefix)
	if !ok {
		return
	}
	items, err := loader()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idx := -1
	for i := range items {
		if getNPMItemID(any(items[i])) == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}
	if action != "" && r.Method == http.MethodPost {
		if action == "enable" || action == "disable" {
			setNPMItemEnabled(any(&items[idx]), action == "enable")
			if err := saveJSONFile(path, items); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if afterSave != nil {
				if err := afterSave(); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			appendAuditLog(auditTypeForJSONItem(items[idx]), id, action+"d", auditMetaForJSONItem(items[idx]))
			writeJSON(w, http.StatusOK, true)
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, items[idx])
	case http.MethodPut:
		next := items[idx]
		if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		preserveJSONBackedItemFields(any(&next), any(items[idx]))
		setNPMItemID(any(&next), id)
		stamp(&next)
		if len(validators) > 0 && validators[0] != nil {
			if err := validators[0](next); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		items[idx] = next
		if err := saveJSONFile(path, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if afterSave != nil {
			if err := afterSave(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		finalizeJSONBackedItem(any(&next))
		appendAuditLog(auditTypeForJSONItem(next), id, "updated", auditMetaForJSONItem(next))
		writeJSON(w, http.StatusOK, next)
	case http.MethodDelete:
		deleted := items[idx]
		items = append(items[:idx], items[idx+1:]...)
		if err := saveJSONFile(path, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if afterSave != nil {
			if err := afterSave(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		appendAuditLog(auditTypeForJSONItem(deleted), id, "deleted", auditMetaForJSONItem(deleted))
		writeJSON(w, http.StatusOK, true)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func parseNPMItemPath(w http.ResponseWriter, r *http.Request, prefix string) (int, string, bool) {
	path := prefix
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return 0, "", false
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, "", false
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	return id, action, true
}

func getNPMItemID(item any) int {
	switch v := item.(type) {
	case npmStream:
		return v.ID
	case npmAccessList:
		return v.ID
	case npmDynamicDNS:
		return v.ID
	case npmDomainMonitor:
		return v.ID
	case npmWakeDevice:
		return v.ID
	}
	return 0
}

func setNPMItemID(item any, id int) {
	switch v := item.(type) {
	case *npmStream:
		v.ID = id
	case *npmAccessList:
		v.ID = id
	case *npmDynamicDNS:
		v.ID = id
	case *npmDomainMonitor:
		v.ID = id
	case *npmWakeDevice:
		v.ID = id
	}
}

func setNPMItemEnabled(item any, enabled bool) {
	switch v := item.(type) {
	case *npmStream:
		v.Enabled = enabled
	case *npmDynamicDNS:
		v.Enabled = enabled
	case *npmDomainMonitor:
		v.Enabled = enabled
	case *npmWakeDevice:
		v.Enabled = enabled
	}
}

func finalizeJSONBackedItem(item any) {
	switch v := item.(type) {
	case *npmDynamicDNS:
		if creds, err := loadCredentials(); err == nil {
			v.DNSProvider = effectiveDynamicDNSProvider(*v, creds)
		}
	}
}

func preserveJSONBackedItemFields(next any, previous any) {
	switch item := next.(type) {
	case *npmAccessList:
		if old, ok := previous.(npmAccessList); ok {
			preserveAccessListPasswords(item, old)
		}
	}
}

func preserveAccessListPasswords(next *npmAccessList, previous npmAccessList) {
	passwords := map[string]string{}
	for _, item := range previous.Items {
		username := strings.TrimSpace(item.Username)
		if username == "" || item.Password == "" {
			continue
		}
		passwords[username] = item.Password
	}
	for i := range next.Items {
		username := strings.TrimSpace(next.Items[i].Username)
		if username == "" || strings.TrimSpace(next.Items[i].Password) != "" {
			continue
		}
		if password, ok := passwords[username]; ok {
			next.Items[i].Password = password
		}
	}
}

func stampNPMStream(item *npmStream, fallback string, touchModified bool) {
	stampCreatedModified(&item.CreatedOn, &item.ModifiedOn, fallback, touchModified)
	item.OwnerUserID = 1
	if item.Meta == nil {
		item.Meta = map[string]any{}
	}
	if !item.TCPForwarding && !item.UDPForwarding {
		item.TCPForwarding = true
	}
	item.Owner = localNPMUser
}

func stampNPMDynamicDNS(item *npmDynamicDNS, fallback string, touchModified bool) {
	stampCreatedModified(&item.CreatedOn, &item.ModifiedOn, fallback, touchModified)
	item.OwnerUserID = 1
	if item.Meta == nil {
		item.Meta = map[string]any{}
	}
	item.DomainNames = dynamicDNSDomains(item.DomainNames)
	if !item.IPv4 && !item.IPv6 {
		item.IPv4 = true
	}
	if strings.TrimSpace(item.CheckInterval) == "" {
		item.CheckInterval = "5m"
	}
	item.IPServiceURL = normalizeDynamicDNSIPServiceURL(item.IPServiceURL)
	item.Owner = localNPMUser
}

func stampNPMDomainMonitor(item *npmDomainMonitor, fallback string, touchModified bool) {
	stampCreatedModified(&item.CreatedOn, &item.ModifiedOn, fallback, touchModified)
	item.OwnerUserID = 1
	if item.Meta == nil {
		item.Meta = map[string]any{}
	}
	normalizeDomainMonitorIssuerMeta(item.Meta)
	item.DomainNames = domainMonitorDomains(item.DomainNames)
	item.Resolvers = uniqueSortedStrings(item.Resolvers)
	item.RegistrarProvider = effectiveDomainMonitorRegistrarProvider(*item)
	if strings.TrimSpace(item.CheckInterval) == "" {
		item.CheckInterval = "24h"
	}
	if item.ThresholdDays <= 0 {
		item.ThresholdDays = 30
	}
	item.Owner = localNPMUser
}

func stampNPMWakeDevice(item *npmWakeDevice, fallback string, touchModified bool) {
	stampCreatedModified(&item.CreatedOn, &item.ModifiedOn, fallback, touchModified)
	item.OwnerUserID = 1
	if item.Meta == nil {
		item.Meta = map[string]any{}
	}
	item.Name = strings.TrimSpace(item.Name)
	item.MACAddress = normalizeWakeMAC(item.MACAddress)
	item.BroadcastAddress = defaultString(strings.TrimSpace(item.BroadcastAddress), "255.255.255.255")
	if item.Port == 0 {
		item.Port = 9
	}
	item.SecureOn = normalizeWakeMAC(item.SecureOn)
	item.Host = strings.TrimSpace(item.Host)
	item.Description = strings.TrimSpace(item.Description)
	item.Owner = localNPMUser
}

func normalizeDomainMonitorIssuerMeta(meta map[string]any) {
	normalizeIssuerValue := func(container map[string]any, keys ...string) {
		for _, key := range keys {
			value, ok := container[key].(string)
			if !ok {
				continue
			}
			if label := readableCertificateIssuerLabel(value); label != value {
				container[key] = label
			}
		}
	}
	normalizeIssuerValue(meta, "ssl_issuer", "sslIssuer")
	switch results := meta["results"].(type) {
	case []any:
		for _, item := range results {
			if row, ok := item.(map[string]any); ok {
				normalizeIssuerValue(row, "ssl_issuer", "sslIssuer")
			}
		}
	case []domainMonitorResult:
		for i := range results {
			results[i].SSLIssuer = readableCertificateIssuerLabel(results[i].SSLIssuer)
		}
		meta["results"] = results
	}
}

func stampNPMAccessList(item *npmAccessList, fallback string, touchModified bool) {
	stampCreatedModified(&item.CreatedOn, &item.ModifiedOn, fallback, touchModified)
	item.OwnerUserID = 1
	if item.Meta == nil {
		item.Meta = map[string]any{}
	}
	if item.Items == nil {
		item.Items = []npmAccessListItem{}
	}
	if item.Clients == nil {
		item.Clients = []npmAccessClient{}
	}
	item.Owner = localNPMUser
	authItems := item.Items[:0]
	for i := range item.Items {
		item.Items[i].Username = strings.TrimSpace(item.Items[i].Username)
		if item.Items[i].Username == "" || strings.TrimSpace(item.Items[i].Password) == "" {
			continue
		}
		if hash, err := accessBasicAuthPasswordHash(item.Items[i].Password); err == nil {
			item.Items[i].Password = hash
		}
		item.Items[i].ID = len(authItems) + 1
		item.Items[i].AccessListID = item.ID
		authItems = append(authItems, item.Items[i])
	}
	item.Items = authItems
	clients := item.Clients[:0]
	for i := range item.Clients {
		item.Clients[i].Address = strings.TrimSpace(item.Clients[i].Address)
		item.Clients[i].Directive = strings.ToLower(strings.TrimSpace(item.Clients[i].Directive))
		if item.Clients[i].Address == "" {
			continue
		}
		item.Clients[i].ID = len(clients) + 1
		item.Clients[i].AccessListID = item.ID
		clients = append(clients, item.Clients[i])
	}
	item.Clients = clients
}

func siteAuditMeta(s Site) map[string]any {
	return map[string]any{
		"name":         s.Name,
		"domainNames":  splitDomains(s.Domain),
		"kind":         defaultString(s.Kind, "proxy"),
		"backend":      s.Backend,
		"redirect_url": s.RedirectURL,
		"accessListID": s.AccessListID,
	}
}

func auditTypeForJSONItem(item any) string {
	switch item.(type) {
	case npmStream:
		return "stream"
	case npmDynamicDNS:
		return "dynamic-dns"
	case npmDomainMonitor:
		return "domain-monitor"
	case npmWakeDevice:
		return "wake-device"
	case npmAccessList:
		return "access-list"
	default:
		return "system"
	}
}

func auditMetaForJSONItem(item any) map[string]any {
	switch v := item.(type) {
	case npmStream:
		return map[string]any{"incomingPort": v.IncomingPort, "forwardingHost": v.ForwardingHost, "forwardingPort": v.ForwardingPort}
	case npmDynamicDNS:
		return map[string]any{"name": v.Name, "domainNames": v.DomainNames, "credentialId": v.CredentialID}
	case npmDomainMonitor:
		return map[string]any{"name": v.Name, "domainNames": v.DomainNames, "status": metaString(v.Meta, "status")}
	case npmWakeDevice:
		return map[string]any{"name": v.Name, "macAddress": v.MACAddress, "host": v.Host}
	case npmAccessList:
		return map[string]any{"name": v.Name}
	default:
		return map[string]any{}
	}
}

func npmCertificateByID(id int) (npmCertificate, bool) {
	for _, c := range npmCertificates() {
		if c.ID == id {
			return c, true
		}
	}
	return npmCertificate{}, false
}

func loadCustomCertRecords() ([]customCertRecord, error) {
	items := []customCertRecord{}
	if err := loadJSONFile(customCertMeta, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func mustLoadCustomCertRecords() []customCertRecord {
	items, err := loadCustomCertRecords()
	if err != nil {
		return []customCertRecord{}
	}
	return items
}

func customCertToNPM(rec customCertRecord) npmCertificate {
	createdOn := strings.TrimSpace(rec.CreatedOn)
	if createdOn == "" {
		createdOn = timestampFromFile(customCertMeta)
	}
	modifiedOn := strings.TrimSpace(rec.ModifiedOn)
	if modifiedOn == "" {
		modifiedOn = createdOn
	}
	return npmCertificate{
		ID:          rec.ID,
		CreatedOn:   createdOn,
		ModifiedOn:  modifiedOn,
		OwnerUserID: 1,
		Provider:    "other",
		NiceName:    rec.Name,
		DomainNames: rec.DomainNames,
		ExpiresOn:   time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
		Meta:        rec.Meta,
		Owner:       localNPMUser,
	}
}

func issuedWildcardCoversDomain(domain string, sanMap map[string]certSAN) bool {
	domain = certDomainName(domain)
	if domain == "" || strings.HasPrefix(domain, "*.") {
		return false
	}
	for candidate := range sanMap {
		if strings.HasPrefix(candidate, "*.") && certCoversHost(candidate, domain) {
			return true
		}
	}
	return false
}

func certOverviewRows() []CertOverview {
	creds, _ := loadCredentials()
	sanMap := scanIssuedCerts()
	sites, _ := readAllSites()
	requestConfigs := loadCertificateRequestConfigs(creds)
	out := []CertOverview{}
	seen := map[string]int{}
	addRow := func(domain string, linkedSite string, signMethod string, credID string, issuerCfg IssuerConfig, lastErr string, lastErrAt time.Time, createdOn string, modifiedOn string) {
		lastErr, lastErrAt = visibleCertError(lastErr, lastErrAt)
		disabled := false
		if strings.HasPrefix(signMethod, "disabled:") {
			disabled = true
			signMethod = strings.TrimPrefix(signMethod, "disabled:")
		}
		domain = certDomainName(domain)
		if domain == "" {
			return
		}
		key := strings.ToLower(domain)
		info, issued := sanMap[key]
		if !issued {
			return
		}
		lastErr = ""
		lastErrAt = time.Time{}
		if idx, ok := seen[key]; ok {
			if disabled && len(out[idx].LinkedSites) == 0 {
				out[idx].Status = "disabled"
			}
			if linkedSite != "" {
				out[idx].LinkedSites = append(out[idx].LinkedSites, linkedSite)
			}
			if out[idx].CredentialID == "" && credID != "" {
				out[idx].CredentialID = credID
				out[idx].SignMethod = signMethod
				out[idx].IssuerConfig = issuerCfg
				if cred, ok := findCredential(credID, creds); ok {
					out[idx].CredentialName = cred.Name
				}
			}
			return
		}
		if issuerCfg.Provider == "" || issuerCfg.Provider == "auto" {
			issuerCfg.Provider = info.Provider
			if saved, ok := loadSavedIssuerConfig(issuerCfg.Provider); ok {
				issuerCfg = saved
			}
		}
		ov := CertOverview{Domain: domain, CreatedOn: createdOn, ModifiedOn: modifiedOn, IsWildcard: strings.HasPrefix(domain, "*."), Provider: defaultString(issuerCfg.Provider, "auto"), SignMethod: signMethod, CredentialID: credID, LastError: lastErr, LastErrorAt: lastErrAt, IssuerConfig: issuerCfg}
		if linkedSite != "" {
			ov.LinkedSites = []string{linkedSite}
		}
		if cred, ok := findCredential(credID, creds); ok {
			ov.CredentialName = cred.Name
		}
		ov.Issued = true
		ov.Issuer = info.Issuer
		ov.NotAfter = info.NotAfter
		if ov.CreatedOn == "" && !info.NotBefore.IsZero() {
			ov.CreatedOn = localTimestamp(info.NotBefore)
		}
		if ov.ModifiedOn == "" && !info.NotAfter.IsZero() {
			ov.ModifiedOn = localTimestamp(info.NotAfter)
		}
		ov.DaysLeft = int(time.Until(info.NotAfter).Hours() / 24)
		switch {
		case disabled:
			ov.Status = "disabled"
		case ov.DaysLeft <= 0:
			ov.Status = "expired"
		case ov.DaysLeft <= 30:
			ov.Status = "expiring"
		default:
			ov.Status = "issued"
		}
		seen[key] = len(out)
		out = append(out, ov)
	}
	addRequestRow := func(domain string, linkedSite string, signMethod string, credID string, issuerCfg IssuerConfig, disabled bool, lastErr string, lastErrAt time.Time, createdOn string, modifiedOn string) {
		lastErr, lastErrAt = visibleCertError(lastErr, lastErrAt)
		domain = certDomainName(domain)
		if domain == "" {
			return
		}
		key := strings.ToLower(domain)
		if _, ok := seen[key]; ok {
			return
		}
		status := "pending"
		if lastErr != "" {
			status = "failed"
		}
		ov := CertOverview{
			Domain:       domain,
			CreatedOn:    createdOn,
			ModifiedOn:   modifiedOn,
			IsWildcard:   strings.HasPrefix(domain, "*."),
			Status:       status,
			Provider:     defaultString(issuerCfg.Provider, "auto"),
			SignMethod:   signMethod,
			CredentialID: credID,
			LastError:    lastErr,
			LastErrorAt:  lastErrAt,
			IssuerConfig: issuerCfg,
		}
		if disabled {
			ov.Status = "disabled"
		}
		if linkedSite != "" {
			ov.LinkedSites = []string{linkedSite}
		}
		if cred, ok := findCredential(credID, creds); ok {
			ov.CredentialName = cred.Name
		}
		seen[key] = len(out)
		out = append(out, ov)
	}
	for _, s := range sites {
		if s.NoTLS {
			continue
		}
		if s.CustomCertFile != "" || s.CustomKeyFile != "" {
			continue
		}
		method := "HTTP-01"
		if s.ChallengePref == "dns" || s.Wildcard {
			method = "DNS-01"
		}
		for _, d := range certificateOverviewDomainsForSite(s) {
			rowDomain := d
			rowMethod := method
			rowCredentialID := s.CredentialID
			rowIssuer := s.Issuer
			rowLastError := s.LastError
			rowLastErrorAt := s.LastErrorAt
			if binding, ok := certificateBindingForDomain(s.CertificateBindings, d); ok {
				if binding.LastError != "" {
					rowLastError = binding.LastError
					rowLastErrorAt = binding.LastErrorAt
				}
				if binding.Mode == "selected" {
					if cfg, ok := selectedBindingCertificateConfigFromRows(binding, out, sanMap); ok {
						rowDomain = cfg.Domain
						rowMethod = "HTTP-01"
						if cfg.ChallengePref == "dns" {
							rowMethod = "DNS-01"
						}
						rowCredentialID = cfg.CredentialID
						rowIssuer = cfg.Issuer
					} else if cfg, ok := certificateConfigFromBinding(binding); ok {
						rowMethod = "HTTP-01"
						if cfg.ChallengePref == "dns" {
							rowMethod = "DNS-01"
						}
						rowCredentialID = cfg.CredentialID
						rowIssuer = cfg.Issuer
					} else if cfg, ok := requestConfigs[binding.CertificateID]; ok {
						rowMethod = defaultString(cfg.SignMethod, rowMethod)
						rowCredentialID = cfg.CredentialID
						rowIssuer = cfg.Issuer
					}
				} else if binding.Mode == "auto" {
					if cfg, ok := requestConfigs[stableID("cert:"+certDomainName(d))]; ok {
						rowMethod = defaultString(cfg.SignMethod, rowMethod)
						rowCredentialID = cfg.CredentialID
						rowIssuer = cfg.Issuer
					}
				}
			}
			if s.Disabled {
				rowMethod = "disabled:" + method
			}
			addRow(rowDomain, s.Name, rowMethod, rowCredentialID, rowIssuer, rowLastError, rowLastErrorAt, s.CreatedOn, s.ModifiedOn)
			addRequestRow(rowDomain, s.Name, rowMethod, rowCredentialID, rowIssuer, s.Disabled, rowLastError, rowLastErrorAt, s.CreatedOn, s.ModifiedOn)
		}
	}
	entries, err := os.ReadDir(sitesDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
				continue
			}
			baseName := strings.TrimSuffix(e.Name(), metaSuffix)
			if !strings.HasPrefix(baseName, managedCertPrefix) {
				continue
			}
			metaPath := filepath.Join(sitesDir, e.Name())
			data, err := os.ReadFile(metaPath)
			if err != nil {
				continue
			}
			var p placeholderMeta
			if json.Unmarshal(data, &p) != nil {
				continue
			}
			p.CreatedOn, p.ModifiedOn = normalizePersistentTimestamps(p.CreatedOn, p.ModifiedOn, metaPath)
			method := "HTTP-01"
			if p.CredentialID != "" || strings.HasPrefix(p.Domain, "*.") {
				method = "DNS-01"
			}
			addRow(p.Domain, "", method, p.CredentialID, p.Issuer, p.LastError, p.LastErrorAt, p.CreatedOn, p.ModifiedOn)
			addRequestRow(p.Domain, "", method, p.CredentialID, p.Issuer, p.Disabled, p.LastError, p.LastErrorAt, p.CreatedOn, p.ModifiedOn)
		}
	}
	for _, cred := range creds {
		if !strings.HasPrefix(cred.Name, "ACME ") {
			continue
		}
		domain := certDomainName(strings.TrimPrefix(cred.Name, "ACME "))
		if domain == "" || !strings.HasPrefix(domain, "*.") {
			continue
		}
		if _, ok := seen[strings.ToLower(domain)]; ok {
			continue
		}
		if _, issued := sanMap[strings.ToLower(domain)]; issued {
			continue
		}
		ov := CertOverview{
			Domain:         domain,
			IsWildcard:     strings.HasPrefix(domain, "*."),
			Status:         "pending",
			Provider:       "auto",
			SignMethod:     "DNS-01",
			CredentialID:   cred.ID,
			CredentialName: cred.Name,
		}
		seen[strings.ToLower(domain)] = len(out)
		out = append(out, ov)
	}
	for domain, info := range sanMap {
		if _, ok := seen[domain]; ok {
			continue
		}
		if issuedWildcardCoversDomain(domain, sanMap) {
			continue
		}
		ov := CertOverview{
			Domain:     domain,
			IsWildcard: strings.HasPrefix(domain, "*."),
			Status:     "issued",
			Issued:     true,
			Provider:   defaultString(info.Provider, "auto"),
			Issuer:     info.Issuer,
			NotAfter:   info.NotAfter,
			DaysLeft:   int(time.Until(info.NotAfter).Hours() / 24),
			SignMethod: "HTTP-01",
		}
		if !info.NotBefore.IsZero() {
			ov.CreatedOn = localTimestamp(info.NotBefore)
			ov.ModifiedOn = ov.CreatedOn
		}
		switch {
		case ov.DaysLeft <= 0:
			ov.Status = "expired"
		case ov.DaysLeft <= 30:
			ov.Status = "expiring"
		}
		seen[domain] = len(out)
		out = append(out, ov)
	}
	return out
}

func certOverviewToNPM(c CertOverview) npmCertificate {
	domain := certDomainName(c.Domain)
	expires := c.NotAfter
	if expires.IsZero() {
		expires = time.Now().Add(90 * 24 * time.Hour)
	}
	createdOn := strings.TrimSpace(c.CreatedOn)
	if createdOn == "" {
		if !c.NotAfter.IsZero() {
			createdOn = localTimestamp(c.NotAfter)
		} else {
			createdOn = seedTimestamp()
		}
	}
	modifiedOn := strings.TrimSpace(c.ModifiedOn)
	if modifiedOn == "" {
		modifiedOn = createdOn
	}
	if c.Status != "failed" {
		c.LastError = ""
		c.LastErrorAt = time.Time{}
	}
	meta := map[string]any{"status": c.Status, "issuer": c.Issuer, "sign_method": c.SignMethod, "credential_id": c.CredentialID, "credential_name": c.CredentialName, "last_error": c.LastError, "last_error_at": c.LastErrorAt}
	meta = mergeMeta(meta, issuerMeta(c.IssuerConfig))
	return npmCertificate{
		ID:          stableID("cert:" + domain),
		CreatedOn:   createdOn,
		ModifiedOn:  modifiedOn,
		OwnerUserID: 1,
		Provider:    defaultString(c.Provider, "auto"),
		NiceName:    domain,
		DomainNames: []string{domain},
		ExpiresOn:   localTimestamp(expires),
		Meta:        meta,
		Owner:       localNPMUser,
	}
}

func loadCertificateRequestConfigs(creds []Credential) map[int]certRequestConfig {
	out := map[int]certRequestConfig{}
	sites, _ := readAllSites()
	for _, s := range sites {
		if s.NoTLS || s.CustomCertFile != "" || s.CustomKeyFile != "" {
			continue
		}
		method := "HTTP-01"
		if s.ChallengePref == "dns" || s.Wildcard {
			method = "DNS-01"
		}
		for _, domain := range certificateOverviewDomainsForSite(s) {
			domain = certDomainName(domain)
			if domain == "" {
				continue
			}
			issuer := s.Issuer
			credID := s.CredentialID
			rowMethod := method
			if binding, ok := certificateBindingForDomain(s.CertificateBindings, domain); ok {
				if cfg, ok := certificateConfigFromBinding(binding); ok {
					issuer = cfg.Issuer
					credID = cfg.CredentialID
					rowMethod = "HTTP-01"
					if cfg.ChallengePref == "dns" {
						rowMethod = "DNS-01"
					}
				} else if inferred, ok := inferRequestConfigFromBinding(s, binding); ok {
					issuer = inferred.Issuer
					credID = inferred.CredentialID
					rowMethod = inferred.SignMethod
				}
			}
			provider := issuer.Provider
			if provider == "" {
				provider = "auto"
			}
			out[stableID("cert:"+domain)] = certRequestConfig{
				Provider:     provider,
				SignMethod:   rowMethod,
				CredentialID: credID,
				Issuer:       issuer,
			}
		}
	}
	for _, cred := range creds {
		if !strings.HasPrefix(cred.Name, "ACME ") {
			continue
		}
		domain := certDomainName(strings.TrimPrefix(cred.Name, "ACME "))
		if domain == "" {
			continue
		}
		issuer := cred.Issuer
		provider := issuer.Provider
		if provider == "" {
			provider = "auto"
		}
		out[stableID("cert:"+domain)] = certRequestConfig{
			Provider:     provider,
			SignMethod:   "DNS-01",
			CredentialID: cred.ID,
			Issuer:       issuer,
		}
	}
	return out
}

func inferRequestConfigFromBinding(s Site, binding CertificateBinding) (certRequestConfig, bool) {
	issuer, ok := inferIssuerFromText(binding.LastError)
	if !ok {
		return certRequestConfig{}, false
	}
	method := "HTTP-01"
	if strings.Contains(binding.LastError, "_acme-challenge") {
		method = "DNS-01"
	}
	credID := firstNonEmpty(binding.CredentialID, s.CredentialID, firstBindingCredentialID(s.CertificateBindings))
	return certRequestConfig{
		Provider:     issuer.Provider,
		SignMethod:   method,
		CredentialID: credID,
		Issuer:       issuer,
	}, true
}

func firstBindingCredentialID(bindings []CertificateBinding) string {
	for _, binding := range bindings {
		if strings.TrimSpace(binding.CredentialID) != "" {
			return binding.CredentialID
		}
	}
	return ""
}

func inferIssuerFromText(text string) (IssuerConfig, bool) {
	switch {
	case strings.Contains(text, "dv.acme-v02.api.pki.goog"):
		if cfg, ok := loadSavedIssuerConfig("google"); ok {
			return cfg, true
		}
		return IssuerConfig{Provider: "google", CADirectory: certificateAuthorities["google"].CADirectory}, true
	case strings.Contains(text, "acme.zerossl.com"):
		if cfg, ok := loadSavedIssuerConfig("zerossl"); ok {
			return cfg, true
		}
		return IssuerConfig{Provider: "zerossl", CADirectory: certificateAuthorities["zerossl"].CADirectory}, true
	case strings.Contains(text, "acme-staging-v02.api.letsencrypt.org"):
		return IssuerConfig{Provider: "letsencrypt-staging", CADirectory: certificateAuthorities["letsencrypt-staging"].CADirectory}, true
	case strings.Contains(text, "acme-v02.api.letsencrypt.org"):
		return IssuerConfig{Provider: "letsencrypt", CADirectory: certificateAuthorities["letsencrypt"].CADirectory}, true
	default:
		return IssuerConfig{}, false
	}
}

func splitBackend(backend string) (string, string, int) {
	scheme := "http"
	rest := strings.TrimSpace(backend)
	if strings.Contains(rest, "://") {
		parts := strings.SplitN(rest, "://", 2)
		scheme = parts[0]
		rest = parts[1]
	}
	// 先剥离路径部分，只保留 host[:port]
	if slash := strings.Index(rest, "/"); slash != -1 {
		rest = rest[:slash]
	}
	host, portText, err := net.SplitHostPort(rest)
	if err == nil {
		port, _ := strconv.Atoi(portText)
		return scheme, host, port
	}
	if strings.Contains(rest, ":") {
		parts := strings.Split(rest, ":")
		port, _ := strconv.Atoi(parts[len(parts)-1])
		return scheme, strings.Join(parts[:len(parts)-1], ":"), port
	}
	if scheme == "https" {
		return scheme, rest, 443
	}
	return scheme, rest, 80
}

func firstDomain(domains []string) string {
	if len(domains) == 0 {
		return "site"
	}
	return certDomainName(domains[0])
}

func uniqueSiteName(domain string) string {
	name := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9._-]+`).ReplaceAllString(domain, "-"))
	name = strings.Trim(name, "-.")
	if name == "" {
		name = "site"
	}
	if len(name) > 48 {
		name = name[:48]
	}
	return name
}

func availableSiteName(base string) string {
	base = uniqueSiteName(base)
	for i := 0; i < 1000; i++ {
		name := base
		if i > 0 {
			suffix := fmt.Sprintf("-%d", i+1)
			cutoff := 64 - len(suffix)
			if cutoff > 0 && len(name) > cutoff {
				name = name[:cutoff]
			}
			name += suffix
		}
		if _, err := os.Stat(filepath.Join(sitesDir, name+metaSuffix)); os.IsNotExist(err) {
			return name
		}
	}
	return fmt.Sprintf("%s-%d", base, time.Now().Unix())
}

func stableID(s string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return int(h.Sum32() & 0x7fffffff)
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func defaultInt(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	n := 200
	if s := r.URL.Query().Get("n"); s != "" {
		fmt.Sscanf(s, "%d", &n)
	}
	data, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []string{})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		lines[i] = fmt.Sprintf("[%s] %s", logLineTimestamp(line), line)
	}
	writeJSON(w, http.StatusOK, lines)
}

func logLineTimestamp(line string) string {
	var ev struct {
		Ts any `json:"ts"`
	}
	if json.Unmarshal([]byte(line), &ev) == nil {
		switch ts := ev.Ts.(type) {
		case float64:
			sec, frac := math.Modf(ts)
			return localTimestamp(time.Unix(int64(sec), int64(frac*1e9)))
		case string:
			if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				return localTimestamp(t)
			}
		}
	}
	return localTimestamp(time.Now())
}

// ============================================================
// 监听 caddy net log writer 推过来的 TLS 事件，持久化错误到 site JSON
// ============================================================

// listenCaddyLogs 在 127.0.0.1:9002 起 TCP server，接 caddy 的 net log writer。
// Caddyfile 全局块配置 `log cert_events { include tls.obtain http.acme_client; output net 127.0.0.1:9002 { soft_start } }`
// caddy 主动 dial 过来按 JSON Lines 推送，断了会重连（soft_start）。
// 我们按行读 JSON，过滤 cert 事件，写 last_error/last_error_at。
func listenCaddyLogs() {
	ln, err := net.Listen("tcp", certLogListen)
	if err != nil {
		log.Printf("cert log listener: %v", err)
		return
	}
	log.Printf("cert log listener on %s", certLogListen)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("cert log accept: %v", err)
			time.Sleep(time.Second)
			continue
		}
		go readCertLog(conn)
	}
}

func readCertLog(conn net.Conn) {
	defer conn.Close()
	log.Printf("cert log conn opened from %s", conn.RemoteAddr())
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		handleCertLogLine(scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("cert log conn closed with err: %v", err)
	} else {
		log.Printf("cert log conn closed")
	}
}

// handleCertLogLine 解析一行 JSON 日志，按 logger + msg 提取事件。
//
// 关注的事件形态：
//   - tls.obtain msg=certificate obtained successfully     → 清错误
//   - tls.obtain level=error                               → 写错误（msg/error 拼）
//   - http.acme_client msg=challenge failed level=error    → 写错误（problem.detail 优先）
//   - http.acme_client msg=validating authorization        → 写错误（problem.detail）
func handleCertLogLine(line []byte) {
	var ev struct {
		Level      string                 `json:"level"`
		Logger     string                 `json:"logger"`
		Msg        string                 `json:"msg"`
		Identifier string                 `json:"identifier"`
		Error      string                 `json:"error"`
		Problem    map[string]interface{} `json:"problem"`
	}
	if err := json.Unmarshal(line, &ev); err != nil {
		return
	}
	if ev.Identifier == "" {
		return
	}

	// 成功事件
	if ev.Logger == "tls.obtain" && strings.Contains(ev.Msg, "certificate obtained successfully") {
		if err := clearCertError(ev.Identifier); err != nil {
			log.Printf("clear cert error for %s: %v", ev.Identifier, err)
		} else {
			log.Printf("cert obtained: %s", ev.Identifier)
			notifyCertificateRenewed(ev.Identifier)
		}
		return
	}

	// 失败事件
	if ev.Level != "error" {
		return
	}
	var msg string
	switch ev.Logger {
	case "http.acme_client":
		if ev.Problem != nil {
			if detail, ok := ev.Problem["detail"].(string); ok && detail != "" {
				msg = detail
			}
		}
		if msg == "" {
			msg = ev.Error
		}
		if msg == "" {
			msg = ev.Msg
		}
	case "tls.obtain":
		msg = ev.Error
		if msg == "" {
			msg = ev.Msg
		}
	default:
		return
	}
	if msg == "" {
		return
	}
	if isIgnorableCertError(msg) {
		log.Printf("ignore cert cleanup noise for %s: %s", ev.Identifier, msg)
		return
	}
	notifyCertificateRenewFailed(ev.Identifier, msg)
	if err := persistCertError(ev.Identifier, msg, time.Now()); err != nil {
		log.Printf("persist cert error for %s: %v", ev.Identifier, err)
	} else {
		log.Printf("cert error: %s -> %s", ev.Identifier, msg)
	}
}

func isIgnorableCertError(msg string) bool {
	msg = strings.TrimSpace(msg)
	return strings.HasPrefix(msg, "remove ") &&
		strings.Contains(msg, "/locks/issue_cert_") &&
		strings.HasSuffix(msg, ": no such file or directory")
}

func visibleCertError(msg string, at time.Time) (string, time.Time) {
	if isIgnorableCertError(msg) {
		return "", time.Time{}
	}
	return msg, at
}

// persistCertError 把错误写到匹配 identifier 的 site 或 placeholder JSON。
// identifier 可能是精确域名（a.b.com）也可能是通配符（*.b.com）。
func persistCertError(identifier, msg string, at time.Time) error {
	return updateCertMetaForIdentifier(identifier, func(s *Site) bool {
		if setBindingCertError(s, identifier, msg, at) {
			if s.LastError != "" || !s.LastErrorAt.IsZero() {
				s.LastError = ""
				s.LastErrorAt = time.Time{}
			}
			return true
		}
		s.LastError = msg
		s.LastErrorAt = at
		return true
	}, func(p *placeholderMeta) bool {
		p.LastError = msg
		p.LastErrorAt = at
		return true
	})
}

func clearCertError(identifier string) error {
	return updateCertMetaForIdentifier(identifier, func(s *Site) bool {
		if clearBindingCertError(s, identifier) {
			if s.LastError != "" || !s.LastErrorAt.IsZero() {
				s.LastError = ""
				s.LastErrorAt = time.Time{}
			}
			return true
		}
		if s.LastError == "" && s.LastErrorAt.IsZero() {
			return false
		}
		s.LastError = ""
		s.LastErrorAt = time.Time{}
		return true
	}, func(p *placeholderMeta) bool {
		if p.LastError == "" && p.LastErrorAt.IsZero() {
			return false
		}
		p.LastError = ""
		p.LastErrorAt = time.Time{}
		return true
	})
}

func setBindingCertError(s *Site, identifier string, msg string, at time.Time) bool {
	key := certDomainKey(identifier)
	changed := false
	for i := range s.CertificateBindings {
		if certDomainKey(s.CertificateBindings[i].Domain) != key {
			continue
		}
		s.CertificateBindings[i].LastError = msg
		s.CertificateBindings[i].LastErrorAt = at
		changed = true
	}
	return changed
}

func clearBindingCertError(s *Site, identifier string) bool {
	key := certDomainKey(identifier)
	changed := false
	for i := range s.CertificateBindings {
		if certDomainKey(s.CertificateBindings[i].Domain) != key {
			continue
		}
		if s.CertificateBindings[i].LastError == "" && s.CertificateBindings[i].LastErrorAt.IsZero() {
			continue
		}
		s.CertificateBindings[i].LastError = ""
		s.CertificateBindings[i].LastErrorAt = time.Time{}
		changed = true
	}
	return changed
}

func updateCertMetaForIdentifier(identifier string, onSite func(*Site) bool, onPlaceholder func(*placeholderMeta) bool) error {
	id := strings.ToLower(strings.TrimSpace(identifier))
	if id == "" {
		return nil
	}
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), metaSuffix) {
			continue
		}
		path := filepath.Join(sitesDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), metaSuffix)
		if strings.HasPrefix(baseName, managedCertPrefix) {
			var p placeholderMeta
			if json.Unmarshal(data, &p) != nil {
				continue
			}
			if strings.ToLower(p.Domain) != id {
				continue
			}
			if !onPlaceholder(&p) {
				continue
			}
			out, _ := json.MarshalIndent(p, "", "  ")
			if err := os.WriteFile(path, out, 0644); err != nil {
				return err
			}
			continue
		}
		var s Site
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		matched := false
		for _, d := range splitDomains(s.Domain) {
			if certDomainKey(d) == id {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if !onSite(&s) {
			continue
		}
		out, _ := json.MarshalIndent(s, "", "  ")
		if err := os.WriteFile(path, out, 0644); err != nil {
			return err
		}
	}
	return nil
}

// ============================================================
// 动态 DNS 日志监听
// ============================================================

// listenDDNSLogs 在 127.0.0.1:9003 起 TCP server，接 caddy 的 net log writer。
// Caddyfile 全局块配置 `log ddns_events { include dynamic_dns; output net 127.0.0.1:9003 { soft_start } }`
// caddy 主动 dial 过来按 JSON Lines 推送，断了会重连（soft_start）。
func listenDDNSLogs() {
	ln, err := net.Listen("tcp", ddnsLogListen)
	if err != nil {
		log.Printf("ddns log listener: %v", err)
		return
	}
	log.Printf("ddns log listener on %s", ddnsLogListen)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("ddns log accept: %v", err)
			time.Sleep(time.Second)
			continue
		}
		go readDDNSLog(conn)
	}
}

func readDDNSLog(conn net.Conn) {
	defer conn.Close()
	log.Printf("ddns log conn opened from %s", conn.RemoteAddr())
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		handleDDNSLogLine(scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("ddns log conn closed with err: %v", err)
	} else {
		log.Printf("ddns log conn closed")
	}
}

func runDynamicDNSStatusLoop() {
	time.Sleep(30 * time.Second)
	if err := refreshDynamicDNSStatuses(); err != nil {
		log.Printf("dynamic dns status: %v", err)
	}
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if err := refreshDynamicDNSStatuses(); err != nil {
			log.Printf("dynamic dns status: %v", err)
		}
	}
}

// handleDDNSLogLine 解析一行 JSON 日志，按 logger + msg 提取动态 DNS 事件。
//
// 关注的事件形态：
//   - dynamic_dns msg="finished updating DNS" → 清错误（成功更新）
//   - dynamic_dns msg="no IP address change; no update needed" → 清错误（检查正常）
//   - dynamic_dns level=error → 写错误（DNS 查询失败、更新失败等）
func handleDDNSLogLine(line []byte) {
	var ev struct {
		Level  string `json:"level"`
		Logger string `json:"logger"`
		Msg    string `json:"msg"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(line, &ev); err != nil {
		return
	}
	if ev.Logger != "dynamic_dns" {
		return
	}

	// 成功事件：IP 更新成功，或检查成功但 IP 没变化。
	if ev.Level != "error" && ddnsSuccessMessage(ev.Msg) {
		if err := refreshDynamicDNSStatuses(); err != nil {
			log.Printf("refresh ddns status: %v", err)
		} else {
			log.Printf("ddns status refreshed")
		}
		return
	}

	// 失败事件
	if ev.Level != "error" {
		return
	}
	msg := ev.Error
	if msg == "" {
		msg = ev.Msg
	}
	if msg == "" {
		return
	}

	// caddy-dynamicdns 的错误日志格式为 "domain X not found" 等，
	// 域名嵌入在 error 字段中，尝试从中提取域名
	domain := extractDDNSDomain(msg)
	if domain != "" {
		if err := persistDDNSError(domain, msg, time.Now()); err != nil {
			log.Printf("persist ddns error for %s: %v", domain, err)
		} else {
			log.Printf("ddns error: %s -> %s", domain, msg)
		}
	} else {
		// 无具体域名 → 写到所有启用的 DDNS 条目
		if err := persistDDNSErrorAll(msg, time.Now()); err != nil {
			log.Printf("persist ddns error (all): %v", err)
		}
	}
}

func ddnsSuccessMessage(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(msg, "finished updating dns") ||
		strings.Contains(msg, "no ip address change") ||
		strings.Contains(msg, "updated ip address")
}

func refreshDynamicDNSStatuses() error {
	items, err := loadNPMDynamicDNS()
	if err != nil {
		return err
	}
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	changed := false
	now := time.Now()
	cache := map[string][]string{}
	for i := range items {
		if items[i].Meta == nil {
			items[i].Meta = map[string]any{}
		}
		if !items[i].Enabled {
			if dynamicDNSClearStatusError(&items[i]) {
				changed = true
			}
			continue
		}
		errText, meta := checkDynamicDNSItemStatus(items[i], creds, cache)
		for key, value := range meta {
			items[i].Meta[key] = value
		}
		if errText == "" {
			if dynamicDNSClearStatusError(&items[i]) {
				changed = true
			}
			items[i].Meta["last_checked"] = now.Format(time.RFC3339)
			changed = true
			continue
		}
		if metaString(items[i].Meta, "last_error", "lastError") != errText {
			changed = true
		}
		items[i].Meta["last_error"] = errText
		items[i].Meta["last_error_at"] = now.Format(time.RFC3339)
		items[i].Meta["last_checked"] = now.Format(time.RFC3339)
		changed = true
	}
	if !changed {
		return nil
	}
	return saveJSONFile(dynamicDNSPath, items)
}

func dynamicDNSClearStatusError(item *npmDynamicDNS) bool {
	if item.Meta == nil {
		return false
	}
	changed := false
	for _, key := range []string{"last_error", "lastError", "last_error_at", "lastErrorAt"} {
		if _, ok := item.Meta[key]; ok {
			delete(item.Meta, key)
			changed = true
		}
	}
	return changed
}

func checkDynamicDNSItemStatus(item npmDynamicDNS, creds []Credential, publicIPCache map[string][]string) (string, map[string]any) {
	meta := map[string]any{}
	wantIPv4 := item.IPv4
	wantIPv6 := item.IPv6
	if !wantIPv4 && !wantIPv6 {
		wantIPv4 = true
	}
	publicIPs, err := currentPublicIPs(wantIPv4, wantIPv6, item.IPServiceURL, publicIPCache)
	if err != nil {
		return "获取当前公网 IP 失败: " + err.Error(), meta
	}
	meta["current_ips"] = publicIPs
	meta["dns_provider"] = effectiveDynamicDNSProvider(item, creds)
	missingMessages := []string{}
	resolvedAll := []string{}
	cred, hasCred := findCredential(item.CredentialID, creds)
	for _, configured := range dynamicDNSDomains(item.DomainNames) {
		if hasCred && cred.Provider == "alidns" {
			targets, err := dynamicDNSRecordTargets(configured)
			if err != nil {
				missingMessages = append(missingMessages, fmt.Sprintf("%s Aliyun 记录目标解析失败: %v", configured, err))
				continue
			}
			for _, target := range targets {
				remoteIPs, err := syncAliyunDynamicDNSRecord(target, cred, publicIPs, wantIPv4, wantIPv6)
				if err != nil {
					missingMessages = append(missingMessages, fmt.Sprintf("%s Aliyun 记录检查失败: %v", target.Host, err))
					continue
				}
				resolvedAll = append(resolvedAll, remoteIPs...)
				if len(remoteIPs) == 0 {
					missingMessages = append(missingMessages, fmt.Sprintf("%s Aliyun 未找到匹配的 A/AAAA 记录", target.Host))
					continue
				}
				if !stringSetsEqual(remoteIPs, publicIPs) {
					missingMessages = append(missingMessages, fmt.Sprintf("%s 远端记录为 %s，当前公网 IP 为 %s", target.Host, strings.Join(remoteIPs, ", "), strings.Join(publicIPs, ", ")))
				}
			}
			continue
		}
		for _, host := range dynamicDNSHostnames(configured) {
			if hasCred && cred.Provider == "cloudflare" {
				remoteIPs, err := syncCloudflareDynamicDNSRecord(host, cred, publicIPs, wantIPv4, wantIPv6)
				if err != nil {
					missingMessages = append(missingMessages, fmt.Sprintf("%s Cloudflare 记录检查失败: %v", host, err))
					continue
				}
				resolvedAll = append(resolvedAll, remoteIPs...)
				if len(remoteIPs) == 0 {
					missingMessages = append(missingMessages, fmt.Sprintf("%s Cloudflare 未找到匹配的 A/AAAA 记录", host))
					continue
				}
				continue
			}
			remoteIPs, err := resolveDomainIPsByVersion(host, item.Resolvers, wantIPv4, wantIPv6)
			if err != nil {
				missingMessages = append(missingMessages, fmt.Sprintf("%s 解析失败: %v", host, err))
				continue
			}
			resolvedAll = append(resolvedAll, remoteIPs...)
			if len(remoteIPs) == 0 {
				missingMessages = append(missingMessages, fmt.Sprintf("%s 没有匹配的 A/AAAA 记录", host))
				continue
			}
			if !stringSetsEqual(remoteIPs, publicIPs) {
				missingMessages = append(missingMessages, fmt.Sprintf("%s 远端记录为 %s，当前公网 IP 为 %s", host, strings.Join(remoteIPs, ", "), strings.Join(publicIPs, ", ")))
			}
		}
	}
	meta["resolved_ips"] = uniqueSortedStrings(resolvedAll)
	if len(missingMessages) > 0 {
		return strings.Join(missingMessages, "; "), meta
	}
	return "", meta
}

func syncCloudflareDynamicDNSRecord(host string, cred Credential, publicIPs []string, wantIPv4 bool, wantIPv6 bool) ([]string, error) {
	records, err := cloudflareDNSRecordsForHost(host, cred)
	if err != nil {
		return nil, err
	}
	remoteIPs := []string{}
	for _, record := range records {
		if !dynamicDNSRecordTypeWanted(record.Type, wantIPv4, wantIPv6) {
			continue
		}
		if ip := net.ParseIP(record.Content); ip != nil {
			remoteIPs = append(remoteIPs, ip.String())
		}
		desired := dynamicDNSDesiredIPForRecordType(record.Type, publicIPs)
		if desired == "" || dynamicDNSRecordContentMatches(record.Content, desired) {
			continue
		}
		if err := updateCloudflareDNSRecordContent(record, cred, desired); err != nil {
			return uniqueSortedStrings(remoteIPs), err
		}
		remoteIPs = replaceIPValue(remoteIPs, record.Content, desired)
	}
	return uniqueSortedStrings(remoteIPs), nil
}

func dynamicDNSRecordContentMatches(current string, desired string) bool {
	currentIP := net.ParseIP(strings.TrimSpace(current))
	desiredIP := net.ParseIP(strings.TrimSpace(desired))
	if currentIP != nil && desiredIP != nil {
		return currentIP.String() == desiredIP.String()
	}
	return strings.TrimSpace(current) == strings.TrimSpace(desired)
}

func dynamicDNSRecordTypeWanted(recordType string, wantIPv4 bool, wantIPv6 bool) bool {
	switch strings.ToUpper(recordType) {
	case "A":
		return wantIPv4
	case "AAAA":
		return wantIPv6
	default:
		return false
	}
}

func dynamicDNSDesiredIPForRecordType(recordType string, publicIPs []string) string {
	for _, value := range publicIPs {
		ip := net.ParseIP(value)
		if ip == nil {
			continue
		}
		if strings.EqualFold(recordType, "A") && ip.To4() != nil {
			return ip.String()
		}
		if strings.EqualFold(recordType, "AAAA") && ip.To4() == nil {
			return ip.String()
		}
	}
	return ""
}

func replaceIPValue(values []string, oldValue string, newValue string) []string {
	oldIP := net.ParseIP(oldValue)
	if oldIP == nil {
		oldValue = strings.TrimSpace(oldValue)
	} else {
		oldValue = oldIP.String()
	}
	newIP := net.ParseIP(newValue)
	if newIP != nil {
		newValue = newIP.String()
	}
	out := []string{}
	replaced := false
	for _, value := range values {
		if value == oldValue && !replaced {
			out = append(out, newValue)
			replaced = true
			continue
		}
		out = append(out, value)
	}
	if !replaced {
		out = append(out, newValue)
	}
	return uniqueSortedStrings(out)
}

func syncAliyunDynamicDNSRecord(target dynamicDNSRecordTarget, cred Credential, publicIPs []string, wantIPv4 bool, wantIPv6 bool) ([]string, error) {
	records, err := aliyunDNSRecordsForTarget(target, cred)
	if err != nil {
		return nil, err
	}
	remoteIPs := []string{}
	for _, record := range records {
		if !dynamicDNSRecordTypeWanted(record.Type, wantIPv4, wantIPv6) {
			continue
		}
		if ip := net.ParseIP(record.Value); ip != nil {
			remoteIPs = append(remoteIPs, ip.String())
		}
		desired := dynamicDNSDesiredIPForRecordType(record.Type, publicIPs)
		if desired == "" || dynamicDNSRecordContentMatches(record.Value, desired) {
			continue
		}
		if err := updateAliyunDNSRecordContent(record, target, cred, desired); err != nil {
			return uniqueSortedStrings(remoteIPs), err
		}
		remoteIPs = replaceIPValue(remoteIPs, record.Value, desired)
	}
	return uniqueSortedStrings(remoteIPs), nil
}

func aliyunDNSRecordsForTarget(target dynamicDNSRecordTarget, cred Credential) ([]aliyunDNSRecord, error) {
	if strings.TrimSpace(cred.AliyunKey) == "" || strings.TrimSpace(cred.AliyunSecret) == "" {
		return nil, errors.New("阿里云 AccessKey ID 或 Secret 为空")
	}
	target.Zone = normalizedDomainMonitorHost(target.Zone)
	target.RR = normalizeDynamicDNSRR(target.RR)
	if target.Zone == "" || target.RR == "" {
		return nil, errors.New("阿里云 DNS 记录目标为空")
	}
	records, err := aliyunDescribeDomainRecords(target.Zone, cred)
	if err != nil {
		return nil, err
	}
	out := []aliyunDNSRecord{}
	for _, record := range records {
		if normalizeDynamicDNSRR(record.RR) != target.RR {
			continue
		}
		if !strings.EqualFold(record.DomainName, target.Zone) && strings.TrimSpace(record.DomainName) != "" {
			continue
		}
		if !dynamicDNSRecordTypeWanted(record.Type, true, true) {
			continue
		}
		out = append(out, record)
	}
	return out, nil
}

func aliyunDescribeDomainRecords(zone string, cred Credential) ([]aliyunDNSRecord, error) {
	out := []aliyunDNSRecord{}
	const pageSize = 500
	for page := 1; ; page++ {
		params := map[string]string{
			"DomainName": zone,
			"PageNumber": strconv.Itoa(page),
			"PageSize":   strconv.Itoa(pageSize),
		}
		var resp aliyunDNSRecordsResponse
		if err := aliyunDNSAPIRequest("DescribeDomainRecords", params, cred, &resp); err != nil {
			return nil, err
		}
		out = append(out, resp.DomainRecord.Records...)
		if resp.TotalCount <= page*pageSize || len(resp.DomainRecord.Records) == 0 {
			break
		}
	}
	return out, nil
}

func updateAliyunDNSRecordContent(record aliyunDNSRecord, target dynamicDNSRecordTarget, cred Credential, content string) error {
	if strings.TrimSpace(record.RecordID) == "" {
		return errors.New("阿里云 DNS 记录缺少 RecordId")
	}
	rr := normalizeDynamicDNSRR(record.RR)
	if rr == "" {
		rr = normalizeDynamicDNSRR(target.RR)
	}
	recordType := strings.ToUpper(strings.TrimSpace(record.Type))
	if recordType == "" {
		return errors.New("阿里云 DNS 记录缺少 Type")
	}
	params := map[string]string{
		"RecordId": record.RecordID,
		"RR":       rr,
		"Type":     recordType,
		"Value":    content,
	}
	if record.TTL > 0 {
		params["TTL"] = strconv.FormatInt(record.TTL, 10)
	}
	if strings.TrimSpace(record.Line) != "" {
		params["Line"] = strings.TrimSpace(record.Line)
	}
	if record.Priority > 0 {
		params["Priority"] = strconv.FormatInt(record.Priority, 10)
	}
	var resp aliyunDNSRecordResponse
	return aliyunDNSAPIRequest("UpdateDomainRecord", params, cred, &resp)
}

func aliyunDNSAPIRequest(action string, params map[string]string, cred Credential, target any) error {
	reqURL, err := aliyunSignedRequestURL(action, params, cred)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if msg := aliyunAPIErrorMessage(data); msg != "" {
			return fmt.Errorf("阿里云 DNS API 返回 %s: %s", res.Status, msg)
		}
		return fmt.Errorf("阿里云 DNS API 返回 %s: %s", res.Status, strings.TrimSpace(string(data)))
	}
	if target == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	if msg := aliyunAPIErrorMessage(data); msg != "" {
		return errors.New("阿里云 DNS API 返回失败: " + msg)
	}
	return nil
}

func aliyunSignedRequestURL(action string, params map[string]string, cred Credential) (string, error) {
	reqURL, err := url.Parse(strings.TrimRight(aliyunDNSAPIBaseURL, "/") + "/")
	if err != nil {
		return "", err
	}
	signed := map[string]string{}
	for key, value := range params {
		signed[key] = value
	}
	signed["Action"] = action
	signed["Version"] = "2015-01-09"
	signed["Format"] = "JSON"
	signed["AccessKeyId"] = strings.TrimSpace(cred.AliyunKey)
	signed["SignatureMethod"] = "HMAC-SHA1"
	signed["SignatureVersion"] = "1.0"
	signed["SignatureNonce"] = aliyunSignatureNonce()
	signed["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	canonical := aliyunCanonicalQuery(signed)
	stringToSign := "GET&%2F&" + aliyunPercentEncode(canonical)
	mac := hmac.New(sha1.New, []byte(cred.AliyunSecret+"&"))
	_, _ = mac.Write([]byte(stringToSign))
	signed["Signature"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	reqURL.RawQuery = aliyunCanonicalQuery(signed)
	return reqURL.String(), nil
}

func aliyunSignatureNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func aliyunCanonicalQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, aliyunPercentEncode(key)+"="+aliyunPercentEncode(params[key]))
	}
	return strings.Join(parts, "&")
}

func aliyunPercentEncode(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

func aliyunAPIErrorMessage(data []byte) string {
	var resp struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return ""
	}
	if strings.TrimSpace(resp.Code) == "" {
		return ""
	}
	if strings.TrimSpace(resp.Message) == "" {
		return resp.Code
	}
	return resp.Code + ": " + resp.Message
}

func cloudflareDNSRecordsForHost(host string, cred Credential) ([]cloudflareDNSRecord, error) {
	if strings.TrimSpace(cred.CFToken) == "" {
		return nil, errors.New("Cloudflare API Token 为空")
	}
	host = normalizedDomainMonitorHost(host)
	if host == "" {
		return nil, errors.New("域名为空")
	}
	labels := strings.Split(host, ".")
	for i := 0; i <= len(labels)-2; i++ {
		zone := strings.Join(labels[i:], ".")
		records, err := cloudflareListDNSRecords(zone, host, cred.CFToken)
		if err != nil {
			if errors.Is(err, errCloudflareZoneNotFound) {
				continue
			}
			return nil, err
		}
		return records, nil
	}
	return nil, fmt.Errorf("Cloudflare 未找到 %s 所属 Zone", host)
}

var errCloudflareZoneNotFound = errors.New("cloudflare zone not found")

func cloudflareListDNSRecords(zone string, host string, token string) ([]cloudflareDNSRecord, error) {
	reqURL, err := url.Parse(strings.TrimRight(cloudflareAPIBaseURL, "/") + "/zones")
	if err != nil {
		return nil, err
	}
	q := reqURL.Query()
	q.Set("name", zone)
	q.Set("per_page", "1")
	reqURL.RawQuery = q.Encode()
	var zones cloudflareListResponse
	if err := cloudflareAPIRequest(http.MethodGet, reqURL.String(), token, nil, &zones); err != nil {
		return nil, err
	}
	if len(zones.Result) == 0 {
		return nil, errCloudflareZoneNotFound
	}
	zoneID := zones.Result[0].ID
	if zoneID == "" {
		return nil, fmt.Errorf("Cloudflare Zone %s 未返回 ID", zone)
	}
	out := []cloudflareDNSRecord{}
	for _, recordType := range []string{"A", "AAAA"} {
		recordURL, err := url.Parse(strings.TrimRight(cloudflareAPIBaseURL, "/") + "/zones/" + zoneID + "/dns_records")
		if err != nil {
			return nil, err
		}
		q = recordURL.Query()
		q.Set("name", host)
		q.Set("type", recordType)
		q.Set("per_page", "100")
		recordURL.RawQuery = q.Encode()
		var records cloudflareListResponse
		if err := cloudflareAPIRequest(http.MethodGet, recordURL.String(), token, nil, &records); err != nil {
			return nil, err
		}
		for _, record := range records.Result {
			record.ZoneID = zoneID
			record.Name = normalizedDomainMonitorHost(record.Name)
			out = append(out, record)
		}
	}
	return out, nil
}

func updateCloudflareDNSRecordContent(record cloudflareDNSRecord, cred Credential, content string) error {
	if record.ZoneID == "" {
		return errors.New("Cloudflare DNS 记录缺少 Zone ID")
	}
	if record.ID == "" {
		return errors.New("Cloudflare DNS 记录缺少记录 ID")
	}
	endpoint := strings.TrimRight(cloudflareAPIBaseURL, "/") + "/zones/" + record.ZoneID + "/dns_records/" + record.ID
	payload := map[string]any{
		"type":    strings.ToUpper(record.Type),
		"name":    record.Name,
		"content": content,
		"proxied": record.Proxied,
	}
	if record.TTL > 0 {
		payload["ttl"] = record.TTL
	}
	var resp cloudflareRecordResponse
	return cloudflareAPIRequest(http.MethodPatch, endpoint, cred.CFToken, payload, &resp)
}

func cloudflareAPIRequest(method string, endpoint string, token string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 8 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare API 返回 %s: %s", res.Status, strings.TrimSpace(string(data)))
	}
	if target == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	if msg := cloudflareAPIErrorMessage(target); msg != "" {
		return errors.New(msg)
	}
	return nil
}

func cloudflareAPIErrorMessage(target any) string {
	var errs []cloudflareAPIError
	success := true
	switch v := target.(type) {
	case *cloudflareListResponse:
		success = v.Success
		errs = v.Errors
	case *cloudflareRecordResponse:
		success = v.Success
		errs = v.Errors
	}
	if success {
		return ""
	}
	parts := []string{}
	for _, err := range errs {
		if err.Message == "" {
			continue
		}
		if err.Code != 0 {
			parts = append(parts, fmt.Sprintf("%d: %s", err.Code, err.Message))
		} else {
			parts = append(parts, err.Message)
		}
	}
	if len(parts) == 0 {
		return "Cloudflare API 返回失败"
	}
	return "Cloudflare API 返回失败: " + strings.Join(parts, "; ")
}

func currentPublicIPs(wantIPv4 bool, wantIPv6 bool, ipServiceURL string, cache map[string][]string) ([]string, error) {
	ipServiceURL = normalizeDynamicDNSIPServiceURL(ipServiceURL)
	key := fmt.Sprintf("%t/%t/%s", wantIPv4, wantIPv6, ipServiceURL)
	if cache != nil {
		if ips, ok := cache[key]; ok {
			return ips, nil
		}
	}
	if ipServiceURL != "" {
		out, err := fetchPublicIPsFromService(ipServiceURL, wantIPv4, wantIPv6)
		if err != nil {
			return nil, err
		}
		if cache != nil {
			cache[key] = out
		}
		return out, nil
	}
	if out, err := currentPublicIPsFromFallbacks(wantIPv4, wantIPv6, cache); err == nil {
		return out, nil
	} else if !wantIPv6 {
		return nil, err
	}
	out := []string{}
	if wantIPv4 {
		ip, err := fetchPublicIP(publicIPv4Endpoint)
		if err != nil {
			return nil, err
		}
		if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() != nil {
			out = append(out, parsed.String())
		} else {
			return nil, fmt.Errorf("IPv4 服务返回无效地址 %q", ip)
		}
	}
	if wantIPv6 {
		ip, err := fetchPublicIP(publicIPv6Endpoint)
		if err != nil {
			return nil, err
		}
		if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() == nil {
			out = append(out, parsed.String())
		} else if wantIPv4 {
			// api64 may return IPv4 on IPv4-only networks; ignore it when IPv4 was already checked.
		} else {
			return nil, fmt.Errorf("IPv6 服务返回无效地址 %q", ip)
		}
	}
	out = uniqueSortedStrings(out)
	if len(out) == 0 {
		return nil, errors.New("未获取到公网 IP")
	}
	if cache != nil {
		cache[key] = out
	}
	return out, nil
}

func currentPublicIPsFromFallbacks(wantIPv4 bool, wantIPv6 bool, cache map[string][]string) ([]string, error) {
	cacheKey := fmt.Sprintf("%t/%t/auto", wantIPv4, wantIPv6)
	if cache != nil {
		if ips, ok := cache[cacheKey]; ok {
			return ips, nil
		}
	}
	endpoints := []string{}
	if wantIPv4 {
		endpoints = append(endpoints, publicIPv4Endpoint)
	}
	if wantIPv6 {
		endpoints = append(endpoints, publicIPv6Endpoint)
	}
	for _, endpoint := range publicIPFallbackEndpoints {
		endpoints = append(endpoints, endpoint)
	}
	endpoints = uniqueStringsPreserveOrder(endpoints)
	errorsOut := []string{}
	collected := []string{}
	for _, endpoint := range endpoints {
		endpoint = normalizeDynamicDNSIPServiceURL(endpoint)
		if endpoint == "" {
			continue
		}
		key := fmt.Sprintf("%t/%t/%s", wantIPv4, wantIPv6, endpoint)
		if cache != nil {
			if ips, ok := cache[key]; ok {
				return ips, nil
			}
		}
		ips, err := fetchPublicIPsFromService(endpoint, wantIPv4, wantIPv6)
		if err != nil {
			errorsOut = append(errorsOut, fmt.Sprintf("%s: %v", endpoint, err))
			continue
		}
		collected = append(collected, publicIPsNotAlreadyIncluded(collected, ips)...)
		if (!wantIPv4 || containsIPv4(collected)) && (!wantIPv6 || containsIPv6(collected)) {
			break
		}
	}
	if len(collected) > 0 {
		out := uniqueSortedStrings(collected)
		if cache != nil {
			cache[cacheKey] = out
		}
		return out, nil
	}
	if len(errorsOut) == 0 {
		return nil, errors.New("未配置公网 IP 检测接口")
	}
	return nil, errors.New(strings.Join(errorsOut, "; "))
}

func containsIPv4(values []string) bool {
	for _, value := range values {
		if ip := net.ParseIP(value); ip != nil && ip.To4() != nil {
			return true
		}
	}
	return false
}

func containsIPv6(values []string) bool {
	for _, value := range values {
		if ip := net.ParseIP(value); ip != nil && ip.To4() == nil {
			return true
		}
	}
	return false
}

func publicIPsNotAlreadyIncluded(existing []string, next []string) []string {
	seen := map[string]bool{}
	for _, value := range existing {
		if ip := net.ParseIP(value); ip != nil {
			seen[ip.String()] = true
			continue
		}
		seen[strings.TrimSpace(value)] = true
	}
	out := []string{}
	for _, value := range next {
		key := strings.TrimSpace(value)
		if ip := net.ParseIP(value); ip != nil {
			key = ip.String()
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func uniqueStringsPreserveOrder(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, strings.TrimSpace(value))
	}
	return out
}

func fetchPublicIPsFromService(endpoint string, wantIPv4 bool, wantIPv6 bool) ([]string, error) {
	body, err := fetchPublicIP(endpoint)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, token := range strings.FieldsFunc(body, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		ip := net.ParseIP(token)
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			if wantIPv4 {
				out = append(out, ip.String())
			}
			continue
		}
		if wantIPv6 {
			out = append(out, ip.String())
		}
	}
	out = uniqueSortedStrings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("%s 未返回匹配的公网 IP", endpoint)
	}
	return out, nil
}

func fetchPublicIP(endpoint string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(endpoint)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 256))
	if err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("%s 返回 %s", endpoint, res.Status)
	}
	return strings.TrimSpace(string(body)), nil
}

func stringSetsEqual(a []string, b []string) bool {
	a = uniqueSortedStrings(a)
	b = uniqueSortedStrings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// extractDDNSDomain 从 caddy-dynamicdns 的 error 消息中提取域名。
// 已知格式：
//
//	"domain komga.cc.cd not found" → "komga.cc.cd"
//	"updating DNS record for example.com: ..." → "example.com"
func extractDDNSDomain(msg string) string {
	// 匹配 "domain X not found" 或 "domain X ..." 格式
	if after, ok := strings.CutPrefix(msg, "domain "); ok {
		// 去掉尾部描述词（"not found"、":"等）
		domain := strings.Fields(after)[0]
		domain = strings.TrimRight(domain, ":")
		if domain != "" && strings.Contains(domain, ".") {
			return domain
		}
	}
	return ""
}

// persistDDNSError 把错误写到包含该域名的 dynamic-dns JSON 条目的 meta。
func persistDDNSError(domain, msg string, at time.Time) error {
	items, err := loadNPMDynamicDNS()
	if err != nil {
		return err
	}
	changed := false
	for i := range items {
		if !items[i].Enabled {
			continue
		}
		matched := false
		for _, d := range items[i].DomainNames {
			// Stored format can be "zone name" (e.g. "komga.cc.cd a") or bare FQDN.
			// domain is the zone extracted from the DDNS error message.
			if d == domain || strings.HasPrefix(d, domain+" ") || strings.HasSuffix(d, "."+domain) {
				matched = true
				break
			}
		}
		if matched {
			if items[i].Meta == nil {
				items[i].Meta = map[string]any{}
			}
			items[i].Meta["last_error"] = msg
			items[i].Meta["last_error_at"] = at.Format(time.RFC3339)
			changed = true
			break
		}
	}
	if !changed {
		return nil
	}
	return saveJSONFile(dynamicDNSPath, items)
}

// persistDDNSErrorAll 把错误写到所有启用的 dynamic-dns 条目（没有具体域名时兜底）。
func persistDDNSErrorAll(msg string, at time.Time) error {
	items, err := loadNPMDynamicDNS()
	if err != nil {
		return err
	}
	changed := false
	for i := range items {
		if !items[i].Enabled {
			continue
		}
		if items[i].Meta == nil {
			items[i].Meta = map[string]any{}
		}
		items[i].Meta["last_error"] = msg
		items[i].Meta["last_error_at"] = at.Format(time.RFC3339)
		changed = true
	}
	if !changed {
		return nil
	}
	return saveJSONFile(dynamicDNSPath, items)
}

// clearDDNSError 清除包含该域名的 dynamic-dns 条目的错误。
func clearDDNSError(domain string) error {
	items, err := loadNPMDynamicDNS()
	if err != nil {
		return err
	}
	changed := false
	for i := range items {
		if items[i].Meta == nil {
			continue
		}
		matched := false
		for _, d := range items[i].DomainNames {
			if d == domain || strings.HasPrefix(d, domain+" ") || strings.HasSuffix(d, "."+domain) {
				matched = true
				break
			}
		}
		if matched {
			delete(items[i].Meta, "last_error")
			delete(items[i].Meta, "last_error_at")
			changed = true
			break
		}
	}
	if !changed {
		return nil
	}
	return saveJSONFile(dynamicDNSPath, items)
}

// clearAllDDNSErrors 清除所有 dynamic-dns 条目的错误（全局成功时用）。
func clearAllDDNSErrors() error {
	items, err := loadNPMDynamicDNS()
	if err != nil {
		return err
	}
	changed := false
	for i := range items {
		if items[i].Meta == nil {
			continue
		}
		if _, ok := items[i].Meta["last_error"]; ok {
			delete(items[i].Meta, "last_error")
			delete(items[i].Meta, "last_error_at")
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return saveJSONFile(dynamicDNSPath, items)
}
