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

const cnamePath = "/api/config/dns/cnameRecords"

type cnameService struct {
	client *Client
}

// cnameListResponse is the API response for listing CNAME records
type cnameListResponse struct {
	Config struct {
		DNS struct {
			CNAMERecords []string `json:"cnameRecords"`
		} `json:"dns"`
	} `json:"config"`
}

// List returns all CNAME records
func (s *cnameService) List(ctx context.Context) ([]pihole.CNAMERecord, error) {
	resp, err := s.client.get(ctx, cnamePath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result cnameListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return parseCNAMEs(result.Config.DNS.CNAMERecords), nil
}

// Get returns a specific CNAME record by domain
func (s *cnameService) Get(ctx context.Context, domain string) (*pihole.CNAMERecord, error) {
	records, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		if r.Domain == domain {
			return &r, nil
		}
	}

	return nil, pihole.ErrCNAMENotFound
}

// Create adds a new CNAME record.
// Includes retry logic for "duplicate CNAME" errors that can occur during
// ForceNew operations when dnsmasq hasn't fully processed a prior delete.
func (s *cnameService) Create(ctx context.Context, domain, target string, opts *pihole.CreateOptions) (*pihole.CNAMERecord, error) {
	path := fmt.Sprintf("%s/%s", cnamePath, url.PathEscape(domain+","+target))

	// Append force parameter if requested
	if opts != nil && opts.Force {
		path += "?force=true"
	}

	const maxRetries = 5
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 200ms, 400ms, 800ms, 1600ms
			delay := time.Duration(200<<uint(attempt-1)) * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := s.client.put(ctx, path, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusCreated {
			resp.Body.Close()
			return &pihole.CNAMERecord{Domain: domain, Target: target}, nil
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyStr := string(body)

		// Pi-hole enforces uniqueness on the full "domain,target" item, so
		// "already present" can only mean the exact record we want already
		// exists — the desired state is reached (see issue #38). "duplicate
		// CNAME" is different: dnsmasq rejects a second CNAME for the same
		// domain with another target, which can resolve once a pending delete
		// completes, so it stays retryable.
		if resp.StatusCode == http.StatusBadRequest && strings.Contains(bodyStr, "already present") {
			tflog.Warn(ctx, "CNAME record already exists with the desired value, adopting it", map[string]interface{}{
				"domain": domain,
				"target": target,
			})
			return &pihole.CNAMERecord{Domain: domain, Target: target}, nil
		}

		if resp.StatusCode == http.StatusBadRequest && strings.Contains(bodyStr, "duplicate CNAME") {
			lastErr = fmt.Errorf("conflicting CNAME for domain exists (attempt %d/%d): %s", attempt+1, maxRetries, bodyStr)
			continue
		}

		// Non-retryable error
		return nil, fmt.Errorf("unexpected status code: %d (expected 201): %s", resp.StatusCode, bodyStr)
	}

	return nil, lastErr
}

// Delete removes a CNAME record.
// Returns nil if the record doesn't exist (idempotent delete, logs warning).
// Verifies the record is fully deleted before returning to prevent race conditions.
func (s *cnameService) Delete(ctx context.Context, domain string) error {
	// First get the record to find its target
	record, err := s.Get(ctx, domain)
	if err != nil {
		// If record not found, delete is already done - warn but don't error
		if err == pihole.ErrCNAMENotFound {
			tflog.Warn(ctx, "CNAME record does not exist, nothing to delete", map[string]interface{}{
				"domain": domain,
			})
			return nil
		}
		return err
	}

	path := fmt.Sprintf("%s/%s", cnamePath, url.PathEscape(domain+","+record.Target))

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
		if err == pihole.ErrCNAMENotFound {
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

	return fmt.Errorf("delete API succeeded but CNAME record %q still visible after %d verification attempts", domain, maxVerifyAttempts)
}

// parseCNAMEs converts "domain,target" strings to CNAMERecord structs
func parseCNAMEs(cnames []string) []pihole.CNAMERecord {
	records := make([]pihole.CNAMERecord, 0, len(cnames))
	for _, c := range cnames {
		parts := strings.SplitN(c, ",", 2)
		if len(parts) == 2 {
			records = append(records, pihole.CNAMERecord{
				Domain: parts[0],
				Target: parts[1],
			})
		}
	}
	return records
}
