package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func withTempPaths(t *testing.T, dir string) {
	t.Helper()
	oldCredentialPath := credentialPath
	oldCustomCertMeta := customCertMeta
	oldCustomCertsDir := customCertsDir
	oldIssuerCredentialPath := issuerCredentialPath
	oldDomainMonitorPath := domainMonitorPath
	oldWakeDevicePath := wakeDevicePath
	oldNotificationChannelPath := notificationChannelPath
	oldNotificationStatePath := notificationStatePath
	oldAutheliaConfigPath := autheliaConfigPath
	oldAuthentikConfigPath := authentikConfigPath
	credentialPath = filepath.Join(dir, "credentials.json")
	customCertMeta = filepath.Join(dir, "custom-certs.json")
	customCertsDir = filepath.Join(dir, "custom-certs")
	issuerCredentialPath = filepath.Join(dir, "issuer-credentials.json")
	domainMonitorPath = filepath.Join(dir, "domain-monitor.json")
	wakeDevicePath = filepath.Join(dir, "wake-devices.json")
	notificationChannelPath = filepath.Join(dir, "notification-channels.json")
	notificationStatePath = filepath.Join(dir, "notification-state.json")
	autheliaConfigPath = filepath.Join(dir, "authelia.json")
	authentikConfigPath = filepath.Join(dir, "authentik.json")
	t.Cleanup(func() {
		credentialPath = oldCredentialPath
		customCertMeta = oldCustomCertMeta
		customCertsDir = oldCustomCertsDir
		issuerCredentialPath = oldIssuerCredentialPath
		domainMonitorPath = oldDomainMonitorPath
		wakeDevicePath = oldWakeDevicePath
		notificationChannelPath = oldNotificationChannelPath
		notificationStatePath = oldNotificationStatePath
		autheliaConfigPath = oldAutheliaConfigPath
		authentikConfigPath = oldAuthentikConfigPath
	})
}

func writeTestCert(t *testing.T, path string, commonName string, dnsNames ...string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey error = %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(90 * 24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
}

func TestProxyHostUsesAutoModeWhenNoSingleCertificateCoversAllDomains(t *testing.T) {
	site := Site{
		Name:    "multi-domain",
		Domain:  "a.domain.com, b.domain.com",
		Backend: "http://127.0.0.1:8080",
	}
	certs := map[string]npmCertificate{
		"a.domain.com": {ID: 1, DomainNames: []string{"a.domain.com"}},
		"b.domain.com": {ID: 2, DomainNames: []string{"b.domain.com"}},
	}

	host := siteToNPMProxyHost(site, certs)

	if got := asInt(host.CertificateID); got != -1 {
		t.Fatalf("CertificateID = %d, want -1 for Caddy auto mode", got)
	}
	if host.Certificate != nil {
		t.Fatalf("Certificate = %#v, want nil when no single certificate covers all domains", host.Certificate)
	}
	bindings, ok := host.Meta["certificate_bindings"].([]CertificateBinding)
	if !ok {
		t.Fatalf("certificate_bindings = %#v, want []CertificateBinding", host.Meta["certificate_bindings"])
	}
	if len(bindings) != 2 || bindings[0].CertificateID != 1 || bindings[1].CertificateID != 2 {
		t.Fatalf("certificate_bindings = %#v, want per-domain selected bindings", bindings)
	}
}

func TestRedirectionAndDeadHostsRoundTripCertificateBindings(t *testing.T) {
	meta := map[string]any{
		"certificate_bindings": []CertificateBinding{
			{Domain: "a.domain.com", Mode: "selected", CertificateID: 1},
			{Domain: "b.domain.com", Mode: "auto"},
		},
	}
	certs := map[string]npmCertificate{
		"a.domain.com": {ID: 1, NiceName: "a.domain.com", DomainNames: []string{"a.domain.com"}},
	}

	redirectionSite, err := npmRedirectionHostToSite(npmRedirectionHost{
		DomainNames:       []string{"a.domain.com", "b.domain.com"},
		ForwardDomainName: "target.domain.com",
		ForwardScheme:     "https",
		ForwardHTTPCode:   http.StatusMovedPermanently,
		CertificateID:     -1,
		SSLForced:         true,
		Enabled:           true,
		Meta:              meta,
	}, "redir")
	if err != nil {
		t.Fatalf("npmRedirectionHostToSite error = %v", err)
	}
	if redirectionSite.CertificateMode != "auto" || len(redirectionSite.CertificateBindings) != 2 {
		t.Fatalf("redirection certificate state = %q %#v, want auto with two bindings", redirectionSite.CertificateMode, redirectionSite.CertificateBindings)
	}
	redirectionOut := siteToNPMRedirectionHost(redirectionSite, certs)
	if got := asInt(redirectionOut.CertificateID); got != -1 {
		t.Fatalf("redirection CertificateID = %d, want -1", got)
	}
	if bindings := certificateBindingsFromMeta(redirectionOut.Meta); len(bindings) != 2 || bindings[0].CertificateID != 1 {
		t.Fatalf("redirection certificate_bindings = %#v, want selected and auto bindings", bindings)
	}

	deadSite, err := npmDeadHostToSite(npmDeadHost{
		DomainNames:   []string{"a.domain.com", "b.domain.com"},
		CertificateID: -1,
		SSLForced:     true,
		Enabled:       true,
		Meta:          meta,
	}, "dead")
	if err != nil {
		t.Fatalf("npmDeadHostToSite error = %v", err)
	}
	if deadSite.CertificateMode != "auto" || len(deadSite.CertificateBindings) != 2 {
		t.Fatalf("dead certificate state = %q %#v, want auto with two bindings", deadSite.CertificateMode, deadSite.CertificateBindings)
	}
	deadOut := siteToNPMDeadHost(deadSite, certs)
	if got := asInt(deadOut.CertificateID); got != -1 {
		t.Fatalf("dead CertificateID = %d, want -1", got)
	}
	if bindings := certificateBindingsFromMeta(deadOut.Meta); len(bindings) != 2 || bindings[0].CertificateID != 1 {
		t.Fatalf("dead certificate_bindings = %#v, want selected and auto bindings", bindings)
	}
}

func TestCertificateAuthoritiesReturnsSavedEABCredentials(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	if err := saveJSONFile(issuerCredentialPath, map[string]IssuerConfig{
		"google": {
			Provider:    "google",
			EABKeyID:    "kid-123",
			EABMACKey:   "mac-456",
			CADirectory: "https://dv.acme-v02.api.pki.goog/directory",
		},
	}); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/caddy/certificates/authorities", nil)
	rr := httptest.NewRecorder()

	npmCertificateAuthoritiesHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &rows); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	var google map[string]any
	for _, row := range rows {
		if row["id"] == "google" {
			google = row
			break
		}
	}
	if google == nil {
		t.Fatalf("google authority not returned: %#v", rows)
	}
	if google["eab_key_id"] != "kid-123" || google["eab_mac_key"] != "mac-456" {
		t.Fatalf("google EAB = %#v/%#v, want saved credentials", google["eab_key_id"], google["eab_mac_key"])
	}
	if google["saved"] != true {
		t.Fatalf("saved = %#v, want true", google["saved"])
	}
}

func TestProxyHostUsesWildcardCertificateWhenItCoversAllDomains(t *testing.T) {
	site := Site{
		Name:    "multi-domain",
		Domain:  "a.domain.com, b.domain.com",
		Backend: "http://127.0.0.1:8080",
	}
	certs := map[string]npmCertificate{
		"*.domain.com": {ID: 10, DomainNames: []string{"*.domain.com"}},
	}

	host := siteToNPMProxyHost(site, certs)

	if got := asInt(host.CertificateID); got != 10 {
		t.Fatalf("CertificateID = %d, want wildcard certificate id", got)
	}
	if host.Certificate == nil || host.Certificate.ID != 10 {
		t.Fatalf("Certificate = %#v, want wildcard certificate", host.Certificate)
	}
}

func TestProxyHostPersistsExplicitAutoCertificateMode(t *testing.T) {
	site := Site{
		Name:            "explicit-auto",
		Domain:          "a.domain.com",
		Backend:         "http://127.0.0.1:8080",
		CertificateMode: "auto",
	}
	certs := map[string]npmCertificate{
		"a.domain.com": {ID: 1, DomainNames: []string{"a.domain.com"}},
	}

	host := siteToNPMProxyHost(site, certs)

	if got := asInt(host.CertificateID); got != -1 {
		t.Fatalf("CertificateID = %d, want -1 for explicit Caddy auto mode", got)
	}
	if got := host.Meta["certificate_mode"]; got != "auto" {
		t.Fatalf("certificate_mode meta = %#v, want auto", got)
	}
}

func TestProxyHostAutoModeExposesSelectedBindingCertificate(t *testing.T) {
	site := Site{
		Name:            "selected-binding",
		Domain:          "a.domain.com",
		Backend:         "http://127.0.0.1:8080",
		CertificateMode: "auto",
		CertificateBindings: []CertificateBinding{
			{Domain: "a.domain.com", Mode: "selected", CertificateID: 1},
		},
	}
	certs := map[string]npmCertificate{
		"a.domain.com": {ID: 1, Provider: "letsencrypt", NiceName: "a.domain.com", DomainNames: []string{"a.domain.com"}},
	}

	host := siteToNPMProxyHost(site, certs)

	if got := asInt(host.CertificateID); got != -1 {
		t.Fatalf("CertificateID = %d, want -1 to preserve per-domain certificate mode", got)
	}
	if host.Certificate == nil || host.Certificate.ID != 1 {
		t.Fatalf("Certificate = %#v, want selected binding certificate", host.Certificate)
	}
}

func TestWildcardProxyHostDetailsPreferWildcardCertificateOverStaleExactBinding(t *testing.T) {
	wildcardID := stableID("cert:*.domain.com")
	exactID := stableID("cert:a.domain.com")
	site := Site{
		Name:            "selected-wildcard",
		Domain:          "a.domain.com",
		Backend:         "http://127.0.0.1:8080",
		Wildcard:        true,
		ChallengePref:   "dns",
		CredentialID:    "cred-1",
		CertificateMode: "auto",
		CertificateBindings: []CertificateBinding{
			{
				Domain:            "a.domain.com",
				Mode:              "selected",
				CertificateID:     exactID,
				CertificateDomain: "a.domain.com",
				Provider:          "letsencrypt",
				ChallengePref:     "http",
			},
		},
	}
	certs := map[string]npmCertificate{
		"*.domain.com": {ID: wildcardID, Provider: "letsencrypt", NiceName: "*.domain.com", DomainNames: []string{"*.domain.com"}, Meta: map[string]any{"sign_method": "DNS-01", "credential_id": "cred-1"}},
		"a.domain.com": {ID: exactID, Provider: "letsencrypt", NiceName: "a.domain.com", DomainNames: []string{"a.domain.com"}},
	}

	host := siteToNPMProxyHost(site, certs)

	if host.Certificate == nil || host.Certificate.ID != wildcardID {
		t.Fatalf("Certificate = %#v, want wildcard certificate", host.Certificate)
	}
	bindings := certificateBindingsFromMeta(host.Meta)
	if len(bindings) != 1 {
		t.Fatalf("bindings = %#v, want one binding", bindings)
	}
	if bindings[0].CertificateID != wildcardID || bindings[0].CertificateDomain != "*.domain.com" || bindings[0].ChallengePref != "dns" {
		t.Fatalf("binding = %#v, want wildcard DNS binding", bindings[0])
	}
}

func TestRenderWildcardSiteIgnoresStaleExactBinding(t *testing.T) {
	site := Site{
		Name:          "selected-wildcard",
		Domain:        "a.domain.com",
		Backend:       "http://127.0.0.1:8080",
		Wildcard:      true,
		ChallengePref: "dns",
		CredentialID:  "cred-1",
		CertificateBindings: []CertificateBinding{
			{
				Domain:            "a.domain.com",
				Mode:              "selected",
				CertificateID:     stableID("cert:a.domain.com"),
				CertificateDomain: "a.domain.com",
				Provider:          "letsencrypt",
				ChallengePref:     "http",
			},
		},
	}

	conf, err := renderSite(site, []Credential{{ID: "cred-1", Provider: "cloudflare", CFToken: "token"}})
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	if !strings.Contains(conf, "*.domain.com {\n") || strings.HasPrefix(conf, "a.domain.com {") {
		t.Fatalf("renderSite = %q, want wildcard site address", conf)
	}
	if !strings.Contains(conf, "dns cloudflare \"token\"") {
		t.Fatalf("renderSite = %q, want wildcard DNS credential", conf)
	}
}

func TestRenderWildcardSiteRecoversCredentialFromACMEWildcardCredential(t *testing.T) {
	site := Site{
		Name:     "selected-wildcard",
		Domain:   "a.domain.com",
		Backend:  "http://127.0.0.1:8080",
		Wildcard: true,
	}
	creds := []Credential{{
		ID:       "cred-1",
		Name:     "ACME *.domain.com",
		Provider: "cloudflare",
		CFToken:  "token",
	}}

	conf, err := renderSite(site, creds)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	if !strings.Contains(conf, "*.domain.com {\n") {
		t.Fatalf("renderSite = %q, want wildcard site address", conf)
	}
	if !strings.Contains(conf, "dns cloudflare \"token\"") {
		t.Fatalf("renderSite = %q, want recovered wildcard DNS credential", conf)
	}
}

func TestSelectedBindingUsesWildcardCertificateConfig(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCredentialPath := credentialPath
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	credentialPath = filepath.Join(tmp, "credentials.json")
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		credentialPath = oldCredentialPath
		caddyCertsDir = oldCaddyCertsDir
	})
	issuer := IssuerConfig{Provider: "letsencrypt"}
	if err := saveJSONFile(credentialPath, []Credential{{
		ID:       "cred-1",
		Name:     "ACME *.domain.com",
		Provider: "cloudflare",
		CFToken:  "token",
		Issuer:   issuer,
	}}); err != nil {
		t.Fatalf("save credentials error = %v", err)
	}
	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "wildcard_.domain.com", "wildcard_.domain.com.crt"), "*.domain.com", "*.domain.com")

	site := Site{
		Name:            "selected-wildcard",
		Domain:          "a.domain.com",
		Backend:         "http://127.0.0.1:8080",
		CertificateMode: "auto",
		CertificateBindings: []CertificateBinding{
			{
				Domain:            "a.domain.com",
				Mode:              "selected",
				CertificateID:     stableID("cert:*.domain.com"),
				CertificateDomain: "*.domain.com",
				Provider:          "letsencrypt",
				ChallengePref:     "dns",
				CredentialID:      "cred-1",
				Issuer:            issuer,
			},
		},
	}

	child := certificateBoundSiteForDomainWithCredentials(site, "a.domain.com", nil)
	if !child.Wildcard {
		t.Fatalf("Wildcard = false, want true for selected wildcard certificate")
	}
	if child.CredentialID != "cred-1" || child.ChallengePref != "dns" {
		t.Fatalf("credential/challenge = %q/%q, want cred-1/dns", child.CredentialID, child.ChallengePref)
	}

	rows := certOverviewRows()
	for _, row := range rows {
		if row.Domain == "a.domain.com" && row.Status == "pending" {
			t.Fatalf("unexpected pending concrete certificate row: %#v", row)
		}
	}
}

