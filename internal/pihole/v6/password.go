package v6

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	configPath = "/api/config"
	pwhashPath = "/api/config/webserver/api/pwhash"

	// reauthInterval is the delay between re-authentication attempts after a
	// password change while waiting for the new password to become active.
	reauthInterval = 500 * time.Millisecond
)

type passwordService struct {
	client *Client
}

// pwhashResponse is the API response for reading the password hash
type pwhashResponse struct {
	Config struct {
		Webserver struct {
			API struct {
				Pwhash string `json:"pwhash"`
			} `json:"api"`
		} `json:"webserver"`
	} `json:"config"`
}

// GetHash returns the current admin password hash (webserver.api.pwhash).
func (s *passwordService) GetHash(ctx context.Context) (string, error) {
	resp, err := s.client.get(ctx, pwhashPath)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result pwhashResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Config.Webserver.API.Pwhash, nil
}

// Update sets the admin password via PATCH /api/config.
//
// Pi-hole invalidates every session (including the one making this request)
// whenever the password is set, even if the value is unchanged. After the
// PATCH succeeds this method re-authenticates, first with the client's stored
// password (which succeeds when the provider authenticates with an app
// password or the value didn't change), then with the new admin password.
// Attempts are retried until one succeeds or ctx expires, which also absorbs
// any propagation delay before the new password becomes active.
func (s *passwordService) Update(ctx context.Context, newPassword string) error {
	body := map[string]interface{}{
		"config": map[string]interface{}{
			"webserver": map[string]interface{}{
				"api": map[string]interface{}{
					"password": newPassword,
				},
			},
		},
	}

	resp, err := s.client.patch(ctx, configPath, body)
	if err != nil {
		return err
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set password: unexpected status code: %d: %s"+
			" (if the password is managed via the FTLCONF_webserver_api_password"+
			" environment variable it is read-only and cannot be changed through the API)",
			resp.StatusCode, string(respBody))
	}

	return s.reauthenticate(ctx, newPassword)
}

// reauthenticate restores the client session after a password change.
// The stored password is tried before the new admin password so that a client
// authenticated with an app password keeps using it.
func (s *passwordService) reauthenticate(ctx context.Context, newPassword string) error {
	var lastErr error
	for attempt := 1; ; attempt++ {
		candidates := []string{s.client.SessionPassword(), newPassword}
		for _, candidate := range candidates {
			if err := s.client.authenticateWith(ctx, candidate); err == nil {
				return nil
			} else if ctx.Err() != nil {
				return ctx.Err()
			} else {
				lastErr = err
			}
		}

		tflog.Debug(ctx, "re-authentication after password change not yet successful, retrying", map[string]interface{}{
			"attempt": attempt,
		})

		select {
		case <-ctx.Done():
			return fmt.Errorf("password was set but re-authentication did not succeed before timeout: %w (last error: %v)", ctx.Err(), lastErr)
		case <-time.After(reauthInterval):
		}
	}
}
