package v6

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

const clientsPath = "/api/clients"

type clientService struct {
	client *Client
}

// clientAPIRecord represents a client record in the Pi-hole v6 API response
type clientAPIRecord struct {
	Client       string `json:"client"`
	Name         string `json:"name"`
	Comment      string `json:"comment"`
	Groups       []int  `json:"groups"`
	ID           int    `json:"id"`
	DateAdded    int64  `json:"date_added"`
	DateModified int64  `json:"date_modified"`
}

// clientsListResponse is the API response for listing clients
type clientsListResponse struct {
	Clients []clientAPIRecord `json:"clients"`
}

// toRecord converts an API record to a pihole.ClientRecord
func (r *clientAPIRecord) toRecord() *pihole.ClientRecord {
	return &pihole.ClientRecord{
		Client:       r.Client,
		Name:         r.Name,
		Comment:      r.Comment,
		Groups:       r.Groups,
		ID:           r.ID,
		DateAdded:    r.DateAdded,
		DateModified: r.DateModified,
	}
}

// List returns all client records
func (s *clientService) List(ctx context.Context) ([]pihole.ClientRecord, error) {
	resp, err := s.client.get(ctx, clientsPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result clientsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	records := make([]pihole.ClientRecord, 0, len(result.Clients))
	for _, c := range result.Clients {
		records = append(records, *c.toRecord())
	}

	return records, nil
}

// Get returns a specific client by identifier
func (s *clientService) Get(ctx context.Context, clientID string) (*pihole.ClientRecord, error) {
	path := fmt.Sprintf("%s/%s", clientsPath, url.PathEscape(clientID))

	resp, err := s.client.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, pihole.ErrClientNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result clientsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Clients) == 0 {
		return nil, pihole.ErrClientNotFound
	}

	return result.Clients[0].toRecord(), nil
}

// Create adds a new client record
func (s *clientService) Create(ctx context.Context, clientID, comment string) (*pihole.ClientRecord, error) {
	body := map[string]string{
		"client":  clientID,
		"comment": comment,
	}

	resp, err := s.client.post(ctx, clientsPath, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d (expected 200 or 201)", resp.StatusCode)
	}

	var result clientsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Clients) == 0 {
		return nil, fmt.Errorf("no client returned in response")
	}

	return result.Clients[0].toRecord(), nil
}

// Update modifies an existing client record
func (s *clientService) Update(ctx context.Context, clientID, comment string) (*pihole.ClientRecord, error) {
	path := fmt.Sprintf("%s/%s", clientsPath, url.PathEscape(clientID))

	body := map[string]string{
		"comment": comment,
	}

	resp, err := s.client.put(ctx, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, pihole.ErrClientNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d (expected 200)", resp.StatusCode)
	}

	var result clientsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Clients) == 0 {
		return nil, fmt.Errorf("no client returned in response")
	}

	return result.Clients[0].toRecord(), nil
}

// Delete removes a client record
func (s *clientService) Delete(ctx context.Context, clientID string) error {
	path := fmt.Sprintf("%s/%s", clientsPath, url.PathEscape(clientID))

	resp, err := s.client.delete(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return pihole.ErrClientNotFound
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d (expected 204)", resp.StatusCode)
	}

	return nil
}