func TestRenderSelectedWildcardBindingUsesWildcardSiteAddress(t *testing.T) {
	issuer := IssuerConfig{Provider: "letsencrypt"}
	site := Site{
		Name:            "selected-wildcard",
		Domain:          "a.domain.com",
		Backend:         "http://127.0.0.1:8080",
		CertificateMode: "auto",
		CertificateBindings: []CertificateBinding{
			{
				Domain:            "a.domain.com",
				Mode:              "selected",
				CertificateID:     stableID("cert:*.domain.com"),
				CertificateDomain: "*.domain.com",
				Provider:          "letsencrypt",
				ChallengePref:     "dns",
				CredentialID:      "cred-1",
				Issuer:            issuer,
			},
		},
	}
	conf, err := renderSite(site, []Credential{{ID: "cred-1", Provider: "cloudflare", CFToken: "token"}})
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	matcher := siteHostMatcherName(site, []string{"a.domain.com"})
	for _, want := range []string{
		"*.domain.com {\n",
		matcher + " host a.domain.com",
		"handle " + matcher + " {",
		"reverse_proxy http://127.0.0.1:8080",
		"dns cloudflare \"token\"",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
	if strings.HasPrefix(conf, "a.domain.com {") {
		t.Fatalf("renderSite = %q, should use wildcard site address instead of exact domain", conf)
	}
}

func TestRenderWildcardCertificateUsesWildcardSiteAddress(t *testing.T) {
	site := Site{
		Name:          "wildcard-cert",
		Domain:        "a.domain.com, b.domain.com",
		Backend:       "http://127.0.0.1:8080",
		Wildcard:      true,
		ChallengePref: "dns",
		CredentialID:  "cred-1",
		Issuer:        IssuerConfig{Provider: "letsencrypt"},
	}
	conf, err := renderSite(site, []Credential{{ID: "cred-1", Provider: "cloudflare", CFToken: "token"}})
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	matcher := siteHostMatcherName(site, []string{"a.domain.com", "b.domain.com"})
	for _, want := range []string{
		"*.domain.com {\n",
		matcher + " host a.domain.com b.domain.com",
		"handle " + matcher + " {",
		"reverse_proxy http://127.0.0.1:8080",
		"dns cloudflare \"token\"",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
	if strings.Contains(conf, "a.domain.com, b.domain.com {") || strings.HasPrefix(conf, "a.domain.com {") {
		t.Fatalf("renderSite = %q, should not expose exact domains as Caddy site addresses", conf)
	}
}

func TestRenderAllSiteConfsAvoidsDuplicateWildcardSiteAddresses(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	sitesDir = tmp
	t.Cleanup(func() {
		sitesDir = oldSitesDir
	})
	cred := Credential{ID: "cred-1", Provider: "cloudflare", CFToken: "token"}
	for _, site := range []Site{
		{
			Name:          "wild-a",
			Domain:        "a.domain.com",
			Backend:       "http://127.0.0.1:8080",
			Wildcard:      true,
			ChallengePref: "dns",
			CredentialID:  cred.ID,
		},
		{
			Name:          "wild-b",
			Domain:        "b.domain.com",
			Backend:       "http://127.0.0.1:8081",
			Wildcard:      true,
			ChallengePref: "dns",
			CredentialID:  cred.ID,
		},
	} {
		if err := saveJSONFile(filepath.Join(sitesDir, site.Name+metaSuffix), site); err != nil {
			t.Fatalf("save site %s error = %v", site.Name, err)
		}
	}

	if err := renderAllSiteConfs([]Credential{cred}); err != nil {
		t.Fatalf("renderAllSiteConfs error = %v", err)
	}
	for _, name := range []string{"wild-a", "wild-b"} {
		if _, err := os.Stat(filepath.Join(sitesDir, name+confSuffix)); !os.IsNotExist(err) {
			t.Fatalf("%s conf still exists, stat err = %v", name, err)
		}
	}
	groupPath := filepath.Join(sitesDir, wildcardGroupPrefix+strconv.Itoa(stableID("*.domain.com"))+confSuffix)
	data, err := os.ReadFile(groupPath)
	if err != nil {
		t.Fatalf("read wildcard group conf error = %v", err)
	}
	conf := string(data)
	for _, want := range []string{
		"*.domain.com {\n",
		"host a.domain.com",
		"host b.domain.com",
		"reverse_proxy http://127.0.0.1:8080",
		"reverse_proxy http://127.0.0.1:8081",
		"dns cloudflare \"token\"",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("wildcard group conf = %q, want %q", conf, want)
		}
	}
	if strings.Count(conf, "*.domain.com {\n") != 1 {
		t.Fatalf("wildcard group conf = %q, want one wildcard site block", conf)
	}
}

func TestSyncWildcardPlaceholdersRemovesPlaceholderForRenderedWildcardSite(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	sitesDir = tmp
	t.Cleanup(func() {
		sitesDir = oldSitesDir
	})
	if err := os.MkdirAll(sitesDir, 0755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	site := Site{
		Name:          "wildcard-cert",
		Domain:        "a.domain.com",
		Backend:       "http://127.0.0.1:8080",
		Wildcard:      true,
		ChallengePref: "dns",
		CredentialID:  "cred-1",
	}
	if err := saveJSONFile(filepath.Join(sitesDir, "wildcard-cert"+metaSuffix), site); err != nil {
		t.Fatalf("save site error = %v", err)
	}
	conflictBase := managedCertPrefix + "domain.com"
	conflictConf := filepath.Join(sitesDir, conflictBase+confSuffix)
	conflictMeta := filepath.Join(sitesDir, conflictBase+metaSuffix)
	if err := os.WriteFile(conflictConf, []byte("*.domain.com {\n    respond 404\n}\n"), 0644); err != nil {
		t.Fatalf("write conflict conf error = %v", err)
	}
	if err := saveJSONFile(conflictMeta, placeholderMeta{Domain: "*.domain.com", CredentialID: "cred-1"}); err != nil {
		t.Fatalf("save conflict meta error = %v", err)
	}
	standaloneBase := managedCertPrefix + "other.com"
	standaloneConf := filepath.Join(sitesDir, standaloneBase+confSuffix)
	standaloneMeta := filepath.Join(sitesDir, standaloneBase+metaSuffix)
	if err := os.WriteFile(standaloneConf, []byte("*.other.com {\n    respond 404\n}\n"), 0644); err != nil {
		t.Fatalf("write standalone conf error = %v", err)
	}
	if err := saveJSONFile(standaloneMeta, placeholderMeta{Domain: "*.other.com", CredentialID: "cred-2"}); err != nil {
		t.Fatalf("save standalone meta error = %v", err)
	}

	if _, err := syncWildcardPlaceholders(nil); err != nil {
		t.Fatalf("syncWildcardPlaceholders error = %v", err)
	}
	if _, err := os.Stat(conflictConf); !os.IsNotExist(err) {
		t.Fatalf("conflict placeholder conf still exists, stat err = %v", err)
	}
	if _, err := os.Stat(conflictMeta); !os.IsNotExist(err) {
		t.Fatalf("conflict placeholder meta still exists, stat err = %v", err)
	}
	if _, err := os.Stat(standaloneConf); err != nil {
		t.Fatalf("standalone placeholder conf was removed: %v", err)
	}
	if _, err := os.Stat(standaloneMeta); err != nil {
		t.Fatalf("standalone placeholder meta was removed: %v", err)
	}
}

func TestCleanupLegacyExactManagedCertPlaceholdersRemovesUnlinkedRecoveredPlaceholder(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCredentialPath := credentialPath
	sitesDir = filepath.Join(tmp, "sites")
	credentialPath = filepath.Join(tmp, "credentials.json")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		credentialPath = oldCredentialPath
	})
	if err := os.MkdirAll(sitesDir, 0755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := saveJSONFile(credentialPath, []Credential{{
		ID:       "cred-exact",
		Name:     "ACME exact.domain.com",
		Provider: "cloudflare",
		CFToken:  "token",
	}}); err != nil {
		t.Fatalf("save credentials error = %v", err)
	}
	baseName := managedCertPrefix + "exact.domain.com"
	confPath := filepath.Join(sitesDir, baseName+confSuffix)
	metaPath := filepath.Join(sitesDir, baseName+metaSuffix)
	if err := os.WriteFile(confPath, []byte("exact.domain.com {\n    respond 404\n}\n"), 0644); err != nil {
		t.Fatalf("write placeholder conf error = %v", err)
	}
	if err := saveJSONFile(metaPath, placeholderMeta{Domain: "exact.domain.com", CredentialID: "cred-exact"}); err != nil {
		t.Fatalf("save placeholder meta error = %v", err)
	}

	if err := cleanupLegacyExactManagedCertPlaceholders(); err != nil {
		t.Fatalf("cleanupLegacyExactManagedCertPlaceholders error = %v", err)
	}
	if _, err := os.Stat(confPath); !os.IsNotExist(err) {
		t.Fatalf("placeholder conf still exists, stat err = %v", err)
	}
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Fatalf("placeholder meta still exists, stat err = %v", err)
	}
}

func TestCleanupLegacyExactManagedCertPlaceholdersKeepsLinkedPlaceholder(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCredentialPath := credentialPath
	sitesDir = filepath.Join(tmp, "sites")
	credentialPath = filepath.Join(tmp, "credentials.json")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		credentialPath = oldCredentialPath
	})
	if err := os.MkdirAll(sitesDir, 0755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := saveJSONFile(credentialPath, []Credential{{
		ID:       "cred-exact",
		Name:     "ACME exact.domain.com",
		Provider: "cloudflare",
		CFToken:  "token",
	}}); err != nil {
		t.Fatalf("save credentials error = %v", err)
	}
	if err := saveJSONFile(filepath.Join(sitesDir, "proxy"+metaSuffix), Site{
		Name:    "proxy",
		Domain:  "exact.domain.com",
		Backend: "http://127.0.0.1:8080",
	}); err != nil {
		t.Fatalf("save site error = %v", err)
	}
	baseName := managedCertPrefix + "exact.domain.com"
	confPath := filepath.Join(sitesDir, baseName+confSuffix)
	metaPath := filepath.Join(sitesDir, baseName+metaSuffix)
	if err := os.WriteFile(confPath, []byte("exact.domain.com {\n    respond 404\n}\n"), 0644); err != nil {
		t.Fatalf("write placeholder conf error = %v", err)
	}
	if err := saveJSONFile(metaPath, placeholderMeta{Domain: "exact.domain.com", CredentialID: "cred-exact"}); err != nil {
		t.Fatalf("save placeholder meta error = %v", err)
	}

	if err := cleanupLegacyExactManagedCertPlaceholders(); err != nil {
		t.Fatalf("cleanupLegacyExactManagedCertPlaceholders error = %v", err)
	}
	if _, err := os.Stat(confPath); err != nil {
		t.Fatalf("linked placeholder conf was removed: %v", err)
	}
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("linked placeholder meta was removed: %v", err)
	}
}

func TestCertOverviewSelectedBindingUsesIssuedWildcardDomain(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCredentialPath := credentialPath
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = filepath.Join(tmp, "sites")
	credentialPath = filepath.Join(tmp, "credentials.json")
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		credentialPath = oldCredentialPath
		caddyCertsDir = oldCaddyCertsDir
	})
	issuer := IssuerConfig{Provider: "letsencrypt"}
	if err := saveJSONFile(credentialPath, []Credential{{
		ID:       "cred-1",
		Name:     "ACME *.domain.com",
		Provider: "cloudflare",
		CFToken:  "token",
		Issuer:   issuer,
	}}); err != nil {
		t.Fatalf("save credentials error = %v", err)
	}
	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "wildcard_.domain.com", "wildcard_.domain.com.crt"), "*.domain.com", "*.domain.com")
	site := Site{
		Name:            "selected-wildcard",
		Domain:          "a.domain.com",
		Backend:         "http://127.0.0.1:8080",
		CertificateMode: "auto",
		CertificateBindings: []CertificateBinding{
			{
				Domain:        "a.domain.com",
				Mode:          "selected",
				CertificateID: stableID("cert:*.domain.com"),
				Provider:      "letsencrypt",
				ChallengePref: "dns",
				CredentialID:  "cred-1",
				Issuer:        issuer,
			},
		},
	}
	if err := saveJSONFile(filepath.Join(sitesDir, "selected-wildcard"+metaSuffix), site); err != nil {
		t.Fatalf("save site error = %v", err)
	}

	rows := certOverviewRows()
	var wildcardRow *CertOverview
	for i := range rows {
		switch rows[i].Domain {
		case "*.domain.com":
			wildcardRow = &rows[i]
		case "a.domain.com":
			t.Fatalf("unexpected concrete certificate row: %#v", rows[i])
		}
	}
	if wildcardRow == nil {
		t.Fatalf("wildcard certificate row not found: %#v", rows)
	}
	if wildcardRow.Status != "issued" || wildcardRow.SignMethod != "DNS-01" || wildcardRow.CredentialID != "cred-1" {
		t.Fatalf("wildcard row = %#v, want issued DNS-01 with credential", *wildcardRow)
	}
	if len(wildcardRow.LinkedSites) != 1 || wildcardRow.LinkedSites[0] != "selected-wildcard" {
		t.Fatalf("linked sites = %#v, want selected-wildcard", wildcardRow.LinkedSites)
	}
}

func TestCertOverviewHidesOrphanExactCertificateCoveredByWildcard(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCredentialPath := credentialPath
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	credentialPath = filepath.Join(tmp, "credentials.json")
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		credentialPath = oldCredentialPath
		caddyCertsDir = oldCaddyCertsDir
	})

	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "wildcard_.domain.com", "wildcard_.domain.com.crt"), "*.domain.com", "*.domain.com")
	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "a.domain.com", "a.domain.com.crt"), "a.domain.com", "a.domain.com")

	for _, row := range certOverviewRows() {
		if row.Domain == "a.domain.com" {
			t.Fatalf("unexpected orphan exact certificate row covered by wildcard: %#v", row)
		}
	}
}

func TestCertOverviewKeepsLinkedExactCertificateCoveredByWildcard(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCredentialPath := credentialPath
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	credentialPath = filepath.Join(tmp, "credentials.json")
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		credentialPath = oldCredentialPath
		caddyCertsDir = oldCaddyCertsDir
	})

	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "wildcard_.domain.com", "wildcard_.domain.com.crt"), "*.domain.com", "*.domain.com")
	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "a.domain.com", "a.domain.com.crt"), "a.domain.com", "a.domain.com")
	site := Site{
		Name:    "exact-site",
		Domain:  "a.domain.com",
		Backend: "http://127.0.0.1:8080",
	}
	if err := saveJSONFile(filepath.Join(sitesDir, "exact-site"+metaSuffix), site); err != nil {
		t.Fatalf("save site error = %v", err)
	}

	rows := certOverviewRows()
	var exactRow *CertOverview
	for i := range rows {
		row := rows[i]
		if row.Domain == "a.domain.com" {
			exactRow = &rows[i]
			break
		}
	}
	if exactRow == nil {
		t.Fatalf("linked exact certificate row not found")
	}
	if len(exactRow.LinkedSites) != 1 || exactRow.LinkedSites[0] != "exact-site" {
		t.Fatalf("linked sites = %#v, want exact-site", exactRow.LinkedSites)
	}
}

