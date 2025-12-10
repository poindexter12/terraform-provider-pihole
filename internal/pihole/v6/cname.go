package v6

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

// Create adds a new CNAME record
func (s *cnameService) Create(ctx context.Context, domain, target string) (*pihole.CNAMERecord, error) {
	path := fmt.Sprintf("%s/%s", cnamePath, url.PathEscape(domain+","+target))

	resp, err := s.client.put(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d (expected 201)", resp.StatusCode)
	}

	return &pihole.CNAMERecord{Domain: domain, Target: target}, nil
}

// Delete removes a CNAME record
func (s *cnameService) Delete(ctx context.Context, domain string) error {
	// First get the record to find its target
	record, err := s.Get(ctx, domain)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("%s/%s", cnamePath, url.PathEscape(domain+","+record.Target))

	resp, err := s.client.delete(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d (expected 204)", resp.StatusCode)
	}

	return nil
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
