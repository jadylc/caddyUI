package dnspod

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/libdns/libdns"
	"github.com/nrdcg/dnspod-go"
)

// Record wraps libdns.RR to include the DNSPod record ID.
type Record struct {
	base libdns.RR
	ID   string
}

// RR returns the underlying libdns.RR struct.
func (r Record) RR() libdns.RR {
	return r.base
}

// Provider wraps the provider implementation as a Caddy module.
type Provider struct {
	APIToken string `json:"api_token,omitempty"`
}

func init() {
	caddy.RegisterModule(Provider{})
}

// CaddyModule returns the Caddy module information.
func (Provider) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "dns.providers.dnspod",
		New: func() caddy.Module { return new(Provider) },
	}
}

// Provision sets up the module.
func (p *Provider) Provision(ctx caddy.Context) error {
	p.APIToken = caddy.NewReplacer().ReplaceAll(p.APIToken, "")
	return nil
}

// UnmarshalCaddyfile sets up the DNS provider from Caddyfile tokens.
func (p *Provider) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if d.NextArg() {
			p.APIToken = d.Val()
		}
		if d.NextArg() {
			return d.ArgErr()
		}
		for nesting := d.Nesting(); d.NextBlock(nesting); {
			switch d.Val() {
			case "api_token":
				if !d.NextArg() {
					return d.ArgErr()
				}
				if p.APIToken != "" {
					return d.Err("API token already set")
				}
				p.APIToken = d.Val()
				if d.NextArg() {
					return d.ArgErr()
				}
			default:
				return d.Errf("unrecognized subdirective '%s'", d.Val())
			}
		}
	}
	if p.APIToken == "" {
		return d.Err("missing API token")
	}
	return nil
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	client := p.getClient()
	zone = strings.Trim(zone, ".")
	domainID, _, err := p.getDomain(zone)
	if err != nil {
		return nil, err
	}
	records, _, err := client.Records.List(domainID, "")
	if err != nil {
		return nil, err
	}

	var libdnsRecords []libdns.Record
	for _, record := range records {
		libdnsRecords = append(libdnsRecords, p.toLibdnsRecord(record))
	}
	return libdnsRecords, nil
}

// AppendRecords adds records to the zone.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	client := p.getClient()
	zone = strings.Trim(zone, ".")
	var addedRecords []libdns.Record

	for _, libdnsRecord := range records {
		domainID, dnspodRecord, returnRecord, err := p.recordContext(zone, libdnsRecord)
		if err != nil {
			return addedRecords, err
		}
		created, _, err := client.Records.Create(domainID, dnspodRecord)
		if err != nil {
			return addedRecords, err
		}
		addedRecords = append(addedRecords, withRecordID(returnRecord, created.ID))
	}

	return addedRecords, nil
}

// SetRecords sets the records in the zone, either by updating existing ones or creating new ones.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	client := p.getClient()
	zone = strings.Trim(zone, ".")

	var setRecords []libdns.Record

	for _, libdnsRecord := range records {
		id := ""
		if r, ok := libdnsRecord.(Record); ok {
			id = r.ID
		}
		domainID, dnspodRec, returnRecord, err := p.recordContext(zone, libdnsRecord)
		if err != nil {
			return setRecords, err
		}
		existingRecords, _, err := client.Records.List(domainID, "")
		if err != nil {
			return setRecords, fmt.Errorf("failed to list existing records: %v", err)
		}

		// If no ID provided, try to find an existing record with matching name and type
		if id == "" {
			for _, existing := range existingRecords {
				if existing.Name == dnspodRec.Name && existing.Type == dnspodRec.Type {
					id = existing.ID
					break
				}
			}
		}

		if id == "" {
			// No existing record found, create new
			created, _, err := client.Records.Create(domainID, dnspodRec)
			if err != nil {
				return setRecords, err
			}
			setRecords = append(setRecords, withRecordID(returnRecord, created.ID))
			continue
		}

		// Update existing record
		dnspodRec.ID = id
		updated, _, err := client.Records.Update(domainID, id, dnspodRec)
		if err != nil {
			// Fallback: Delete and Re-create if Update fails
			_, _ = client.Records.Delete(domainID, id)
			created, _, err := client.Records.Create(domainID, dnspodRec)
			if err != nil {
				return setRecords, fmt.Errorf("update failed (%v) and fallback create also failed: %v", id, err)
			}
			setRecords = append(setRecords, withRecordID(returnRecord, created.ID))
			continue
		}
		setRecords = append(setRecords, withRecordID(returnRecord, updated.ID))
	}

	return setRecords, nil
}

