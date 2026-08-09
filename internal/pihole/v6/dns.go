package v6

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

const dnsHostsPath = "/api/config/dns/hosts"

type dnsService struct {
	client *Client
}

// dnsListResponse is the API response for listing DNS records
type dnsListResponse struct {
	Config struct {
		DNS struct {
			Hosts []string `json:"hosts"`
		} `json:"dns"`
	} `json:"config"`
}

// List returns all local DNS records
func (s *dnsService) List(ctx context.Context) ([]pihole.DNSRecord, error) {
	resp, err := s.client.get(ctx, dnsHostsPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result dnsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return parseDNSHosts(result.Config.DNS.Hosts), nil
}

// Get returns a specific DNS record by domain
func (s *dnsService) Get(ctx context.Context, domain string) (*pihole.DNSRecord, error) {
	records, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		if r.Domain == domain {
			return &r, nil
		}
	}

	return nil, pihole.ErrDNSNotFound
}

// Create adds a new DNS record.
// If opts.Force is true and the record already exists, it will be deleted first.
// An "already present" response is treated as success since Pi-hole enforces
// uniqueness on the full "IP domain" item — the existing record is exactly
// the desired one.
func (s *dnsService) Create(ctx context.Context, domain, ip string, opts *pihole.CreateOptions) (*pihole.DNSRecord, error) {
	// If force is requested and record exists, delete it first
	if opts != nil && opts.Force {
		existing, err := s.Get(ctx, domain)
		if err == nil && existing != nil {
			// Record exists - delete it first
			if delErr := s.Delete(ctx, domain); delErr != nil {
				return nil, fmt.Errorf("force delete failed: %w", delErr)
			}
			// Brief pause to let Pi-hole process the delete
			time.Sleep(100 * time.Millisecond)
		}
		// If err != nil (not found), that's fine - proceed with create
	}

	path := fmt.Sprintf("%s/%s", dnsHostsPath, url.PathEscape(ip+" "+domain))

	resp, err := s.client.put(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusCreated {
		resp.Body.Close()
		return &pihole.DNSRecord{Domain: domain, IP: ip}, nil
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Pi-hole enforces uniqueness on the full "IP domain" item, so "already
	// present" can only mean the exact record we want already exists — the
	// desired state is reached. Treating it as success makes creates converge
	// instead of flaking when FTL's config write-back resurrects a deleted
	// record (its dnsmasq-test child processes race under rapid mutations,
	// losing committed deletes; see issue #38).
	if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "already present") {
		tflog.Warn(ctx, "DNS record already exists with the desired value, adopting it", map[string]interface{}{
			"domain": domain,
			"ip":     ip,
		})
		return &pihole.DNSRecord{Domain: domain, IP: ip}, nil
	}

	return nil, fmt.Errorf("unexpected status code: %d (expected 201): %s", resp.StatusCode, string(body))
}

// Delete removes a DNS record.
// Returns nil if the record doesn't exist (idempotent delete, logs warning).
// Verifies the record is fully deleted before returning to prevent race conditions.
func (s *dnsService) Delete(ctx context.Context, domain string) error {
	// First get the record to find its IP
	record, err := s.Get(ctx, domain)
	if err != nil {
		// If record not found, delete is already done - warn but don't error
		if err == pihole.ErrDNSNotFound {
			tflog.Warn(ctx, "DNS record does not exist, nothing to delete", map[string]interface{}{
				"domain": domain,
			})
			return nil
		}
		return err
	}

	path := fmt.Sprintf("%s/%s", dnsHostsPath, url.PathEscape(record.IP+" "+domain))

	resp, err := s.client.delete(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 204 = deleted, 404 = already gone (both are success)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("unexpected status code: %d (expected 204)", resp.StatusCode)
	}

	// Verify the record is actually gone before returning.
	// Pi-hole's API can return 204 before the delete is fully processed internally,
	// causing race conditions when Create() is called immediately after.
	const maxVerifyAttempts = 10
	const verifyDelay = 100 * time.Millisecond
	for attempt := 0; attempt < maxVerifyAttempts; attempt++ {
		_, err := s.Get(ctx, domain)
		if err == pihole.ErrDNSNotFound {
			// Record is confirmed deleted
			return nil
		}
		// Record still visible, wait and retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(verifyDelay):
		}
	}

	return fmt.Errorf("delete API succeeded but record %q still visible after %d verification attempts", domain, maxVerifyAttempts)
}

// parseDNSHosts converts "IP domain" strings to DNSRecord structs
func parseDNSHosts(hosts []string) []pihole.DNSRecord {
	records := make([]pihole.DNSRecord, 0, len(hosts))
	for _, h := range hosts {
		parts := strings.SplitN(h, " ", 2)
		if len(parts) == 2 {
			records = append(records, pihole.DNSRecord{
				IP:     parts[0],
				Domain: parts[1],
			})
		}
	}
	return records
}
