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

	dns         *dnsService
	cname       *cnameService
	clientMgmt  *clientService
	passwordSvc *passwordService
}

// NewClient creates a new Pi-hole v6 API client
func NewClient(ctx context.Context, cfg pihole.Config) (*Client, error) {
	httpClient := retryablehttp.NewClient()
	httpClient.Logger = nil // Disable debug logging
	stdClient := httpClient.StandardClient()

	// Configure TLS settings
	tlsConfig := &tls.Config{}
	needsCustomTransport := false

	// Handle custom CA file
	if cfg.CAFile != "" {
		ca, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file %q: %w", cfg.CAFile, err)
		}

		rootCAs := x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("failed to parse CA certificates from %q", cfg.CAFile)
		}

		tlsConfig.RootCAs = rootCAs
		needsCustomTransport = true
	}

	// Handle insecure skip verify (for self-signed certs without CA file)
	if cfg.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
		needsCustomTransport = true
	}

	if needsCustomTransport {
		stdClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
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
	c.clientMgmt = &clientService{client: c}
	c.passwordSvc = &passwordService{client: c}

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

// ClientManagement returns the client management service
func (c *Client) ClientManagement() pihole.ClientManagementService {
	return c.clientMgmt
}

// Password returns the admin password management service
func (c *Client) Password() pihole.PasswordService {
	return c.passwordSvc
}

// SessionID returns the current session ID
func (c *Client) SessionID() string {
	c.sessionLock.RLock()
	defer c.sessionLock.RUnlock()
	return c.sessionID
}

// authenticate obtains a session ID from the Pi-hole API
func (c *Client) authenticate(ctx context.Context) error {
	return c.authenticateWith(ctx, c.password)
}

// authenticateWith obtains a session ID using the given password. On success
// the client's stored password and session are swapped to the new credentials,
// so subsequent requests from any resource keep working after a password change.
func (c *Client) authenticateWith(ctx context.Context, password string) error {
	body := map[string]string{"password": password}
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
	c.password = password
	c.sessionLock.Unlock()

	return nil
}

// SessionPassword returns the password the client last authenticated with
func (c *Client) SessionPassword() string {
	c.sessionLock.RLock()
	defer c.sessionLock.RUnlock()
	return c.password
}

// request performs an authenticated HTTP request. If the session has been
// invalidated (Pi-hole purges all sessions on any password change — on some
// FTL versions in a delayed second pass — and expires idle sessions after
// webserver.session.timeout), it re-authenticates once with the stored
// credential and retries the request.
func (c *Client) request(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	resp, err := c.doRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	resp.Body.Close()

	if err := c.authenticate(ctx); err != nil {
		return nil, fmt.Errorf("session expired and re-authentication failed: %w", err)
	}

	return c.doRequest(ctx, method, path, body)
}

// doRequest performs a single authenticated HTTP request without retry
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
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

// post performs an authenticated POST request
func (c *Client) post(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.request(ctx, http.MethodPost, path, body)
}

// put performs an authenticated PUT request
func (c *Client) put(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.request(ctx, http.MethodPut, path, body)
}

// patch performs an authenticated PATCH request
func (c *Client) patch(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.request(ctx, http.MethodPatch, path, body)
}

// delete performs an authenticated DELETE request
func (c *Client) delete(ctx context.Context, path string) (*http.Response, error) {
	return c.request(ctx, http.MethodDelete, path, nil)
}

// Logout terminates the current session with Pi-hole
func (c *Client) Logout(ctx context.Context) error {
	c.sessionLock.RLock()
	sid := c.sessionID
	c.sessionLock.RUnlock()

	if sid == "" {
		return nil // No session to logout
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/auth", nil)
	if err != nil {
		return err
	}

	req.Header.Set(sessionHeader, sid)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Clear the session ID
	c.sessionLock.Lock()
	c.sessionID = ""
	c.sessionLock.Unlock()

	return nil
}