// DeleteRecords deletes the records from the zone.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	client := p.getClient()
	zone = strings.Trim(zone, ".")
	var deletedRecords []libdns.Record

	for _, libdnsRecord := range records {
		id := ""
		if r, ok := libdnsRecord.(Record); ok {
			id = r.ID
		}
		domainID, dnspodRec, _, err := p.recordContext(zone, libdnsRecord)
		if err != nil {
			return deletedRecords, err
		}
		if id == "" {
			existingRecords, _, err := client.Records.List(domainID, "")
			if err != nil {
				return deletedRecords, fmt.Errorf("failed to list existing records: %v", err)
			}
			for _, existing := range existingRecords {
				if existing.Name == dnspodRec.Name && existing.Type == dnspodRec.Type && existing.Value == dnspodRec.Value {
					id = existing.ID
					break
				}
			}
		}
		if id == "" {
			deletedRecords = append(deletedRecords, libdnsRecord)
			continue
		}
		_, err = client.Records.Delete(domainID, id)
		if err != nil {
			return deletedRecords, err
		}
		deletedRecords = append(deletedRecords, libdnsRecord)
	}

	return deletedRecords, nil
}

func (p *Provider) getClient() *dnspod.Client {
	return dnspod.NewClient(dnspod.CommonParams{
		LoginToken: p.APIToken,
	})
}

func (p *Provider) getDomainID(zone string) (string, error) {
	id, _, err := p.getDomain(zone)
	return id, err
}

func (p *Provider) getDomain(zone string) (string, string, error) {
	if _, err := strconv.Atoi(zone); err == nil {
		return zone, zone, nil
	}

	client := p.getClient()
	domains, _, err := client.Domains.List()
	if err != nil {
		return "", "", err
	}

	for _, d := range domains {
		if d.Name == zone {
			return d.ID.String(), d.Name, nil
		}
	}

	return "", "", fmt.Errorf("domain %s not found", zone)
}

func (p *Provider) recordContext(zone string, record libdns.Record) (string, dnspod.Record, libdns.Record, error) {
	zone = strings.Trim(zone, ".")
	dnspodRecord := p.fromLibdnsRecord(record)
	returnRecord := withRecordID(record, "")
	if domainID, _, err := p.getDomain(zone); err == nil {
		dnspodRecord.Name = normalizeDNSPodRecordName(dnspodRecord.Name)
		return domainID, dnspodRecord, returnRecord, nil
	}

	fqdn := recordFQDN(dnspodRecord.Name, zone)
	client := p.getClient()
	domains, _, err := client.Domains.List()
	if err != nil {
		return "", dnspod.Record{}, nil, err
	}
	bestID := ""
	bestName := ""
	for _, domain := range domains {
		name := strings.Trim(domain.Name, ".")
		if name == "" {
			continue
		}
		if fqdn == name || strings.HasSuffix(fqdn, "."+name) {
			if len(name) > len(bestName) {
				bestName = name
				bestID = domain.ID.String()
			}
		}
	}
	if bestID == "" {
		return "", dnspod.Record{}, nil, fmt.Errorf("domain %s not found", zone)
	}
	relative := strings.TrimSuffix(fqdn, "."+bestName)
	if relative == fqdn {
		relative = "@"
	}
	dnspodRecord.Name = normalizeDNSPodRecordName(relative)
	return bestID, dnspodRecord, returnRecord, nil
}

func recordFQDN(name string, zone string) string {
	name = strings.Trim(strings.TrimSpace(name), ".")
	zone = strings.Trim(strings.TrimSpace(zone), ".")
	if name == "" || name == "@" {
		return zone
	}
	if zone == "" || name == zone || strings.HasSuffix(name, "."+zone) {
		return name
	}
	return name + "." + zone
}

func normalizeDNSPodRecordName(name string) string {
	name = strings.Trim(strings.TrimSpace(name), ".")
	if name == "" {
		return "@"
	}
	return name
}

func withRecordID(record libdns.Record, id string) libdns.Record {
	return Record{
		ID:   id,
		base: record.RR(),
	}
}

func (p *Provider) toLibdnsRecord(record dnspod.Record) libdns.Record {
	ttl, _ := strconv.Atoi(record.TTL)
	return Record{
		ID: record.ID,
		base: libdns.RR{
			Type: record.Type,
			Name: record.Name,
			Data: record.Value,
			TTL:  time.Duration(ttl) * time.Second,
		},
	}
}

func (p *Provider) fromLibdnsRecord(record libdns.Record) dnspod.Record {
	rr := record.RR()
	return dnspod.Record{
		Type:  rr.Type,
		Name:  rr.Name,
		Value: rr.Data,
		TTL:   fmt.Sprintf("%.0f", rr.TTL.Seconds()),
		Line:  "默认",
	}
}

var (
	_ caddyfile.Unmarshaler = (*Provider)(nil)
	_ caddy.Provisioner     = (*Provider)(nil)
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