func TestCertOverviewWildcardSiteUsesWildcardDomain(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCredentialPath := credentialPath
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	credentialPath = filepath.Join(tmp, "credentials.json")
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		credentialPath = oldCredentialPath
		caddyCertsDir = oldCaddyCertsDir
	})

	if err := saveJSONFile(credentialPath, []Credential{{
		ID:       "cred-1",
		Name:     "ACME *.domain.com",
		Provider: "cloudflare",
		CFToken:  "token",
	}}); err != nil {
		t.Fatalf("save credentials error = %v", err)
	}
	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "wildcard_.domain.com", "wildcard_.domain.com.crt"), "*.domain.com", "*.domain.com")
	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "a.domain.com", "a.domain.com.crt"), "a.domain.com", "a.domain.com")
	site := Site{
		Name:          "wildcard-site",
		Domain:        "a.domain.com",
		Backend:       "http://127.0.0.1:8080",
		Wildcard:      true,
		ChallengePref: "dns",
		CredentialID:  "cred-1",
	}
	if err := saveJSONFile(filepath.Join(sitesDir, "wildcard-site"+metaSuffix), site); err != nil {
		t.Fatalf("save site error = %v", err)
	}

	rows := certOverviewRows()
	var wildcardRow *CertOverview
	for i := range rows {
		row := rows[i]
		switch row.Domain {
		case "*.domain.com":
			wildcardRow = &rows[i]
		case "a.domain.com":
			t.Fatalf("unexpected exact row for wildcard site: %#v", row)
		}
	}
	if wildcardRow == nil {
		t.Fatalf("wildcard row not found")
	}
	if len(wildcardRow.LinkedSites) != 1 || wildcardRow.LinkedSites[0] != "wildcard-site" {
		t.Fatalf("linked sites = %#v, want wildcard-site", wildcardRow.LinkedSites)
	}
}

func TestCertificateUsageIDsIncludeSelectedBindings(t *testing.T) {
	cert := npmCertificate{ID: 1}
	ids := certificateUsageIDs(-1, &cert, map[string]any{
		"certificate_bindings": []CertificateBinding{
			{Domain: "a.domain.com", Mode: "selected", CertificateID: 2},
			{Domain: "b.domain.com", Mode: "auto"},
			{Domain: "c.domain.com", Mode: "selected", CertificateID: 2},
		},
	})

	wantAutoID := stableID("cert:b.domain.com")
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != wantAutoID {
		t.Fatalf("certificateUsageIDs = %#v, want [1 2 %d]", ids, wantAutoID)
	}
}

func TestCertOverviewUsesRequestConfigForAutoBinding(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCredentialPath := credentialPath
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	credentialPath = filepath.Join(tmp, "credentials.json")
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		credentialPath = oldCredentialPath
		caddyCertsDir = oldCaddyCertsDir
	})

	issuer := IssuerConfig{Provider: "zerossl", CADirectory: "https://acme.zerossl.com/v2/DV90"}
	if err := saveJSONFile(credentialPath, []Credential{{
		ID:       "cred-1",
		Name:     "ACME test.example.net",
		Provider: "cloudflare",
		CFToken:  "token",
		Issuer:   issuer,
	}}); err != nil {
		t.Fatalf("saveJSONFile credentials error = %v", err)
	}
	site := Site{
		Name:            "multi-domain",
		Domain:          "api.example.org, test.example.net",
		Backend:         "http://127.0.0.1:8080",
		CertificateMode: "auto",
		CertificateBindings: []CertificateBinding{
			{Domain: "api.example.org", Mode: "selected", CertificateID: 100, Provider: "google"},
			{Domain: "test.example.net", Mode: "auto"},
		},
	}
	if err := saveJSONFile(filepath.Join(tmp, "multi-domain.json"), site); err != nil {
		t.Fatalf("saveJSONFile site error = %v", err)
	}

	var got CertOverview
	for _, row := range certOverviewRows() {
		if row.Domain == "test.example.net" {
			got = row
			break
		}
	}
	if got.Domain == "" {
		t.Fatalf("test.example.net overview row not found")
	}
	if got.Provider != "zerossl" || got.IssuerConfig.Provider != "zerossl" {
		t.Fatalf("provider = %q/%q, want zerossl", got.Provider, got.IssuerConfig.Provider)
	}
	if got.SignMethod != "DNS-01" || got.CredentialID != "cred-1" {
		t.Fatalf("sign method/credential = %q/%q, want DNS-01/cred-1", got.SignMethod, got.CredentialID)
	}
	if len(got.LinkedSites) != 1 || got.LinkedSites[0] != "multi-domain" {
		t.Fatalf("linked sites = %#v, want multi-domain", got.LinkedSites)
	}
}

func TestCertOverviewInfersProviderForLegacyAutoBindingError(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCredentialPath := credentialPath
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	credentialPath = filepath.Join(tmp, "credentials.json")
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		credentialPath = oldCredentialPath
		caddyCertsDir = oldCaddyCertsDir
	})

	if err := saveJSONFile(credentialPath, []Credential{{ID: "cred-1", Name: "ACME api.example.org", Provider: "cloudflare", CFToken: "token"}}); err != nil {
		t.Fatalf("saveJSONFile credentials error = %v", err)
	}
	site := Site{
		Name:            "multi-domain",
		Domain:          "api.example.org, test.example.net",
		Backend:         "http://127.0.0.1:8080",
		CertificateMode: "auto",
		CertificateBindings: []CertificateBinding{
			{Domain: "api.example.org", Mode: "selected", CertificateID: 100, Provider: "google", CredentialID: "cred-1"},
			{
				Domain:    "test.example.net",
				Mode:      "auto",
				LastError: `[test.example.net] checking DNS propagation of "_acme-challenge.test.example.net." (ca=https://acme.zerossl.com/v2/DV90)`,
			},
		},
	}
	if err := saveJSONFile(filepath.Join(tmp, "multi-domain.json"), site); err != nil {
		t.Fatalf("saveJSONFile site error = %v", err)
	}

	var got CertOverview
	for _, row := range certOverviewRows() {
		if row.Domain == "test.example.net" {
			got = row
			break
		}
	}
	if got.Provider != "zerossl" || got.IssuerConfig.Provider != "zerossl" {
		t.Fatalf("provider = %q/%q, want zerossl", got.Provider, got.IssuerConfig.Provider)
	}
	if got.SignMethod != "DNS-01" || got.CredentialID != "cred-1" {
		t.Fatalf("sign method/credential = %q/%q, want DNS-01/cred-1", got.SignMethod, got.CredentialID)
	}
}

func TestApplyCertificateRequestToSiteUpdatesMatchingBindingIssuer(t *testing.T) {
	site := Site{
		Name:            "multi-domain",
		Domain:          "a.example.com, b.example.com",
		Backend:         "http://127.0.0.1:8080",
		CertificateMode: "auto",
		CertificateBindings: []CertificateBinding{
			{Domain: "a.example.com", Mode: "selected", Provider: "auto"},
			{Domain: "b.example.com", Mode: "selected", Provider: "google"},
		},
	}

	applyCertificateRequestToSite(&site, "a.example.com", IssuerConfig{Provider: "zerossl", CADirectory: "https://acme.zerossl.com/v2/DV90"}, "dns", "cred-1")

	got := site.CertificateBindings[0]
	if got.Provider != "zerossl" || got.Issuer.Provider != "zerossl" {
		t.Fatalf("binding provider = %q/%q, want zerossl", got.Provider, got.Issuer.Provider)
	}
	if got.ChallengePref != "dns" || got.CredentialID != "cred-1" {
		t.Fatalf("binding challenge/credential = %q/%q, want dns/cred-1", got.ChallengePref, got.CredentialID)
	}
	if got.CertificateID != stableID("cert:a.example.com") {
		t.Fatalf("binding certificate ID = %d, want stable cert ID", got.CertificateID)
	}
	if site.CertificateBindings[1].Provider != "google" {
		t.Fatalf("unmatched binding provider = %q, want unchanged google", site.CertificateBindings[1].Provider)
	}
}

func TestPersistCertErrorUsesMatchingBindingAndClearsSiteError(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	sitesDir = tmp
	t.Cleanup(func() { sitesDir = oldSitesDir })

	oldSiteErrorAt := time.Now().Add(-time.Hour)
	site := Site{
		Name:        "multi-domain",
		Domain:      "a.example.com, b.example.com",
		Backend:     "http://127.0.0.1:8080",
		LastError:   "old shared error",
		LastErrorAt: oldSiteErrorAt,
		CertificateBindings: []CertificateBinding{
			{Domain: "a.example.com", Mode: "selected", LastError: "a error", LastErrorAt: oldSiteErrorAt},
			{Domain: "b.example.com", Mode: "selected"},
		},
	}
	if err := saveJSONFile(filepath.Join(tmp, "multi-domain.json"), site); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	errAt := time.Now()
	if err := persistCertError("b.example.com", "b error", errAt); err != nil {
		t.Fatalf("persistCertError error = %v", err)
	}

	var got Site
	if err := loadJSONFile(filepath.Join(tmp, "multi-domain.json"), &got); err != nil {
		t.Fatalf("loadJSONFile error = %v", err)
	}
	if got.LastError != "" || !got.LastErrorAt.IsZero() {
		t.Fatalf("site error = %q/%v, want cleared", got.LastError, got.LastErrorAt)
	}
	if got.CertificateBindings[0].LastError != "a error" {
		t.Fatalf("first binding error = %q, want unchanged a error", got.CertificateBindings[0].LastError)
	}
	if got.CertificateBindings[1].LastError != "b error" {
		t.Fatalf("second binding error = %q, want b error", got.CertificateBindings[1].LastError)
	}
}

func TestIgnorableLockCleanupErrorIsHiddenFromCertificateOverview(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		caddyCertsDir = oldCaddyCertsDir
	})

	site := Site{
		Name:    "multi-domain",
		Domain:  "a.example.com, b.example.com",
		Backend: "http://127.0.0.1:8080",
		CertificateBindings: []CertificateBinding{
			{
				Domain:      "a.example.com",
				Mode:        "selected",
				Provider:    "zerossl",
				LastError:   "remove /data/caddy/locks/issue_cert_a.example.com.lock: no such file or directory",
				LastErrorAt: time.Now(),
			},
			{
				Domain:      "b.example.com",
				Mode:        "selected",
				Provider:    "google",
				LastError:   "real b error",
				LastErrorAt: time.Now(),
			},
		},
	}
	if err := saveJSONFile(filepath.Join(tmp, "multi-domain.json"), site); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	certs := npmCertificates()
	byName := map[string]npmCertificate{}
	for _, cert := range certs {
		byName[cert.NiceName] = cert
	}
	if got := byName["a.example.com"].Meta["last_error"]; got != "" {
		t.Fatalf("a.example.com last_error = %#v, want hidden", got)
	}
	if got := byName["a.example.com"].Meta["status"]; got != "pending" {
		t.Fatalf("a.example.com status = %#v, want pending", got)
	}
	if got := byName["b.example.com"].Meta["last_error"]; got != "real b error" {
		t.Fatalf("b.example.com last_error = %#v, want real b error", got)
	}
	if got := byName["b.example.com"].Meta["status"]; got != "failed" {
		t.Fatalf("b.example.com status = %#v, want failed", got)
	}
}

func TestIssuedCertificateDoesNotExposeStaleLastError(t *testing.T) {
	cert := certOverviewToNPM(CertOverview{
		Domain:      "issued.example.com",
		Status:      "issued",
		LastError:   "stale error",
		LastErrorAt: time.Now(),
	})

	if got := cert.Meta["last_error"]; got != "" {
		t.Fatalf("last_error = %#v, want hidden for issued certificate", got)
	}
	if got := cert.Meta["last_error_at"]; !got.(time.Time).IsZero() {
		t.Fatalf("last_error_at = %#v, want zero for issued certificate", got)
	}
}

func TestCertsHandlerDoesNotExposeStaleErrorsForIssuedCertificates(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		caddyCertsDir = oldCaddyCertsDir
	})

	site := Site{
		Name:        "site",
		Domain:      "issued.example.com",
		Backend:     "http://127.0.0.1:8080",
		LastError:   "stale error",
		LastErrorAt: time.Now(),
	}
	if err := saveJSONFile(filepath.Join(tmp, "site.json"), site); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}
	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "issued.example.com", "issued.example.com.crt"), "issued.example.com", "issued.example.com")

	req := httptest.NewRequest(http.MethodGet, "/certs", nil)
	rr := httptest.NewRecorder()
	certsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	var rows []CertOverview
	if err := json.Unmarshal(rr.Body.Bytes(), &rows); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1: %#v", len(rows), rows)
	}
	if rows[0].LastError != "" || !rows[0].LastErrorAt.IsZero() {
		t.Fatalf("last error = %q/%v, want hidden for issued certificate", rows[0].LastError, rows[0].LastErrorAt)
	}
}

func TestCertsHandlerShowsIssuedCertificateFromCommonName(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		caddyCertsDir = oldCaddyCertsDir
	})

	site := Site{
		Name:    "site",
		Domain:  "common-name.example.com",
		Backend: "http://127.0.0.1:8080",
	}
	if err := saveJSONFile(filepath.Join(tmp, "site.json"), site); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}
	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "common-name.example.com", "common-name.example.com.crt"), "common-name.example.com")

	req := httptest.NewRequest(http.MethodGet, "/certs", nil)
	rr := httptest.NewRecorder()
	certsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	var rows []CertOverview
	if err := json.Unmarshal(rr.Body.Bytes(), &rows); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1: %#v", len(rows), rows)
	}
	if rows[0].Domain != "common-name.example.com" || rows[0].Status != "issued" {
		t.Fatalf("row = %#v, want issued common-name.example.com", rows[0])
	}
}

func TestCertsHandlerShowsIssuedCertificateWithoutSiteMetadata(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		caddyCertsDir = oldCaddyCertsDir
	})
	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "orphan.example.com", "orphan.example.com.crt"), "orphan.example.com", "orphan.example.com")

	req := httptest.NewRequest(http.MethodGet, "/certs", nil)
	rr := httptest.NewRecorder()
	certsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	var rows []CertOverview
	if err := json.Unmarshal(rr.Body.Bytes(), &rows); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1: %#v", len(rows), rows)
	}
	if rows[0].Domain != "orphan.example.com" || rows[0].Status != "issued" || !rows[0].Issued {
		t.Fatalf("row = %#v, want issued orphan.example.com", rows[0])
	}
}

func TestCertsHandlerShowsPendingAutoCertificateFromProxyHost(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		caddyCertsDir = oldCaddyCertsDir
	})

	site := Site{
		Name:            "auto-proxy",
		Domain:          "auto.example.com",
		Backend:         "http://127.0.0.1:8080",
		CertificateMode: "auto",
		CertificateBindings: []CertificateBinding{
			{Domain: "auto.example.com", Mode: "auto"},
		},
	}
	if err := saveJSONFile(filepath.Join(tmp, "auto-proxy.json"), site); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/certs", nil)
	rr := httptest.NewRecorder()
	certsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	var rows []CertOverview
	if err := json.Unmarshal(rr.Body.Bytes(), &rows); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1: %#v", len(rows), rows)
	}
	if rows[0].Domain != "auto.example.com" || rows[0].Status != "pending" || rows[0].SignMethod != "HTTP-01" {
		t.Fatalf("row = %#v, want pending HTTP-01 auto.example.com", rows[0])
	}
	if len(rows[0].LinkedSites) != 1 || rows[0].LinkedSites[0] != "auto-proxy" {
		t.Fatalf("linked sites = %#v, want auto-proxy", rows[0].LinkedSites)
	}
}

