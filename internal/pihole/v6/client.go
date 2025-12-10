package v6

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

const (
	sessionHeader = "X-FTL-SID"
)

// Client implements pihole.Client for Pi-hole v6 API
type Client struct {
	baseURL   string
	password  string
	userAgent string
	http      *http.Client

	sessionID   string
	sessionLock sync.RWMutex

	dns   *dnsService
	cname *cnameService
}

// NewClient creates a new Pi-hole v6 API client
func NewClient(ctx context.Context, cfg pihole.Config) (*Client, error) {
	httpClient := retryablehttp.NewClient()
	httpClient.Logger = nil // Disable debug logging
	stdClient := httpClient.StandardClient()

	// Configure TLS if CA file provided
	if cfg.CAFile != "" {
		ca, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file %q: %w", cfg.CAFile, err)
		}

		rootCAs := x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("failed to parse CA certificates from %q", cfg.CAFile)
		}

		stdClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: rootCAs,
			},
		}
	}

	c := &Client{
		baseURL:   cfg.BaseURL,
		password:  cfg.Password,
		userAgent: cfg.UserAgent,
		http:      stdClient,
		sessionID: cfg.SessionID,
	}

	c.dns = &dnsService{client: c}
	c.cname = &cnameService{client: c}

	// If no session ID provided, authenticate now
	if c.sessionID == "" {
		if err := c.authenticate(ctx); err != nil {
			return nil, fmt.Errorf("failed to authenticate: %w", err)
		}
	}

	return c, nil
}

// LocalDNS returns the DNS record service
func (c *Client) LocalDNS() pihole.LocalDNSService {
	return c.dns
}

// LocalCNAME returns the CNAME record service
func (c *Client) LocalCNAME() pihole.LocalCNAMEService {
	return c.cname
}

// SessionID returns the current session ID
func (c *Client) SessionID() string {
	c.sessionLock.RLock()
	defer c.sessionLock.RUnlock()
	return c.sessionID
}

// authenticate obtains a session ID from the Pi-hole API
func (c *Client) authenticate(ctx context.Context) error {
	body := map[string]string{"password": c.password}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/auth", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return pihole.ErrAuthFailed
	}

	var result struct {
		Session struct {
			SID string `json:"sid"`
		} `json:"session"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Session.SID == "" {
		return pihole.ErrSessionNotFound
	}

	c.sessionLock.Lock()
	c.sessionID = result.Session.SID
	c.sessionLock.Unlock()

	return nil
}

// request performs an authenticated HTTP request
func (c *Client) request(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	c.sessionLock.RLock()
	req.Header.Set(sessionHeader, c.sessionID)
	c.sessionLock.RUnlock()

	return c.http.Do(req)
}

// get performs an authenticated GET request
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	return c.request(ctx, http.MethodGet, path, nil)
}

// put performs an authenticated PUT request
func (c *Client) put(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.request(ctx, http.MethodPut, path, body)
}

// delete performs an authenticated DELETE request
func (c *Client) delete(ctx context.Context, path string) (*http.Response, error) {
	return c.request(ctx, http.MethodDelete, path, nil)
}
