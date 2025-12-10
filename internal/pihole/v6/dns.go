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
// Includes retry logic for transient errors that can occur during
// ForceNew operations when Pi-hole hasn't fully processed a prior delete.
func (s *dnsService) Create(ctx context.Context, domain, ip string, opts *pihole.CreateOptions) (*pihole.DNSRecord, error) {
	path := fmt.Sprintf("%s/%s", dnsHostsPath, url.PathEscape(ip+" "+domain))

	// Append force parameter if requested
	if opts != nil && opts.Force {
		path += "?force=true"
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Wait before retry - Pi-hole may need time to process the delete
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}

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

		// Check if this is a retryable error (duplicate/conflict during ForceNew)
		bodyStr := string(body)
		if resp.StatusCode == http.StatusBadRequest && strings.Contains(bodyStr, "already present") {
			lastErr = fmt.Errorf("item already present (attempt %d/3): %s", attempt+1, bodyStr)
			continue
		}

		// Non-retryable error
		return nil, fmt.Errorf("unexpected status code: %d (expected 201): %s", resp.StatusCode, bodyStr)
	}

	return nil, lastErr
}

// Delete removes a DNS record.
// Returns nil if the record doesn't exist (idempotent delete).
func (s *dnsService) Delete(ctx context.Context, domain string) error {
	// First get the record to find its IP
	record, err := s.Get(ctx, domain)
	if err != nil {
		// If record not found, delete is already done
		if err == pihole.ErrDNSNotFound {
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

	return nil
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