func TestNPMCertificatesShowsPendingAutoCertificateFromProxyHost(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		caddyCertsDir = oldCaddyCertsDir
	})

	site := Site{
		Name:            "auto-proxy",
		Domain:          "auto.example.com",
		Backend:         "http://127.0.0.1:8080",
		CertificateMode: "auto",
		CertificateBindings: []CertificateBinding{
			{Domain: "auto.example.com", Mode: "auto"},
		},
	}
	if err := saveJSONFile(filepath.Join(tmp, "auto-proxy.json"), site); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	certs := npmCertificates()
	if len(certs) != 1 {
		t.Fatalf("certs len = %d, want 1: %#v", len(certs), certs)
	}
	if certs[0].NiceName != "auto.example.com" || certs[0].Meta["status"] != "pending" {
		t.Fatalf("cert = %#v, want pending auto.example.com", certs[0])
	}
	if len(certs[0].ProxyHosts) != 1 || certs[0].ProxyHosts[0].DomainNames[0] != "auto.example.com" {
		t.Fatalf("proxy hosts = %#v, want auto.example.com usage", certs[0].ProxyHosts)
	}
}

func TestNPMCertificatesShowsIssuedAutoCertificateFromProxyHost(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		caddyCertsDir = oldCaddyCertsDir
	})

	site := Site{
		Name:            "auto-proxy",
		Domain:          "auto.example.com",
		Backend:         "http://127.0.0.1:8080",
		CertificateMode: "auto",
		CertificateBindings: []CertificateBinding{
			{Domain: "auto.example.com", Mode: "auto"},
		},
	}
	if err := saveJSONFile(filepath.Join(tmp, "auto-proxy.json"), site); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}
	writeTestCert(t, filepath.Join(caddyCertsDir, "acme-v02.api.letsencrypt.org-directory", "auto.example.com", "auto.example.com.crt"), "auto.example.com", "auto.example.com")

	certs := npmCertificates()
	if len(certs) != 1 {
		t.Fatalf("certs len = %d, want 1: %#v", len(certs), certs)
	}
	if certs[0].NiceName != "auto.example.com" || certs[0].Meta["status"] != "issued" {
		t.Fatalf("cert = %#v, want issued auto.example.com", certs[0])
	}
	if len(certs[0].ProxyHosts) != 1 || certs[0].ProxyHosts[0].DomainNames[0] != "auto.example.com" {
		t.Fatalf("proxy hosts = %#v, want auto.example.com usage", certs[0].ProxyHosts)
	}
}

func TestNPMProxyHostAutoCertificateModeCreatesCertificateOverviewRow(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	oldCaddyCertsDir := caddyCertsDir
	sitesDir = tmp
	caddyCertsDir = filepath.Join(tmp, "certs")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		caddyCertsDir = oldCaddyCertsDir
	})

	host := npmProxyHost{
		DomainNames:   []string{"auto.example.com"},
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		CertificateID: -1,
		SSLForced:     true,
		Enabled:       true,
		Meta: map[string]any{
			"certificate_bindings": []map[string]any{
				{"domain": "auto.example.com", "mode": "auto"},
			},
		},
	}
	site, err := npmProxyHostToSite(host, "auto-proxy")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}
	if err := saveJSONFile(filepath.Join(tmp, "auto-proxy.json"), site); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	certs := npmCertificates()
	if len(certs) != 1 {
		t.Fatalf("certs len = %d, want 1: %#v", len(certs), certs)
	}
	if certs[0].NiceName != "auto.example.com" || certs[0].Meta["status"] != "pending" {
		t.Fatalf("cert = %#v, want pending auto.example.com", certs[0])
	}
}

func TestCertificateIssuerNameUsesOrganizationAndCommonName(t *testing.T) {
	cert := &x509.Certificate{
		Issuer: pkix.Name{
			Organization: []string{"Let's Encrypt"},
			CommonName:   "YE2",
		},
	}
	if got := certificateIssuerName(cert); got != "Let's Encrypt（YE2）" {
		t.Fatalf("certificateIssuerName() = %q, want Let's Encrypt（YE2）", got)
	}
}

func TestCertificateIssuerNameFallsBackToCommonName(t *testing.T) {
	cert := &x509.Certificate{
		Issuer: pkix.Name{CommonName: "Caddy Local Authority - ECC Intermediate"},
	}
	if got := certificateIssuerName(cert); got != "Caddy Local Authority - ECC Intermediate" {
		t.Fatalf("certificateIssuerName() = %q, want CN fallback", got)
	}
}

func TestHandleCertLogLineIgnoresLockCleanupNoise(t *testing.T) {
	tmp := t.TempDir()
	oldSitesDir := sitesDir
	sitesDir = tmp
	t.Cleanup(func() { sitesDir = oldSitesDir })

	site := Site{
		Name:    "site",
		Domain:  "a.example.com",
		Backend: "http://127.0.0.1:8080",
		CertificateBindings: []CertificateBinding{
			{Domain: "a.example.com", Mode: "selected"},
		},
	}
	if err := saveJSONFile(filepath.Join(tmp, "site.json"), site); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	handleCertLogLine([]byte(`{"level":"error","logger":"tls.obtain","identifier":"a.example.com","error":"remove /data/caddy/locks/issue_cert_a.example.com.lock: no such file or directory"}`))

	var got Site
	if err := loadJSONFile(filepath.Join(tmp, "site.json"), &got); err != nil {
		t.Fatalf("loadJSONFile error = %v", err)
	}
	if got.LastError != "" || got.CertificateBindings[0].LastError != "" {
		t.Fatalf("site after ignored log = %#v, want no persisted error", got)
	}
}

func TestProxyHostVisibleCertErrorIgnoresSelectedCertificateBindingError(t *testing.T) {
	site := Site{
		Name:    "selected-wildcard",
		Domain:  "a.domain.com",
		Backend: "http://127.0.0.1:8080",
		CertificateBindings: []CertificateBinding{
			{
				Domain:            "a.domain.com",
				Mode:              "selected",
				CertificateID:     stableID("cert:*.domain.com"),
				CertificateDomain: "*.domain.com",
				LastError:         "old exact certificate error",
				LastErrorAt:       time.Now(),
			},
		},
	}
	if msg, at := proxyHostVisibleCertError(site); msg != "" || !at.IsZero() {
		t.Fatalf("proxyHostVisibleCertError = %q/%v, want no stale selected binding error", msg, at)
	}
}

func TestProxyHostEnabledReflectsDisabledSite(t *testing.T) {
	site := Site{
		Name:     "disabled-proxy",
		Domain:   "disabled.example.com",
		Backend:  "http://127.0.0.1:8080",
		Disabled: true,
	}

	host := siteToNPMProxyHost(site, nil)

	if host.Enabled {
		t.Fatalf("Enabled = true, want false for disabled site")
	}
}

func TestProxyHostExposesServiceNameAndCertificateError(t *testing.T) {
	site := Site{
		Name:        "alist-proxy",
		ServiceName: "AList",
		Domain:      "alist.example.com",
		Backend:     "http://127.0.0.1:5244",
		LastError:   "certificate failed",
		LastErrorAt: time.Now(),
	}

	host := siteToNPMProxyHost(site, nil)

	if host.ServiceName != "AList" {
		t.Fatalf("ServiceName = %q, want AList", host.ServiceName)
	}
	if got := metaString(host.Meta, "last_error", "lastError"); got != "certificate failed" {
		t.Fatalf("last_error = %q, want certificate failed", got)
	}
}

func TestNPMProxyHostToSitePersistsServiceName(t *testing.T) {
	host := npmProxyHost{
		ServiceName:   "Home Assistant",
		DomainNames:   []string{"ha.example.com"},
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8123,
		Enabled:       true,
		Meta:          map[string]any{},
	}

	site, err := npmProxyHostToSite(host, "ha")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}
	if site.ServiceName != "Home Assistant" {
		t.Fatalf("ServiceName = %q, want Home Assistant", site.ServiceName)
	}
}

func TestNPMProxyHostToSitePersistsDisabledState(t *testing.T) {
	host := npmProxyHost{
		DomainNames:   []string{"disabled.example.com"},
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		Enabled:       false,
		Meta:          map[string]any{},
	}

	site, err := npmProxyHostToSite(host, "disabled-proxy")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}

	if !site.Disabled {
		t.Fatalf("Disabled = false, want true")
	}
}

func TestNPMProxyHostRoundTripsUpstreamInsecureSkipVerify(t *testing.T) {
	host := npmProxyHost{
		DomainNames:                []string{"self-signed.example.com"},
		ForwardScheme:              "https",
		ForwardHost:                "127.0.0.1",
		ForwardPort:                8443,
		SSLForced:                  true,
		Enabled:                    true,
		Meta:                       map[string]any{},
		UpstreamInsecureSkipVerify: true,
	}

	site, err := npmProxyHostToSite(host, "self-signed")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}
	if !site.UpstreamInsecureSkipVerify {
		t.Fatalf("UpstreamInsecureSkipVerify = false, want true")
	}

	out := siteToNPMProxyHost(site, nil)
	if !out.UpstreamInsecureSkipVerify {
		t.Fatalf("round-trip UpstreamInsecureSkipVerify = false, want true")
	}
}

