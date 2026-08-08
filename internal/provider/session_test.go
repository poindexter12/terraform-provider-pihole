package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

// fakeClient is a pihole.Client that only implements what session cleanup
// touches: it counts Logout calls and can block to exercise the deadline.
type fakeClient struct {
	logouts atomic.Int32

	// block, when non-nil, holds Logout until it is closed or ctx expires.
	block chan struct{}
}

func (f *fakeClient) LocalDNS() pihole.LocalDNSService                 { return nil }
func (f *fakeClient) LocalCNAME() pihole.LocalCNAMEService             { return nil }
func (f *fakeClient) ClientManagement() pihole.ClientManagementService { return nil }
func (f *fakeClient) Password() pihole.PasswordService                 { return nil }
func (f *fakeClient) SessionID() string                                { return "fake-session" }

func (f *fakeClient) Logout(ctx context.Context) error {
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	f.logouts.Add(1)

	return nil
}

// resetSessions isolates a test from sessions registered elsewhere in the
// package, and leaves the registry empty for the tests that follow.
func resetSessions(t *testing.T) {
	t.Helper()

	sessions.drain()
	t.Cleanup(func() {
		sessions.drain()
	})
}

func TestSessionRegistryLogsOutEverySession(t *testing.T) {
	resetSessions(t)

	first, second := &fakeClient{}, &fakeClient{}
	sessions.add(first)
	sessions.add(second)

	sessions.logoutAll(context.Background())

	if got := first.logouts.Load(); got != 1 {
		t.Errorf("first client logouts = %d, want 1", got)
	}

	if got := second.logouts.Load(); got != 1 {
		t.Errorf("second client logouts = %d, want 1", got)
	}
}

// Both the stop path and the normal exit path can run in the same process, so
// a session must not be logged out twice.
func TestSessionRegistryLogoutIsIdempotent(t *testing.T) {
	resetSessions(t)

	client := &fakeClient{}
	sessions.add(client)

	sessions.logoutAll(context.Background())
	sessions.logoutAll(context.Background())

	if got := client.logouts.Load(); got != 1 {
		t.Errorf("logouts = %d, want 1", got)
	}
}

// Cleanup runs inside go-plugin's shutdown grace period, so an unreachable
// Pi-hole must not keep the process alive past the context deadline.
func TestSessionRegistryLogoutRespectsDeadline(t *testing.T) {
	resetSessions(t)

	blocked := &fakeClient{block: make(chan struct{})}
	defer close(blocked.block)

	sessions.add(blocked)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})

	go func() {
		defer close(done)

		sessions.logoutAll(ctx)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("logoutAll did not return after its context expired")
	}
}

// authServer stands in for Pi-hole's /api/auth endpoint and reports whether a
// session was ever deleted.
func authServer(t *testing.T) (*httptest.Server, func() bool) {
	t.Helper()

	var mu sync.Mutex
	deleted := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth" {
			http.NotFound(w, r)

			return
		}

		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"session":{"sid":"test-session"}}`))
		case http.MethodDelete:
			mu.Lock()
			deleted = true
			mu.Unlock()

			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))

	t.Cleanup(server.Close)

	return server, func() bool {
		mu.Lock()
		defer mu.Unlock()

		return deleted
	}
}

func configureProvider(t *testing.T, url string) {
	t.Helper()

	provider := Provider()
	data := schema.TestResourceDataRaw(t, provider.Schema, map[string]interface{}{
		"url":      url,
		"password": "test",
	})

	if _, diags := configure("test", provider)(context.Background(), data); diags.HasError() {
		t.Fatalf("configure returned diagnostics: %v", diags)
	}
}

func TestConfigureSessionIsLoggedOutOnShutdown(t *testing.T) {
	resetSessions(t)
	t.Setenv("__PIHOLE_SESSION_ID", "")

	server, wasDeleted := authServer(t)

	configureProvider(t, server.URL)

	if wasDeleted() {
		t.Fatal("session was deleted before shutdown")
	}

	LogoutAll()

	if !wasDeleted() {
		t.Error("session was not deleted on shutdown")
	}
}

// Sessions supplied through __PIHOLE_SESSION_ID belong to whoever created
// them, so shutdown must leave them alone.
func TestConfigureExternalSessionIsNotLoggedOut(t *testing.T) {
	resetSessions(t)
	t.Setenv("__PIHOLE_SESSION_ID", "externally-managed")

	server, wasDeleted := authServer(t)

	configureProvider(t, server.URL)

	LogoutAll()

	if wasDeleted() {
		t.Error("externally managed session was deleted on shutdown")
	}
}