func TestRenderSiteSkipsDisabledSite(t *testing.T) {
	conf, err := renderSite(Site{
		Name:     "disabled-proxy",
		Domain:   "disabled.example.com",
		Backend:  "http://127.0.0.1:8080",
		Disabled: true,
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	if conf != "" {
		t.Fatalf("renderSite = %q, want empty config for disabled site", conf)
	}
}

func TestRenderSiteSupportsUpstreamInsecureSkipVerify(t *testing.T) {
	conf, err := renderSite(Site{
		Name:                       "self-signed-upstream",
		Domain:                     "self-signed.example.com",
		Backend:                    "https://127.0.0.1:8443",
		Headers:                    "X-Legacy-Header: should-not-render",
		NoTLS:                      true,
		UpstreamInsecureSkipVerify: true,
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	for _, want := range []string{
		"reverse_proxy https://127.0.0.1:8443 {",
		"transport http {",
		"tls_insecure_skip_verify",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
	if strings.Contains(conf, "X-Legacy-Header") {
		t.Fatalf("renderSite = %q, should not render removed advanced config headers", conf)
	}

	httpConf, err := renderSite(Site{
		Name:                       "http-upstream",
		Domain:                     "http-upstream.example.com",
		Backend:                    "http://127.0.0.1:8080",
		NoTLS:                      true,
		UpstreamInsecureSkipVerify: true,
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	if strings.Contains(httpConf, "tls_insecure_skip_verify") {
		t.Fatalf("renderSite = %q, should not skip TLS verify for HTTP upstream", httpConf)
	}
}

func TestRenderSiteSupportsMultipleUpstreamsAndLoadBalancingPolicy(t *testing.T) {
	conf, err := renderSite(Site{
		Name:                "multi-upstream",
		Domain:              "app.example.com",
		Backend:             "http://127.0.0.1:8080",
		LoadBalancingPolicy: "round_robin",
		Upstreams: []ProxyUpstream{
			{ForwardScheme: "http", ForwardHost: "127.0.0.1", ForwardPort: 8080},
			{ForwardScheme: "http", ForwardHost: "127.0.0.1", ForwardPort: 8081},
		},
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	for _, want := range []string{
		"reverse_proxy http://127.0.0.1:8080 http://127.0.0.1:8081 {",
		"lb_policy round_robin",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
}

func TestRenderSiteSupportsWeightedRoundRobin(t *testing.T) {
	conf, err := renderSite(Site{
		Name:                "weighted-upstream",
		Domain:              "app.example.com",
		Backend:             "http://127.0.0.1:8080",
		LoadBalancingPolicy: "weighted_round_robin",
		Upstreams: []ProxyUpstream{
			{ForwardScheme: "http", ForwardHost: "127.0.0.1", ForwardPort: 8080, Weight: 3},
			{ForwardScheme: "http", ForwardHost: "127.0.0.1", ForwardPort: 8081, Weight: 1},
		},
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	if !strings.Contains(conf, "lb_policy weighted_round_robin 3 1") {
		t.Fatalf("renderSite = %q, want weighted round robin policy", conf)
	}
}

func TestRenderSiteSupportsLocationMultipleUpstreams(t *testing.T) {
	conf, err := renderSite(Site{
		Name:    "location-upstreams",
		Domain:  "app.example.com",
		Backend: "http://127.0.0.1:8080",
		Locations: []ProxyLocation{
			{
				Path:                "/api",
				Backend:             "http://127.0.0.1:9000",
				LoadBalancingPolicy: "least_conn",
				Upstreams: []ProxyUpstream{
					{ForwardScheme: "http", ForwardHost: "127.0.0.1", ForwardPort: 9000},
					{ForwardScheme: "http", ForwardHost: "127.0.0.1", ForwardPort: 9001},
				},
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	for _, want := range []string{
		"handle_path /api {",
		"reverse_proxy http://127.0.0.1:9000 http://127.0.0.1:9001 {",
		"lb_policy least_conn",
		"header_up Host {upstream_hostport}",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
}

func TestNPMProxyHostRoundTripsMultipleUpstreams(t *testing.T) {
	host := npmProxyHost{
		DomainNames:         []string{"app.example.com"},
		ForwardScheme:       "http",
		ForwardHost:         "127.0.0.1",
		ForwardPort:         8080,
		LoadBalancingPolicy: "least_conn",
		Upstreams: []npmProxyUpstream{
			{ForwardScheme: "http", ForwardHost: "10.0.0.1", ForwardPort: 8080, Weight: 2},
			{ForwardScheme: "http", ForwardHost: "10.0.0.2", ForwardPort: 8080, Weight: 1},
		},
		Enabled: true,
		Meta:    map[string]any{},
	}

	site, err := npmProxyHostToSite(host, "app")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}
	if site.Backend != "http://10.0.0.1:8080" || len(site.Upstreams) != 2 || site.LoadBalancingPolicy != "least_conn" {
		t.Fatalf("site upstreams = %q/%#v/%q", site.Backend, site.Upstreams, site.LoadBalancingPolicy)
	}
	out := siteToNPMProxyHost(site, nil)
	if out.ForwardHost != "10.0.0.1" || len(out.Upstreams) != 2 || out.Upstreams[0].Weight != 2 || out.LoadBalancingPolicy != "least_conn" {
		t.Fatalf("round-trip proxy host = %#v", out)
	}
}

func TestValidateSiteProxyConfigRejectsMixedSchemes(t *testing.T) {
	err := validateSiteProxyConfig(Site{
		Backend: "http://127.0.0.1:8080",
		Upstreams: []ProxyUpstream{
			{ForwardScheme: "http", ForwardHost: "127.0.0.1", ForwardPort: 8080},
			{ForwardScheme: "https", ForwardHost: "127.0.0.1", ForwardPort: 8443},
		},
	})
	if err == nil {
		t.Fatalf("validateSiteProxyConfig error = nil, want mixed scheme error")
	}
}

func TestRenderNoTLSCustomListenPortsUseExplicitHTTP(t *testing.T) {
	site := Site{
		Name:    "custom-http-port",
		Domain:  "test.example.net:8080, test.example.net:8443",
		Backend: "http://127.0.0.1:8080",
		NoTLS:   true,
	}

	conf, err := renderSite(site, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	for _, want := range []string{
		"http://test.example.net:8080, http://test.example.net:8443 {\n",
		"reverse_proxy http://127.0.0.1:8080",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
	if strings.HasPrefix(conf, ":8443 {") || strings.Contains(conf, "\n:8443 {") || strings.Contains(conf, ", :8443") {
		t.Fatalf("renderSite = %q, should not render ambiguous bare custom port", conf)
	}
}

func TestRenderSiteUsesCustomCertificateFiles(t *testing.T) {
	conf, err := renderSite(Site{
		Name:           "custom-cert-site",
		Domain:         "custom.example.com",
		Backend:        "http://127.0.0.1:8080",
		CustomCertFile: "/config/custom-certs/1/certificate.pem",
		CustomKeyFile:  "/config/custom-certs/1/key.pem",
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}

	want := `tls "/config/custom-certs/1/certificate.pem" "/config/custom-certs/1/key.pem"`
	if !strings.Contains(conf, want) {
		t.Fatalf("renderSite = %q, want custom tls directive %q", conf, want)
	}
}

func TestRenderTLSBlockSupportsHurricaneElectric(t *testing.T) {
	conf := renderTLSBlock(Credential{Provider: "he", HEAPIKey: "he-key"}, IssuerConfig{Provider: "letsencrypt"}, "    ")
	for _, want := range []string{
		"    tls {\n",
		"        dns he {\n",
		"            api_key \"he-key\"\n",
		"        }\n",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderTLSBlock = %q, want %q", conf, want)
		}
	}
}

func TestAutheliaSettingsDefaultsAndRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	defaults, err := loadAutheliaConfig()
	if err != nil {
		t.Fatalf("loadAutheliaConfig defaults error = %v", err)
	}
	if defaults.Enabled || defaults.Upstream != "http://authelia:9091" || defaults.URI != "/api/authz/forward-auth" || !defaults.FailOpen {
		t.Fatalf("defaults = %#v, want disabled Authelia defaults with fail-open", defaults)
	}

	in := AutheliaConfig{
		Enabled:     true,
		Upstream:    "https://authelia.example.com",
		URI:         "/api/authz/verify",
		CopyHeaders: []string{"Remote-User", "Remote-Email", "Remote-User"},
		FailOpen:    false,
	}
	if err := saveAutheliaConfig(in); err != nil {
		t.Fatalf("saveAutheliaConfig error = %v", err)
	}
	out, err := loadAutheliaConfig()
	if err != nil {
		t.Fatalf("loadAutheliaConfig saved error = %v", err)
	}
	if !out.Enabled || out.Upstream != in.Upstream || out.URI != in.URI || out.FailOpen {
		t.Fatalf("round-trip AutheliaConfig = %#v", out)
	}
	if got := strings.Join(out.CopyHeaders, ","); got != "Remote-User,Remote-Email" {
		t.Fatalf("CopyHeaders = %q, want deduped stable order", got)
	}
}

func TestAutheliaSettingsRejectInvalidValues(t *testing.T) {
	cases := []AutheliaConfig{
		{Enabled: true, Upstream: "ftp://authelia:9091"},
		{Enabled: true, Upstream: "http://authelia:9091/path"},
		{Enabled: true, Upstream: "http://authelia:9091", URI: "api/authz/forward-auth"},
		{Enabled: true, Upstream: "http://authelia:9091", URI: "/api/authz\nforward-auth"},
		{Enabled: true, Upstream: "http://authelia:9091", CopyHeaders: []string{"Remote User"}},
	}
	for _, cfg := range cases {
		if _, err := normalizeAutheliaConfig(cfg); err == nil {
			t.Fatalf("normalizeAutheliaConfig(%#v) error = nil, want error", cfg)
		}
	}
}

func TestAuthentikSettingsDefaultsAndRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	defaults, err := loadAuthentikConfig()
	if err != nil {
		t.Fatalf("loadAuthentikConfig defaults error = %v", err)
	}
	if defaults.Enabled || defaults.Upstream != "http://authentik-server:9000" || defaults.URI != "/outpost.goauthentik.io/auth/caddy" {
		t.Fatalf("defaults = %#v, want disabled Authentik defaults", defaults)
	}

	in := AuthentikConfig{
		Enabled:     true,
		Upstream:    "https://authentik.example.com",
		URI:         "/outpost.goauthentik.io/auth/caddy",
		CopyHeaders: []string{"X-authentik-username", "X-authentik-email", "X-authentik-username"},
	}
	if err := saveAuthentikConfig(in); err != nil {
		t.Fatalf("saveAuthentikConfig error = %v", err)
	}
	out, err := loadAuthentikConfig()
	if err != nil {
		t.Fatalf("loadAuthentikConfig saved error = %v", err)
	}
	if !out.Enabled || out.Upstream != in.Upstream || out.URI != in.URI {
		t.Fatalf("round-trip AuthentikConfig = %#v", out)
	}
	if got := strings.Join(out.CopyHeaders, ","); got != "X-authentik-username,X-authentik-email" {
		t.Fatalf("CopyHeaders = %q, want deduped stable order", got)
	}
}

func TestAuthentikSettingsRejectInvalidValues(t *testing.T) {
	cases := []AuthentikConfig{
		{Enabled: true, Upstream: "ftp://authentik:9000"},
		{Enabled: true, Upstream: "http://authentik:9000/path"},
		{Enabled: true, Upstream: "http://authentik:9000", URI: "outpost.goauthentik.io/auth/caddy"},
		{Enabled: true, Upstream: "http://authentik:9000", URI: "/outpost.goauthentik.io/auth/caddy\nx"},
		{Enabled: true, Upstream: "http://authentik:9000", CopyHeaders: []string{"X authentik username"}},
	}
	for _, cfg := range cases {
		if _, err := normalizeAuthentikConfig(cfg); err == nil {
			t.Fatalf("normalizeAuthentikConfig(%#v) error = nil, want error", cfg)
		}
	}
}

func TestRenderSiteSkipsDisabledForwardAuth(t *testing.T) {
	conf, err := renderSite(Site{
		Name:    "authelia-disabled",
		Domain:  "auth.example.com",
		Backend: "http://127.0.0.1:8080",
		ForwardAuth: ForwardAuthConfig{
			Enabled:  false,
			Upstream: "http://authelia:9091",
		},
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	if strings.Contains(conf, "forward_auth") {
		t.Fatalf("renderSite = %q, want no forward_auth block", conf)
	}
}

func TestRenderSiteUsesCustomAutheliaForwardAuth(t *testing.T) {
	conf, err := renderSite(Site{
		Name:    "authelia",
		Domain:  "auth.example.com",
		Backend: "http://127.0.0.1:8080",
		ForwardAuth: ForwardAuthConfig{
			Enabled:  true,
			Upstream: "http://authelia:9091",
		},
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	for _, want := range []string{
		"forward_auth http://authelia:9091 {",
		"uri /api/authz/forward-auth",
		"copy_headers Remote-User Remote-Groups Remote-Email Remote-Name",
		"header_up X-Forwarded-Method {method}",
		"reverse_proxy http://127.0.0.1:8080",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
}

func TestRenderSiteUsesPathScopedForwardAuth(t *testing.T) {
	conf, err := renderSite(Site{
		Name:    "authelia-path",
		Domain:  "auth.example.com",
		Path:    "/admin/*",
		Backend: "http://127.0.0.1:8080",
		ForwardAuth: ForwardAuthConfig{
			Enabled:  true,
			Upstream: "http://authelia:9091",
		},
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	for _, want := range []string{
		"handle /admin/* {",
		"forward_auth http://authelia:9091 {",
		"header_up X-Forwarded-Host {host}",
		"reverse_proxy http://127.0.0.1:8080",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
}

func TestRenderSiteUsesAuthentikForwardAuth(t *testing.T) {
	conf, err := renderSite(Site{
		Name:    "authentik",
		Domain:  "auth.example.com",
		Backend: "http://127.0.0.1:8080",
		ForwardAuth: ForwardAuthConfig{
			Enabled:  true,
			Provider: "authentik",
			Upstream: "https://authentik.example.com",
		},
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	for _, want := range []string{
		"handle /outpost.goauthentik.io/* {",
		"reverse_proxy https://authentik.example.com {",
		"header_up Host {upstream_hostport}",
		"forward_auth https://authentik.example.com {",
		"uri /outpost.goauthentik.io/auth/caddy",
		"copy_headers X-authentik-username",
		"reverse_proxy http://127.0.0.1:8080",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
}

func TestRenderSiteUsesPathScopedAuthentikForwardAuth(t *testing.T) {
	conf, err := renderSite(Site{
		Name:    "authentik-path",
		Domain:  "auth.example.com",
		Path:    "/admin/*",
		Backend: "http://127.0.0.1:8080",
		ForwardAuth: ForwardAuthConfig{
			Enabled:  true,
			Provider: "authentik",
			Upstream: "http://authentik:9000",
		},
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	if !strings.Contains(conf, "    handle /outpost.goauthentik.io/* {\n        reverse_proxy http://authentik:9000\n    }\n") {
		t.Fatalf("renderSite = %q, want root-level Authentik outpost route", conf)
	}
	if strings.Contains(conf, "route {\n            handle /outpost.goauthentik.io/*") {
		t.Fatalf("renderSite = %q, want no nested Authentik outpost route inside path handler", conf)
	}
	if !strings.Contains(conf, "    handle /admin/* {\n        forward_auth http://authentik:9000 {\n            uri /outpost.goauthentik.io/auth/caddy\n") {
		t.Fatalf("renderSite = %q, want path-scoped Authentik forward_auth", conf)
	}
}

func TestRenderSiteUsesGlobalAutheliaFailOpen(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	if err := saveAutheliaConfig(AutheliaConfig{Enabled: true, Upstream: "https://authelia.example.com", FailOpen: true}); err != nil {
		t.Fatalf("saveAutheliaConfig error = %v", err)
	}
	conf, err := renderSite(Site{
		Name:    "authelia-global",
		Domain:  "auth.example.com",
		Backend: "http://127.0.0.1:8080",
		ForwardAuth: ForwardAuthConfig{
			Enabled:   true,
			UseGlobal: true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	for _, want := range []string{
		"reverse_proxy https://authelia.example.com",
		"rewrite /api/authz/forward-auth",
		"request_header Remote-User {rp.header.Remote-User}",
		"@authelia_unavailable expression",
		"reverse_proxy http://127.0.0.1:8080",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
}

func TestRenderSiteRejectsDisabledGlobalAuthelia(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	if _, err := renderSite(Site{
		Name:    "authelia-global-disabled",
		Domain:  "auth.example.com",
		Backend: "http://127.0.0.1:8080",
		ForwardAuth: ForwardAuthConfig{
			Enabled:   true,
			UseGlobal: true,
		},
	}, nil); err == nil {
		t.Fatalf("renderSite error = nil, want global Authelia disabled error")
	}
}

func TestNPMProxyHostUsesGlobalAuthelia(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	if err := saveAutheliaConfig(AutheliaConfig{Enabled: true}); err != nil {
		t.Fatalf("saveAutheliaConfig error = %v", err)
	}
	host := npmProxyHost{
		DomainNames:   []string{"auth.example.com"},
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		SSLForced:     true,
		Enabled:       true,
		Meta:          map[string]any{},
		ForwardAuth: ForwardAuthConfig{
			Enabled:   true,
			UseGlobal: true,
		},
	}

	site, err := npmProxyHostToSite(host, "authelia-global")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}
	if !site.ForwardAuth.Enabled || !site.ForwardAuth.UseGlobal || site.ForwardAuth.Upstream != "" {
		t.Fatalf("ForwardAuth = %#v, want global mode only", site.ForwardAuth)
	}
}

func TestNPMProxyHostUsesGlobalAuthentik(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	if err := saveAuthentikConfig(AuthentikConfig{Enabled: true}); err != nil {
		t.Fatalf("saveAuthentikConfig error = %v", err)
	}
	host := npmProxyHost{
		DomainNames:   []string{"auth.example.com"},
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		Enabled:       true,
		Meta:          map[string]any{},
		ForwardAuth: ForwardAuthConfig{
			Enabled:   true,
			Provider:  "authentik",
			UseGlobal: true,
		},
	}

	site, err := npmProxyHostToSite(host, "authentik-global")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}
	if !site.ForwardAuth.Enabled || !site.ForwardAuth.UseGlobal || site.ForwardAuth.Provider != "authentik" {
		t.Fatalf("ForwardAuth = %#v, want global Authentik mode only", site.ForwardAuth)
	}
}

func TestNPMProxyHostClearsDisabledForwardAuth(t *testing.T) {
	host := npmProxyHost{
		DomainNames:   []string{"public.example.com"},
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		Enabled:       true,
		Meta:          map[string]any{},
		ForwardAuth: ForwardAuthConfig{
			Enabled:   false,
			Provider:  "authelia",
			Upstream:  "http://authelia:9091",
			UseGlobal: true,
		},
	}

	site, err := npmProxyHostToSite(host, "public")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}
	if site.ForwardAuth.Enabled || site.ForwardAuth.Provider != "" || site.ForwardAuth.Upstream != "" || site.ForwardAuth.URI != "" || len(site.ForwardAuth.CopyHeaders) != 0 || site.ForwardAuth.UseGlobal {
		t.Fatalf("ForwardAuth = %#v, want zero value when disabled", site.ForwardAuth)
	}
	out := siteToNPMProxyHost(site, nil)
	if out.ForwardAuth.Enabled || out.ForwardAuth.Provider != "" || out.ForwardAuth.UseGlobal {
		t.Fatalf("round-trip ForwardAuth = %#v, want disabled without stale fields", out.ForwardAuth)
	}
	conf, err := renderSite(site, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	if strings.Contains(conf, "forward_auth") {
		t.Fatalf("renderSite = %q, want no forward_auth block", conf)
	}
}

func TestNPMProxyHostSupportsAdditionalListenPorts(t *testing.T) {
	host := npmProxyHost{
		DomainNames:   []string{"test.example.net"},
		ListenPort:    443,
		ListenPorts:   []int{443, 11111},
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		SSLForced:     true,
		Enabled:       true,
		Meta:          map[string]any{},
	}

	site, err := npmProxyHostToSite(host, "multi-port")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}
	if site.Domain != "test.example.net:443, test.example.net:11111" {
		t.Fatalf("Domain = %q, want 443 and 11111 listen addresses", site.Domain)
	}
	conf, err := renderSite(site, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	if !strings.Contains(conf, "test.example.net:443, test.example.net:11111 {") {
		t.Fatalf("renderSite = %q, want combined multi-port site address", conf)
	}

	out := siteToNPMProxyHost(site, nil)
	if out.ListenPort != 443 || len(out.ListenPorts) != 2 || out.ListenPorts[0] != 443 || out.ListenPorts[1] != 11111 {
		t.Fatalf("listen ports = %d/%#v, want 443/[443 11111]", out.ListenPort, out.ListenPorts)
	}
	if len(out.DomainNames) != 1 || out.DomainNames[0] != "test.example.net" {
		t.Fatalf("DomainNames = %#v, want clean domain once", out.DomainNames)
	}
}

func TestNPMProxyHostKeepsDefaultListenPortWithAdditionalPorts(t *testing.T) {
	host := npmProxyHost{
		DomainNames:   []string{"test.example.net"},
		ListenPort:    443,
		ListenPorts:   []int{443, 11111},
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		SSLForced:     true,
		Enabled:       true,
		Meta:          map[string]any{},
	}
	site, err := npmProxyHostToSite(host, "multi-port")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}
	out := siteToNPMProxyHost(site, nil)
	if out.ListenPort != 443 {
		t.Fatalf("ListenPort = %d, want default 443 preserved", out.ListenPort)
	}
	if len(out.ListenPorts) != 2 || out.ListenPorts[0] != 443 || out.ListenPorts[1] != 11111 {
		t.Fatalf("ListenPorts = %#v, want [443 11111]", out.ListenPorts)
	}
	if !strings.Contains(site.Domain, "test.example.net:443") || !strings.Contains(site.Domain, "test.example.net:11111") {
		t.Fatalf("site.Domain = %q, want both 443 and 11111", site.Domain)
	}
}

func TestNPMProxyHostRoundTripsForwardAuth(t *testing.T) {
	host := npmProxyHost{
		DomainNames:   []string{"auth.example.com"},
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		SSLForced:     true,
		Enabled:       true,
		Meta:          map[string]any{},
		ForwardAuth: ForwardAuthConfig{
			Enabled:     true,
			Provider:    "authelia",
			Upstream:    "https://authelia.example.com",
			URI:         "/api/authz/forward-auth",
			CopyHeaders: []string{"Remote-User", "Remote-Email", "Remote-User"},
		},
	}

	site, err := npmProxyHostToSite(host, "authelia")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}
	if !site.ForwardAuth.Enabled || site.ForwardAuth.Upstream != "https://authelia.example.com" {
		t.Fatalf("ForwardAuth = %#v, want enabled custom upstream", site.ForwardAuth)
	}
	if got := strings.Join(site.ForwardAuth.CopyHeaders, ","); got != "Remote-User,Remote-Email" {
		t.Fatalf("CopyHeaders = %q, want deduped stable order", got)
	}

	out := siteToNPMProxyHost(site, nil)
	if !out.ForwardAuth.Enabled || out.ForwardAuth.URI != "/api/authz/forward-auth" {
		t.Fatalf("round-trip ForwardAuth = %#v", out.ForwardAuth)
	}
}

func TestNPMProxyHostRoundTripsAuthentikForwardAuth(t *testing.T) {
	host := npmProxyHost{
		DomainNames:   []string{"auth.example.com"},
		ForwardScheme: "http",
		ForwardHost:   "127.0.0.1",
		ForwardPort:   8080,
		SSLForced:     true,
		Enabled:       true,
		Meta:          map[string]any{},
		ForwardAuth: ForwardAuthConfig{
			Enabled:     true,
			Provider:    "authentik",
			Upstream:    "https://authentik.example.com",
			URI:         "/outpost.goauthentik.io/auth/caddy",
			CopyHeaders: []string{"X-authentik-username", "X-authentik-email", "X-authentik-username"},
		},
	}

	site, err := npmProxyHostToSite(host, "authentik")
	if err != nil {
		t.Fatalf("npmProxyHostToSite error = %v", err)
	}
	if !site.ForwardAuth.Enabled || site.ForwardAuth.Provider != "authentik" || site.ForwardAuth.Upstream != "https://authentik.example.com" {
		t.Fatalf("ForwardAuth = %#v, want enabled custom Authentik upstream", site.ForwardAuth)
	}
	if got := strings.Join(site.ForwardAuth.CopyHeaders, ","); got != "X-authentik-username,X-authentik-email" {
		t.Fatalf("CopyHeaders = %q, want deduped stable order", got)
	}

	out := siteToNPMProxyHost(site, nil)
	if !out.ForwardAuth.Enabled || out.ForwardAuth.Provider != "authentik" || out.ForwardAuth.URI != "/outpost.goauthentik.io/auth/caddy" {
		t.Fatalf("round-trip ForwardAuth = %#v", out.ForwardAuth)
	}
}

func TestNPMProxyHostRejectsInvalidForwardAuth(t *testing.T) {
	cases := []ForwardAuthConfig{
		{Enabled: true, Provider: "other", Upstream: "http://authelia:9091"},
		{Enabled: true, Upstream: "ftp://authelia:9091"},
		{Enabled: true, Upstream: "http://authelia:9091/path"},
		{Enabled: true, Upstream: "http://authelia:9091", URI: "api/authz/forward-auth"},
		{Enabled: true, Upstream: "http://authelia:9091", URI: "/api/authz\nforward-auth"},
		{Enabled: true, Upstream: "http://authelia:9091", CopyHeaders: []string{"Remote User"}},
		{Enabled: true, Provider: "authentik", Upstream: "http://authentik:9000/path"},
	}
	for _, cfg := range cases {
		host := npmProxyHost{
			DomainNames:   []string{"auth.example.com"},
			ForwardScheme: "http",
			ForwardHost:   "127.0.0.1",
			ForwardPort:   8080,
			Enabled:       true,
			Meta:          map[string]any{},
			ForwardAuth:   cfg,
		}
		if _, err := npmProxyHostToSite(host, "authelia"); err == nil {
			t.Fatalf("npmProxyHostToSite(%#v) error = nil, want error", cfg)
		}
	}
}

func TestApplyCertificateConfigToSiteUsesCustomCertificate(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	rec := customCertRecord{
		ID:          123,
		Name:        "custom cert",
		DomainNames: []string{"custom.example.com"},
		CertFile:    filepath.Join(tmp, "certificate.pem"),
		KeyFile:     filepath.Join(tmp, "key.pem"),
	}
	if err := saveJSONFile(customCertMeta, []customCertRecord{rec}); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	site := Site{ChallengePref: "dns", CredentialID: "old", Wildcard: true}
	if !applyCertificateConfigToSite(&site, rec.ID) {
		t.Fatalf("applyCertificateConfigToSite returned false")
	}

	if site.CustomCertFile != rec.CertFile || site.CustomKeyFile != rec.KeyFile {
		t.Fatalf("custom files = %q/%q, want %q/%q", site.CustomCertFile, site.CustomKeyFile, rec.CertFile, rec.KeyFile)
	}
	if site.CredentialID != "" || site.Wildcard || site.Issuer.Provider != "" {
		t.Fatalf("site kept ACME state: %#v", site)
	}
}

func TestCustomCertificateSiteReferencesCertificateDomain(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	rec := customCertRecord{
		ID:          123,
		Name:        "custom cert",
		DomainNames: []string{"custom.example.com"},
		CertFile:    filepath.Join(tmp, "certificate.pem"),
		KeyFile:     filepath.Join(tmp, "key.pem"),
	}
	if err := saveJSONFile(customCertMeta, []customCertRecord{rec}); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	site := Site{
		Name:           "custom-cert-site",
		Domain:         "custom.example.com",
		Backend:        "http://127.0.0.1:8080",
		CustomCertFile: rec.CertFile,
		CustomKeyFile:  rec.KeyFile,
	}

	if !siteReferencesCertificateDomain(site, "custom.example.com") {
		t.Fatalf("siteReferencesCertificateDomain() = false, want true")
	}
}

func TestStampNPMStreamPreservesEnabledState(t *testing.T) {
	enabled := npmStream{Enabled: true, TCPForwarding: true}
	stampNPMStream(&enabled, "", true)
	if !enabled.Enabled {
		t.Fatalf("Enabled = false, want true")
	}

	disabled := npmStream{Enabled: false, TCPForwarding: true}
	stampNPMStream(&disabled, "", true)
	if disabled.Enabled {
		t.Fatalf("Enabled = true, want false")
	}
}

func TestStampNPMAccessListReturnsEmptySlices(t *testing.T) {
	item := npmAccessList{Name: "lan"}
	stampNPMAccessList(&item, "", true)
	if item.Items == nil {
		t.Fatalf("Items = nil, want empty slice")
	}
	if item.Clients == nil {
		t.Fatalf("Clients = nil, want empty slice")
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}
	body := string(data)
	if !strings.Contains(body, `"items":[]`) || !strings.Contains(body, `"clients":[]`) {
		t.Fatalf("json = %s, want items and clients empty arrays", body)
	}
}

func TestLoadNPMAccessListsCountsLinkedProxyHosts(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	oldSitesDir := sitesDir
	oldAccessListPath := accessListPath
	sitesDir = filepath.Join(tmp, "sites")
	accessListPath = filepath.Join(tmp, "access-lists.json")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		accessListPath = oldAccessListPath
	})

	if err := saveJSONFile(accessListPath, []npmAccessList{{ID: 123, Name: "lan"}}); err != nil {
		t.Fatalf("saveJSONFile access list error = %v", err)
	}
	if err := saveJSONFile(filepath.Join(sitesDir, "app.json"), Site{
		Name:         "app",
		Kind:         "proxy",
		Domain:       "app.example.com",
		Backend:      "http://127.0.0.1:8080",
		AccessListID: 123,
	}); err != nil {
		t.Fatalf("saveJSONFile site error = %v", err)
	}

	items, err := loadNPMAccessLists()
	if err != nil {
		t.Fatalf("loadNPMAccessLists error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].ProxyHostCount != 1 {
		t.Fatalf("ProxyHostCount = %d, want 1", items[0].ProxyHostCount)
	}
}

func TestRenderSiteAppliesAccessListRules(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	oldSitesDir := sitesDir
	oldAccessListPath := accessListPath
	sitesDir = filepath.Join(tmp, "sites")
	accessListPath = filepath.Join(tmp, "access-lists.json")
	t.Cleanup(func() {
		sitesDir = oldSitesDir
		accessListPath = oldAccessListPath
	})

	if err := saveJSONFile(accessListPath, []npmAccessList{{
		ID:   123,
		Name: "private",
		Items: []npmAccessListItem{{
			Username: "alice",
			Password: "secret",
		}},
		Clients: []npmAccessClient{{
			Directive: "allow",
			Address:   "192.0.2.0/24",
		}},
	}}); err != nil {
		t.Fatalf("saveJSONFile access list error = %v", err)
	}

	conf, err := renderSite(Site{
		Name:         "app",
		Domain:       "app.example.com",
		Backend:      "http://127.0.0.1:8080",
		AccessListID: 123,
	}, nil)
	if err != nil {
		t.Fatalf("renderSite error = %v", err)
	}
	for _, want := range []string{
		"@access_allow {",
		"not remote_ip 192.0.2.0/24",
		"respond @access_allow 403",
		"@access_auth_required {",
		"remote_ip 192.0.2.0/24",
		"basic_auth @access_auth_required {",
		`"alice" "$2`,
		"request_header -Authorization",
		"reverse_proxy http://127.0.0.1:8080",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("renderSite = %q, want %q", conf, want)
		}
	}
	if strings.Contains(conf, "secret") {
		t.Fatalf("renderSite = %q, should not contain plaintext password", conf)
	}
}

func TestRenderSiteAccessListSatisfyAnyBypassesAuthForAllowedIPs(t *testing.T) {
	var b strings.Builder
	err := writeAccessListBlock(&b, npmAccessList{
		Name:       "private",
		SatisfyAny: true,
		Items: []npmAccessListItem{{
			Username: "alice",
			Password: "secret",
		}},
		Clients: []npmAccessClient{{
			Directive: "allow",
			Address:   "192.0.2.0/24",
		}},
	}, "    ")
	if err != nil {
		t.Fatalf("writeAccessListBlock error = %v", err)
	}
	conf := b.String()
	for _, want := range []string{
		"@access_auth_required {",
		"not remote_ip 192.0.2.0/24",
		"basic_auth @access_auth_required {",
		`"alice" "$2`,
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("config = %q, want %q", conf, want)
		}
	}
	if strings.Contains(conf, "respond @access_allow 403") {
		t.Fatalf("config = %q, should not reject non-allowed clients before auth when satisfy_any is true", conf)
	}
}

func TestAccessListPutPreservesBlankPasswords(t *testing.T) {
	oldHash, err := accessBasicAuthPasswordHash("old-secret")
	if err != nil {
		t.Fatalf("accessBasicAuthPasswordHash error = %v", err)
	}
	next := npmAccessList{
		Items: []npmAccessListItem{{
			Username: "alice",
			Password: "",
		}},
	}
	previous := npmAccessList{
		Items: []npmAccessListItem{{
			Username: "alice",
			Password: oldHash,
		}},
	}

	preserveAccessListPasswords(&next, previous)
	stampNPMAccessList(&next, "", true)

	if next.Items[0].Password != oldHash {
		t.Fatalf("Password = %q, want preserved old hash", next.Items[0].Password)
	}
}

func TestHandleJSONBackedItemPutPreservesExistingFields(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "streams.json")
	items := []npmStream{{
		ID:             1,
		CreatedOn:      "2026-01-01T00:00:00+08:00",
		ModifiedOn:     "2026-01-02T00:00:00+08:00",
		IncomingPort:   8080,
		ForwardingHost: "old.example.com",
		ForwardingPort: 80,
		TCPForwarding:  true,
		Enabled:        true,
		Meta:           map[string]any{"keep": true},
	}}
	if err := saveJSONFile(path, items); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}
	body, _ := json.Marshal(map[string]any{
		"forwarding_host": "new.example.com",
	})
	req := httptest.NewRequest(http.MethodPut, "/1", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handleJSONBackedItem[npmStream](rr, req, "1", path, func() ([]npmStream, error) {
		var out []npmStream
		err := loadJSONFile(path, &out)
		return out, err
	}, func(item *npmStream) {
		stampNPMStream(item, "", true)
	}, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	var saved []npmStream
	if err := loadJSONFile(path, &saved); err != nil {
		t.Fatalf("loadJSONFile error = %v", err)
	}
	if len(saved) != 1 {
		t.Fatalf("saved len = %d, want 1", len(saved))
	}
	got := saved[0]
	if got.CreatedOn != "2026-01-01T00:00:00+08:00" {
		t.Fatalf("CreatedOn = %q, want preserved original", got.CreatedOn)
	}
	if !got.Enabled || !got.TCPForwarding || got.IncomingPort != 8080 || got.ForwardingPort != 80 {
		t.Fatalf("saved stream lost existing fields: %#v", got)
	}
	if got.ForwardingHost != "new.example.com" {
		t.Fatalf("ForwardingHost = %q, want updated value", got.ForwardingHost)
	}
	if got.Meta["keep"] != true {
		t.Fatalf("Meta = %#v, want preserved keep flag", got.Meta)
	}
}

func TestLoadJSONBackedItemsDoesNotRefreshTimestamps(t *testing.T) {
	tmp := t.TempDir()
	oldDynamicDNSPath := dynamicDNSPath
	dynamicDNSPath = filepath.Join(tmp, "dynamic-dns.json")
	t.Cleanup(func() {
		dynamicDNSPath = oldDynamicDNSPath
	})

	created := "2026-01-01T00:00:00+08:00"
	modified := "2026-01-02T00:00:00+08:00"
	if err := saveJSONFile(dynamicDNSPath, []npmDynamicDNS{{
		ID:            1,
		CreatedOn:     created,
		ModifiedOn:    modified,
		Name:          "ddns",
		DomainNames:   []string{"app.example.com"},
		CheckInterval: "5m",
		Enabled:       true,
	}}); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	items, err := loadNPMDynamicDNS()
	if err != nil {
		t.Fatalf("loadNPMDynamicDNS error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].CreatedOn != created || items[0].ModifiedOn != modified {
		t.Fatalf("timestamps = %q/%q, want preserved %q/%q", items[0].CreatedOn, items[0].ModifiedOn, created, modified)
	}
}

func TestParseWakeMACSupportsCommonFormats(t *testing.T) {
	cases := []string{
		"00:11:22:33:44:55",
		"00-11-22-33-44-55",
		"0011.2233.4455",
		"001122334455",
	}
	for _, input := range cases {
		got, err := parseWakeMAC(input)
		if err != nil {
			t.Fatalf("parseWakeMAC(%q) error = %v", input, err)
		}
		if got.String() != "00:11:22:33:44:55" {
			t.Fatalf("parseWakeMAC(%q) = %q", input, got.String())
		}
	}
}

func TestBuildWakeMagicPacket(t *testing.T) {
	mac, err := parseWakeMAC("00:11:22:33:44:55")
	if err != nil {
		t.Fatalf("parseWakeMAC error = %v", err)
	}
	packet := buildWakeMagicPacket(mac, nil)
	if len(packet) != 102 {
		t.Fatalf("packet len = %d, want 102", len(packet))
	}
	for i := 0; i < 6; i++ {
		if packet[i] != 0xff {
			t.Fatalf("packet[%d] = %x, want ff", i, packet[i])
		}
	}
	for offset := 6; offset < len(packet); offset += 6 {
		if !bytes.Equal(packet[offset:offset+6], mac) {
			t.Fatalf("packet repeat at %d = %x, want %x", offset, packet[offset:offset+6], mac)
		}
	}

	secureOn, err := parseWakeMAC("66:77:88:99:AA:BB")
	if err != nil {
		t.Fatalf("parse secureOn error = %v", err)
	}
	packet = buildWakeMagicPacket(mac, secureOn)
	if len(packet) != 108 {
		t.Fatalf("secure packet len = %d, want 108", len(packet))
	}
	if !bytes.Equal(packet[len(packet)-6:], secureOn) {
		t.Fatalf("secureOn tail = %x, want %x", packet[len(packet)-6:], secureOn)
	}
}

func TestWakeDeviceHandlerSendsPacketAndRecordsMeta(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket error = %v", err)
	}
	defer conn.Close()
	port := conn.LocalAddr().(*net.UDPAddr).Port
	if err := saveJSONFile(wakeDevicePath, []npmWakeDevice{{
		ID:               1,
		Name:             "NAS",
		MACAddress:       "00:11:22:33:44:55",
		BroadcastAddress: "127.0.0.1",
		Port:             port,
		Enabled:          true,
		Meta:             map[string]any{},
	}}); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	received := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 256)
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			received <- nil
			return
		}
		received <- buf[:n]
	}()

	req := httptest.NewRequest(http.MethodPost, "/caddy/wake-devices/1/wake", nil)
	rr := httptest.NewRecorder()
	npmWakeDeviceItemHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	packet := <-received
	if len(packet) != 102 {
		t.Fatalf("received packet len = %d, want 102", len(packet))
	}
	mac, _ := parseWakeMAC("00:11:22:33:44:55")
	if !bytes.Equal(packet[6:12], mac) {
		t.Fatalf("received mac = %x, want %x", packet[6:12], mac)
	}
	var saved []npmWakeDevice
	if err := loadJSONFile(wakeDevicePath, &saved); err != nil {
		t.Fatalf("loadJSONFile error = %v", err)
	}
	if got := metaString(saved[0].Meta, "last_woken_at"); got == "" {
		t.Fatalf("last_woken_at not recorded: %#v", saved[0].Meta)
	}
	if got := metaString(saved[0].Meta, "last_error"); got != "" {
		t.Fatalf("last_error = %q, want empty", got)
	}
}

func TestValidateWakeDeviceRejectsInvalidOptionalFields(t *testing.T) {
	item := npmWakeDevice{
		Name:             "NAS",
		MACAddress:       "00:11:22:33:44:55",
		BroadcastAddress: "not-an-ip",
		Port:             9,
	}
	if err := validateWakeDevice(item); err == nil {
		t.Fatalf("validateWakeDevice() error = nil, want invalid broadcast address")
	}
	item.BroadcastAddress = "255.255.255.255"
	item.Port = 70000
	if err := validateWakeDevice(item); err == nil {
		t.Fatalf("validateWakeDevice() error = nil, want invalid port")
	}
	item.Port = 9
	item.SecureOn = "bad"
	if err := validateWakeDevice(item); err == nil {
		t.Fatalf("validateWakeDevice() error = nil, want invalid secureOn")
	}
}

func TestSiteReferencesCertificateDomain(t *testing.T) {
	cases := []struct {
		name       string
		site       Site
		certDomain string
		want       bool
	}{
		{
			name:       "exact active site",
			site:       Site{Name: "site", Domain: "example.com", Backend: "http://127.0.0.1:8080"},
			certDomain: "example.com",
			want:       true,
		},
		{
			name:       "wildcard active site",
			site:       Site{Name: "site", Domain: "app.example.com", Backend: "http://127.0.0.1:8080", Wildcard: true},
			certDomain: "*.example.com",
			want:       true,
		},
		{
			name:       "disabled site ignored",
			site:       Site{Name: "site", Domain: "example.com", Backend: "http://127.0.0.1:8080", Disabled: true},
			certDomain: "example.com",
			want:       false,
		},
		{
			name:       "http only site ignored",
			site:       Site{Name: "site", Domain: "example.com", Backend: "http://127.0.0.1:8080", NoTLS: true},
			certDomain: "example.com",
			want:       false,
		},
		{
			name: "selected binding to another certificate does not match by wildcard",
			site: Site{
				Name:    "site",
				Domain:  "app.example.com",
				Backend: "http://127.0.0.1:8080",
				CertificateBindings: []CertificateBinding{
					{Domain: "app.example.com", Mode: "selected", CertificateID: -999},
				},
			},
			certDomain: "*.example.com",
			want:       false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := siteReferencesCertificateDomain(tc.site, tc.certDomain); got != tc.want {
				t.Fatalf("siteReferencesCertificateDomain() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCertificateDomainNamesFromFileReadsSANs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "certificate.pem")
	writeTestCert(t, path, "fallback.example.com", "example.com", "www.example.com")

	names, err := certificateDomainNamesFromFile(path)
	if err != nil {
		t.Fatalf("certificateDomainNamesFromFile error = %v", err)
	}

	if len(names) != 3 || names[0] != "example.com" || names[1] != "www.example.com" || names[2] != "fallback.example.com" {
		t.Fatalf("names = %#v, want SANs plus common name", names)
	}
}

func TestDDNSSuccessMessage(t *testing.T) {
	cases := []string{
		"finished updating DNS",
		"no IP address change; no update needed",
		"updated IP address records",
	}
	for _, msg := range cases {
		if !ddnsSuccessMessage(msg) {
			t.Fatalf("ddnsSuccessMessage(%q) = false, want true", msg)
		}
	}
	if ddnsSuccessMessage("no IP addresses returned") {
		t.Fatal("ddnsSuccessMessage(error text) = true, want false")
	}
}

func TestDynamicDNSCaddyDomainLineSplitsFQDN(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "alist.example.com", want: "example.com alist"},
		{input: "example.com", want: "example.com @"},
		{input: "*.example.com", want: "example.com *"},
		{input: "*.home.example.com", want: "example.com *.home"},
		{input: "example.com alist", want: "example.com alist"},
		{input: "example.com *", want: "example.com *"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := dynamicDNSCaddyDomainLine(tc.input)
			if err != nil {
				t.Fatalf("dynamicDNSCaddyDomainLine error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("dynamicDNSCaddyDomainLine() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDynamicDNSHostnamesSkipWildcardRecords(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{input: "alist.example.com", want: []string{"alist.example.com"}},
		{input: "*.example.com", want: nil},
		{input: "*.home.example.com", want: nil},
		{input: "example.com *", want: nil},
		{input: "example.com alist", want: []string{"alist.example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := dynamicDNSHostnames(tc.input)
			if !stringSetsEqual(got, tc.want) {
				t.Fatalf("dynamicDNSHostnames() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestDynamicDNSRecordTargets(t *testing.T) {
	cases := []struct {
		input string
		want  []dynamicDNSRecordTarget
	}{
		{input: "alist.example.com", want: []dynamicDNSRecordTarget{{Zone: "example.com", RR: "alist", Host: "alist.example.com"}}},
		{input: "example.com", want: []dynamicDNSRecordTarget{{Zone: "example.com", RR: "@", Host: "example.com"}}},
		{input: "*.example.com", want: []dynamicDNSRecordTarget{{Zone: "example.com", RR: "*", Host: "*.example.com"}}},
		{input: "*.home.example.com", want: []dynamicDNSRecordTarget{{Zone: "example.com", RR: "*.home", Host: "*.home.example.com"}}},
		{input: "example.com * alist", want: []dynamicDNSRecordTarget{
			{Zone: "example.com", RR: "*", Host: "*.example.com"},
			{Zone: "example.com", RR: "alist", Host: "alist.example.com"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := dynamicDNSRecordTargets(tc.input)
			if err != nil {
				t.Fatalf("dynamicDNSRecordTargets error = %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("dynamicDNSRecordTargets() = %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("dynamicDNSRecordTargets()[%d] = %#v, want %#v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestRenderDynamicDNSConfigSplitsFQDNForCaddyPlugin(t *testing.T) {
	conf, err := renderDynamicDNSConfig([]npmDynamicDNS{
		{
			Name:          "alist",
			DomainNames:   []string{"alist.example.com", "*.example.com"},
			CredentialID:  "cred-1",
			IPv4:          true,
			CheckInterval: "5m",
			Enabled:       true,
		},
	}, []Credential{{ID: "cred-1", Provider: "cloudflare", CFToken: "token"}})
	if err != nil {
		t.Fatalf("renderDynamicDNSConfig error = %v", err)
	}
	if !strings.Contains(conf, "        example.com alist\n") {
		t.Fatalf("rendered config = %q, want split zone and record", conf)
	}
	if !strings.Contains(conf, "        example.com *\n") {
		t.Fatalf("rendered config = %q, want wildcard record", conf)
	}
	if strings.Contains(conf, "        alist.example.com\n") {
		t.Fatalf("rendered config = %q, should not pass FQDN directly to caddy-dynamicdns", conf)
	}
	if strings.Contains(conf, "        example.com @\n") {
		t.Fatalf("rendered config = %q, should not rewrite wildcard as root record", conf)
	}
}

func TestRenderDynamicDNSConfigUsesCurrentIPSourceSyntax(t *testing.T) {
	conf, err := renderDynamicDNSConfig([]npmDynamicDNS{
		{
			Name:          "alist",
			DomainNames:   []string{"alist.example.com"},
			CredentialID:  "cred-1",
			IPv4:          true,
			CheckInterval: "5m",
			IPServiceURL:  "https://ipinfo.io/ip",
			Enabled:       true,
		},
	}, []Credential{{ID: "cred-1", Provider: "cloudflare", CFToken: "token"}})
	if err != nil {
		t.Fatalf("renderDynamicDNSConfig error = %v", err)
	}
	if !strings.Contains(conf, "    ip_source simple_http \"https://ipinfo.io/ip\"\n") {
		t.Fatalf("rendered config = %q, want simple_http ip_source", conf)
	}
	if strings.Contains(conf, "    ips") {
		t.Fatalf("rendered config = %q, should not use legacy ips block", conf)
	}
}

func TestRenderDynamicDNSConfigSupportsHurricaneElectric(t *testing.T) {
	conf, err := renderDynamicDNSConfig([]npmDynamicDNS{
		{
			Name:          "he-ddns",
			DomainNames:   []string{"home.example.com"},
			CredentialID:  "cred-he",
			IPv4:          true,
			CheckInterval: "5m",
			Enabled:       true,
		},
	}, []Credential{{ID: "cred-he", Provider: "he", HEAPIKey: "he-key"}})
	if err != nil {
		t.Fatalf("renderDynamicDNSConfig error = %v", err)
	}
	for _, want := range []string{
		"    provider he {\n",
		"        api_key \"he-key\"\n",
		"        example.com home\n",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("rendered config = %q, want %q", conf, want)
		}
	}
}

func TestRenderDynamicDNSConfigAutoIPSourceUsesFallbacks(t *testing.T) {
	oldIPv4Endpoint := publicIPv4Endpoint
	oldFallbacks := publicIPFallbackEndpoints
	t.Cleanup(func() {
		publicIPv4Endpoint = oldIPv4Endpoint
		publicIPFallbackEndpoints = oldFallbacks
	})
	publicIPv4Endpoint = "https://primary.example/ip"
	publicIPFallbackEndpoints = []string{"https://backup.example/ip"}

	conf, err := renderDynamicDNSConfig([]npmDynamicDNS{
		{
			Name:          "alist",
			DomainNames:   []string{"alist.example.com"},
			CredentialID:  "cred-1",
			IPv4:          true,
			CheckInterval: "5m",
			Enabled:       true,
		},
	}, []Credential{{ID: "cred-1", Provider: "cloudflare", CFToken: "token"}})
	if err != nil {
		t.Fatalf("renderDynamicDNSConfig error = %v", err)
	}
	if !strings.Contains(conf, "    ip_source simple_http \"https://primary.example/ip\"\n") {
		t.Fatalf("rendered config = %q, want primary ip_source", conf)
	}
	if !strings.Contains(conf, "    ip_source simple_http \"https://backup.example/ip\"\n") {
		t.Fatalf("rendered config = %q, want fallback ip_source", conf)
	}
}

func TestDynamicDNSNormalizesIfconfigIPServiceURL(t *testing.T) {
	item := npmDynamicDNS{IPServiceURL: "https://ifconfig.me/"}
	stampNPMDynamicDNS(&item, "", true)
	if item.IPServiceURL != "https://ifconfig.me/ip" {
		t.Fatalf("IPServiceURL = %q, want https://ifconfig.me/ip", item.IPServiceURL)
	}

	conf, err := renderDynamicDNSConfig([]npmDynamicDNS{
		{
			Name:          "alist",
			DomainNames:   []string{"alist.example.com"},
			CredentialID:  "cred-1",
			IPv4:          true,
			CheckInterval: "5m",
			IPServiceURL:  "https://ifconfig.me/",
			Enabled:       true,
		},
	}, []Credential{{ID: "cred-1", Provider: "cloudflare", CFToken: "token"}})
	if err != nil {
		t.Fatalf("renderDynamicDNSConfig error = %v", err)
	}
	if !strings.Contains(conf, "    ip_source simple_http \"https://ifconfig.me/ip\"\n") {
		t.Fatalf("rendered config = %q, want normalized ifconfig.me/ip", conf)
	}
}

func TestLoadDynamicDNSExposesDNSProviderFromCredential(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	oldDynamicDNSPath := dynamicDNSPath
	oldCredentialPath := credentialPath
	dynamicDNSPath = filepath.Join(tmp, "dynamic-dns.json")
	credentialPath = filepath.Join(tmp, "credentials.json")
	t.Cleanup(func() {
		dynamicDNSPath = oldDynamicDNSPath
		credentialPath = oldCredentialPath
	})
	if err := saveCredentials([]Credential{{ID: "cred-1", Provider: "alidns", AliyunKey: "key", AliyunSecret: "secret"}}); err != nil {
		t.Fatalf("saveCredentials error = %v", err)
	}
	if err := saveJSONFile(dynamicDNSPath, []npmDynamicDNS{{
		ID:            1,
		Name:          "alist",
		DomainNames:   []string{"alist.example.com"},
		CredentialID:  "cred-1",
		IPv4:          true,
		CheckInterval: "5m",
		Enabled:       true,
		Meta:          map[string]any{},
	}}); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	items, err := loadNPMDynamicDNS()
	if err != nil {
		t.Fatalf("loadNPMDynamicDNS error = %v", err)
	}
	if len(items) != 1 || items[0].DNSProvider != "阿里云 DNS" {
		t.Fatalf("DNSProvider = %#v, want 阿里云 DNS", items)
	}
}

func TestCheckDynamicDNSItemStatusUsesCustomIPServiceURL(t *testing.T) {
	oldIPv4Endpoint := publicIPv4Endpoint
	t.Cleanup(func() {
		publicIPv4Endpoint = oldIPv4Endpoint
	})
	publicIPv4Endpoint = "http://127.0.0.1:1"

	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ip" {
			t.Fatalf("path = %q, want /ip", r.URL.Path)
		}
		_, _ = w.Write([]byte("203.0.113.10"))
	}))
	defer ipServer.Close()

	resolverAddr, closeResolver := startTestDNSServer(t, map[string]string{"alist.example.com.": "203.0.113.10"})
	defer closeResolver()

	errText, meta := checkDynamicDNSItemStatus(npmDynamicDNS{
		Name:          "alist",
		DomainNames:   []string{"alist.example.com"},
		IPv4:          true,
		CheckInterval: "5m",
		Resolvers:     []string{resolverAddr},
		IPServiceURL:  ipServer.URL + "/ip",
		Enabled:       true,
	}, nil, map[string][]string{})
	if errText != "" {
		t.Fatalf("checkDynamicDNSItemStatus error = %q", errText)
	}
	if got := meta["current_ips"]; strings.Join(got.([]string), ",") != "203.0.113.10" {
		t.Fatalf("current_ips = %#v, want 203.0.113.10", got)
	}
}

func TestCurrentPublicIPsAutoFallsBackAfterEndpointFailure(t *testing.T) {
	oldIPv4Endpoint := publicIPv4Endpoint
	oldFallbacks := publicIPFallbackEndpoints
	t.Cleanup(func() {
		publicIPv4Endpoint = oldIPv4Endpoint
		publicIPFallbackEndpoints = oldFallbacks
	})

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "blocked", http.StatusForbidden)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("203.0.113.10"))
	}))
	defer second.Close()
	publicIPv4Endpoint = first.URL
	publicIPFallbackEndpoints = []string{second.URL}

	ips, err := currentPublicIPs(true, false, "", map[string][]string{})
	if err != nil {
		t.Fatalf("currentPublicIPs error = %v", err)
	}
	if strings.Join(ips, ",") != "203.0.113.10" {
		t.Fatalf("currentPublicIPs = %#v, want fallback IP", ips)
	}
}

func TestCheckDynamicDNSItemStatusUpdatesAliyunWildcardRecord(t *testing.T) {
	oldAliyunDNSAPIBaseURL := aliyunDNSAPIBaseURL
	t.Cleanup(func() {
		aliyunDNSAPIBaseURL = oldAliyunDNSAPIBaseURL
	})

	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("203.0.113.50"))
	}))
	defer ipServer.Close()

	updates := []string{}
	aliyunServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("AccessKeyId") != "aliyun-key" {
			t.Fatalf("AccessKeyId = %q, want aliyun-key", r.URL.Query().Get("AccessKeyId"))
		}
		if r.URL.Query().Get("Signature") == "" {
			t.Fatalf("Signature is empty")
		}
		switch r.URL.Query().Get("Action") {
		case "DescribeDomainRecords":
			if r.URL.Query().Get("DomainName") != "example.com" {
				t.Fatalf("DomainName = %q, want example.com", r.URL.Query().Get("DomainName"))
			}
			_, _ = w.Write([]byte(`{"RequestId":"req-1","TotalCount":2,"PageNumber":1,"PageSize":500,"DomainRecords":{"Record":[{"RecordId":"old-record","DomainName":"example.com","RR":"*","Type":"A","Value":"203.0.113.51","TTL":600,"Line":"default"},{"RecordId":"current-record","DomainName":"example.com","RR":"*","Type":"A","Value":"203.0.113.50","TTL":600,"Line":"default"}]}}`))
		case "UpdateDomainRecord":
			updates = append(updates, r.URL.Query().Get("RecordId"))
			if r.URL.Query().Get("RecordId") != "old-record" {
				t.Fatalf("RecordId = %q, want old-record", r.URL.Query().Get("RecordId"))
			}
			if r.URL.Query().Get("RR") != "*" {
				t.Fatalf("RR = %q, want *", r.URL.Query().Get("RR"))
			}
			if r.URL.Query().Get("Type") != "A" {
				t.Fatalf("Type = %q, want A", r.URL.Query().Get("Type"))
			}
			if r.URL.Query().Get("Value") != "203.0.113.50" {
				t.Fatalf("Value = %q, want 203.0.113.50", r.URL.Query().Get("Value"))
			}
			_, _ = w.Write([]byte(`{"RequestId":"req-2","RecordId":"old-record"}`))
		default:
			t.Fatalf("unexpected Aliyun API request: %s", r.URL.String())
		}
	}))
	defer aliyunServer.Close()
	aliyunDNSAPIBaseURL = aliyunServer.URL

	errText, meta := checkDynamicDNSItemStatus(npmDynamicDNS{
		Name:          "wildcard",
		DomainNames:   []string{"*.example.com"},
		CredentialID:  "cred-1",
		IPv4:          true,
		CheckInterval: "5m",
		IPServiceURL:  ipServer.URL,
		Enabled:       true,
	}, []Credential{{ID: "cred-1", Provider: "alidns", AliyunKey: "aliyun-key", AliyunSecret: "aliyun-secret"}}, map[string][]string{})
	if errText != "" {
		t.Fatalf("checkDynamicDNSItemStatus error = %q", errText)
	}
	if len(updates) != 1 || updates[0] != "old-record" {
		t.Fatalf("updates = %#v, want old-record", updates)
	}
	if got := meta["resolved_ips"]; strings.Join(got.([]string), ",") != "203.0.113.50" {
		t.Fatalf("resolved_ips = %#v, want updated source IP", got)
	}
}

func TestRefreshDynamicDNSStatusesReportsRemoteARecordMismatch(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	oldDynamicDNSPath := dynamicDNSPath
	oldIPv4Endpoint := publicIPv4Endpoint
	oldIPv6Endpoint := publicIPv6Endpoint
	dynamicDNSPath = filepath.Join(tmp, "dynamic-dns.json")
	t.Cleanup(func() {
		dynamicDNSPath = oldDynamicDNSPath
		publicIPv4Endpoint = oldIPv4Endpoint
		publicIPv6Endpoint = oldIPv6Endpoint
	})

	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("203.0.113.10"))
	}))
	defer ipServer.Close()
	publicIPv4Endpoint = ipServer.URL
	publicIPv6Endpoint = ipServer.URL

	resolverAddr, closeResolver := startTestDNSServer(t, map[string]string{"alist.example.com.": "198.51.100.20"})
	defer closeResolver()

	items := []npmDynamicDNS{{
		ID:            1,
		Name:          "alist",
		DomainNames:   []string{"alist.example.com"},
		IPv4:          true,
		CheckInterval: "5m",
		Resolvers:     []string{resolverAddr},
		Enabled:       true,
		Meta:          map[string]any{},
	}}
	if err := saveJSONFile(dynamicDNSPath, items); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	if err := refreshDynamicDNSStatuses(); err != nil {
		t.Fatalf("refreshDynamicDNSStatuses error = %v", err)
	}

	var got []npmDynamicDNS
	if err := loadJSONFile(dynamicDNSPath, &got); err != nil {
		t.Fatalf("loadJSONFile error = %v", err)
	}
	lastErr := metaString(got[0].Meta, "last_error", "lastError")
	if !strings.Contains(lastErr, "远端记录为 198.51.100.20") || !strings.Contains(lastErr, "当前公网 IP 为 203.0.113.10") {
		t.Fatalf("last_error = %q, want remote/current IP mismatch", lastErr)
	}
}

func TestSyncDynamicDNSAndReloadRefreshesStatusAfterSave(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)
	oldDynamicDNSPath := dynamicDNSPath
	oldDynamicDNSConfPath := dynamicDNSConfPath
	oldCredentialPath := credentialPath
	oldCaddyfile := caddyfile
	oldCaddyAdmin := caddyAdmin
	oldAliyunDNSAPIBaseURL := aliyunDNSAPIBaseURL
	dynamicDNSPath = filepath.Join(tmp, "dynamic-dns.json")
	dynamicDNSConfPath = filepath.Join(tmp, "dynamic-dns.conf")
	credentialPath = filepath.Join(tmp, "credentials.json")
	caddyfile = filepath.Join(tmp, "Caddyfile")
	t.Cleanup(func() {
		dynamicDNSPath = oldDynamicDNSPath
		dynamicDNSConfPath = oldDynamicDNSConfPath
		credentialPath = oldCredentialPath
		caddyfile = oldCaddyfile
		caddyAdmin = oldCaddyAdmin
		aliyunDNSAPIBaseURL = oldAliyunDNSAPIBaseURL
	})

	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("203.0.113.10"))
	}))
	defer ipServer.Close()

	aliyunServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("Action") {
		case "DescribeDomainRecords":
			if r.URL.Query().Get("DomainName") != "example.com" {
				t.Fatalf("DomainName = %q, want example.com", r.URL.Query().Get("DomainName"))
			}
			_, _ = w.Write([]byte(`{"RequestId":"req-1","TotalCount":1,"PageNumber":1,"PageSize":500,"DomainRecords":{"Record":[{"RecordId":"record-1","DomainName":"example.com","RR":"alist","Type":"A","Value":"203.0.113.10","TTL":600,"Line":"default"}]}}`))
		default:
			t.Fatalf("unexpected Aliyun API request: %s", r.URL.String())
		}
	}))
	defer aliyunServer.Close()
	aliyunDNSAPIBaseURL = aliyunServer.URL

	resolverAddr, closeResolver := startTestDNSServer(t, map[string]string{"alist.example.com.": "203.0.113.10"})
	defer closeResolver()

	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/load" {
			t.Fatalf("path = %q, want /load", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer adminServer.Close()
	caddyAdmin = adminServer.URL

	if err := os.WriteFile(caddyfile, []byte("{\n}\n"), 0644); err != nil {
		t.Fatalf("WriteFile Caddyfile error = %v", err)
	}
	if err := saveCredentials([]Credential{{ID: "cred-1", Provider: "alidns", AliyunKey: "key", AliyunSecret: "secret"}}); err != nil {
		t.Fatalf("saveCredentials error = %v", err)
	}
	if err := saveJSONFile(dynamicDNSPath, []npmDynamicDNS{{
		ID:            1,
		Name:          "alist",
		DomainNames:   []string{"alist.example.com"},
		CredentialID:  "cred-1",
		IPv4:          true,
		CheckInterval: "5m",
		Resolvers:     []string{resolverAddr},
		IPServiceURL:  ipServer.URL,
		Enabled:       true,
		Meta:          map[string]any{},
	}}); err != nil {
		t.Fatalf("saveJSONFile error = %v", err)
	}

	if err := syncDynamicDNSAndReload(); err != nil {
		t.Fatalf("syncDynamicDNSAndReload error = %v", err)
	}

	var got []npmDynamicDNS
	if err := loadJSONFile(dynamicDNSPath, &got); err != nil {
		t.Fatalf("loadJSONFile error = %v", err)
	}
	if metaString(got[0].Meta, "last_checked", "lastChecked") == "" {
		t.Fatalf("last_checked not set in meta: %#v", got[0].Meta)
	}
	if gotIPs, _ := got[0].Meta["current_ips"].([]any); len(gotIPs) != 1 || gotIPs[0] != "203.0.113.10" {
		t.Fatalf("current_ips = %#v, want 203.0.113.10", got[0].Meta["current_ips"])
	}
	if errText := metaString(got[0].Meta, "last_error", "lastError"); errText != "" {
		t.Fatalf("last_error = %q, want empty", errText)
	}
}

func TestCheckDynamicDNSItemStatusUpdatesCloudflareProxiedRecord(t *testing.T) {
	oldCloudflareAPIBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldCloudflareAPIBaseURL
	})

	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("203.0.113.50"))
	}))
	defer ipServer.Close()

	patchCalled := false
	cfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cf-token" {
			t.Fatalf("Authorization = %q, want Bearer cf-token", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/zones":
			if r.URL.Query().Get("name") != "example.net" {
				_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"zone-1","name":"example.net"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/zones/zone-1/dns_records" && r.URL.Query().Get("type") == "A":
			if r.URL.Query().Get("name") != "test.example.net" {
				t.Fatalf("record name = %q, want test.example.net", r.URL.Query().Get("name"))
			}
			_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"record-1","type":"A","name":"test.example.net","content":"203.0.113.51","ttl":1,"proxied":true}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/zones/zone-1/dns_records" && r.URL.Query().Get("type") == "AAAA":
			_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/zones/zone-1/dns_records/record-1":
			patchCalled = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("Decode patch body error = %v", err)
			}
			if body["content"] != "203.0.113.50" {
				t.Fatalf("patch content = %#v, want 203.0.113.50", body["content"])
			}
			if body["proxied"] != true {
				t.Fatalf("patch proxied = %#v, want true", body["proxied"])
			}
			_, _ = w.Write([]byte(`{"success":true,"result":{"id":"record-1","type":"A","name":"test.example.net","content":"203.0.113.50","ttl":1,"proxied":true}}`))
		default:
			t.Fatalf("unexpected Cloudflare API request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer cfServer.Close()
	cloudflareAPIBaseURL = cfServer.URL

	resolverAddr, closeResolver := startTestDNSServer(t, map[string]string{"test.example.net.": "198.51.100.30"})
	defer closeResolver()

	errText, meta := checkDynamicDNSItemStatus(npmDynamicDNS{
		Name:          "wvw",
		DomainNames:   []string{"test.example.net"},
		CredentialID:  "cred-1",
		IPv4:          true,
		CheckInterval: "5m",
		Resolvers:     []string{resolverAddr},
		IPServiceURL:  ipServer.URL,
		Enabled:       true,
	}, []Credential{{ID: "cred-1", Provider: "cloudflare", CFToken: "cf-token"}}, map[string][]string{})
	if errText != "" {
		t.Fatalf("checkDynamicDNSItemStatus error = %q", errText)
	}
	if !patchCalled {
		t.Fatalf("Cloudflare PATCH was not called")
	}
	if got := meta["resolved_ips"]; strings.Join(got.([]string), ",") != "203.0.113.50" {
		t.Fatalf("resolved_ips = %#v, want updated source IP", got)
	}
}

func startTestDNSServer(t *testing.T, records map[string]string) (string, func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket error = %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 512)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			req := append([]byte(nil), buf[:n]...)
			resp := buildTestDNSResponse(req, records)
			if len(resp) > 0 {
				_, _ = conn.WriteTo(resp, addr)
			}
		}
	}()
	return conn.LocalAddr().String(), func() {
		_ = conn.Close()
		<-done
	}
}

func buildTestDNSResponse(req []byte, records map[string]string) []byte {
	if len(req) < 12 {
		return nil
	}
	qnameEnd := 12
	labels := []string{}
	for qnameEnd < len(req) {
		l := int(req[qnameEnd])
		if l == 0 {
			qnameEnd++
			break
		}
		qnameEnd++
		if qnameEnd+l > len(req) {
			return nil
		}
		labels = append(labels, string(req[qnameEnd:qnameEnd+l]))
		qnameEnd += l
	}
	if qnameEnd+4 > len(req) {
		return nil
	}
	questionEnd := qnameEnd + 4
	qtype := uint16(req[qnameEnd])<<8 | uint16(req[qnameEnd+1])
	name := strings.ToLower(strings.Join(labels, ".") + ".")
	ip := net.ParseIP(records[name]).To4()
	answerCount := uint16(0)
	if qtype == 1 && ip != nil {
		answerCount = 1
	}
	resp := make([]byte, 0, questionEnd+32)
	resp = append(resp, req[:2]...)
	resp = append(resp, 0x81, 0x80)
	resp = append(resp, 0x00, 0x01)
	resp = append(resp, byte(answerCount>>8), byte(answerCount))
	resp = append(resp, 0x00, 0x00, 0x00, 0x00)
	resp = append(resp, req[12:questionEnd]...)
	if answerCount == 1 {
		resp = append(resp, 0xc0, 0x0c)
		resp = append(resp, 0x00, 0x01)
		resp = append(resp, 0x00, 0x01)
		resp = append(resp, 0x00, 0x00, 0x00, 0x3c)
		resp = append(resp, 0x00, 0x04)
		resp = append(resp, ip...)
	}
	return resp
}
