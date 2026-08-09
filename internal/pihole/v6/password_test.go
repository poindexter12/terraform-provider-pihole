package v6

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

// mockPihole simulates the Pi-hole v6 auth/config API surface with the
// behaviors observed against a real instance:
//   - every password set invalidates ALL sessions, including the caller's,
//     even when the value is unchanged
//   - the pwhash only changes when the password value actually changes
//   - optionally, a freshly set password is rejected by /api/auth for a
//     number of attempts (propagation delay)
type mockPihole struct {
	mu sync.Mutex

	adminPassword string
	appPassword   string // optional second accepted credential
	sessions      map[string]bool
	hashVersion   int
	sidCounter    int

	// propagationDelay is how many auth attempts with a newly set admin
	// password to reject before accepting it.
	propagationDelay     int
	propagationRemaining int

	// delayedPurges simulates older FTL versions that purge sessions again
	// asynchronously after a password change: each pending purge kills all
	// sessions right before the next authenticated request is validated.
	delayedPurges int
}

func newMockPihole(adminPassword string) *mockPihole {
	return &mockPihole{
		adminPassword: adminPassword,
		sessions:      map[string]bool{},
		hashVersion:   1,
	}
}

func (m *mockPihole) hash() string {
	return fmt.Sprintf("$MOCK-HASH$v=%d", m.hashVersion)
}

func (m *mockPihole) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		if body.Password == m.adminPassword && m.propagationRemaining > 0 {
			m.propagationRemaining--
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if body.Password != m.adminPassword && (m.appPassword == "" || body.Password != m.appPassword) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		m.sidCounter++
		sid := fmt.Sprintf("sid-%d", m.sidCounter)
		m.sessions[sid] = true

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"session":{"sid":%q}}`, sid)
	})

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !m.validSession(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var body struct {
			Config struct {
				Webserver struct {
					API struct {
						Password string `json:"password"`
					} `json:"api"`
				} `json:"webserver"`
			} `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		newPassword := body.Config.Webserver.API.Password
		if newPassword != m.adminPassword {
			m.adminPassword = newPassword
			m.hashVersion++
			m.propagationRemaining = m.propagationDelay
		}

		// Any password set kills every session, changed or not
		m.sessions = map[string]bool{}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"config":{}}`)
	})

	mux.HandleFunc("/api/config/webserver/api/pwhash", func(w http.ResponseWriter, r *http.Request) {
		if !m.validSession(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"config":{"webserver":{"api":{"pwhash":%q}}}}`, m.hash())
	})

	return mux
}

func (m *mockPihole) validSession(r *http.Request) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.delayedPurges > 0 {
		m.delayedPurges--
		m.sessions = map[string]bool{}
	}

	return m.sessions[r.Header.Get(sessionHeader)]
}

func newTestClient(t *testing.T, mock *mockPihole, password string) (*Client, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(mock.handler())
	t.Cleanup(server.Close)

	client, err := NewClient(context.Background(), pihole.Config{
		BaseURL:  server.URL,
		Password: password,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	return client, server
}

func TestPasswordGetHash(t *testing.T) {
	mock := newMockPihole("initial")
	client, _ := newTestClient(t, mock, "initial")

	hash, err := client.Password().GetHash(context.Background())
	if err != nil {
		t.Fatalf("GetHash: %v", err)
	}
	if hash != "$MOCK-HASH$v=1" {
		t.Fatalf("unexpected hash: %q", hash)
	}
}

func TestPasswordUpdateRotatesAndReauthenticates(t *testing.T) {
	mock := newMockPihole("initial")
	client, _ := newTestClient(t, mock, "initial")
	oldSID := client.SessionID()

	if err := client.Password().Update(context.Background(), "rotated"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if client.SessionID() == oldSID {
		t.Fatal("expected a fresh session after password change")
	}
	if client.SessionPassword() != "rotated" {
		t.Fatalf("expected client password swapped to new value, got %q", client.SessionPassword())
	}

	// The refreshed session must be usable
	hash, err := client.Password().GetHash(context.Background())
	if err != nil {
		t.Fatalf("GetHash after rotation: %v", err)
	}
	if hash != "$MOCK-HASH$v=2" {
		t.Fatalf("expected hash to change after rotation, got %q", hash)
	}
}

func TestPasswordUpdateSameValueKeepsHash(t *testing.T) {
	mock := newMockPihole("initial")
	client, _ := newTestClient(t, mock, "initial")
	oldSID := client.SessionID()

	if err := client.Password().Update(context.Background(), "initial"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Sessions die even on a same-value set...
	if client.SessionID() == oldSID {
		t.Fatal("expected a fresh session even for a same-value set")
	}

	// ...but the hash must be stable (no drift false-positives)
	hash, err := client.Password().GetHash(context.Background())
	if err != nil {
		t.Fatalf("GetHash: %v", err)
	}
	if hash != "$MOCK-HASH$v=1" {
		t.Fatalf("expected stable hash for unchanged password, got %q", hash)
	}
}

func TestPasswordUpdateWithPropagationDelay(t *testing.T) {
	mock := newMockPihole("initial")
	mock.propagationDelay = 2 // reject the new password twice before accepting
	client, _ := newTestClient(t, mock, "initial")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Password().Update(ctx, "rotated"); err != nil {
		t.Fatalf("Update with propagation delay: %v", err)
	}

	if client.SessionPassword() != "rotated" {
		t.Fatalf("expected client password swapped to new value, got %q", client.SessionPassword())
	}
}

func TestPasswordUpdatePreservesAppPasswordAuth(t *testing.T) {
	mock := newMockPihole("admin-pw")
	mock.appPassword = "app-pw"
	client, _ := newTestClient(t, mock, "app-pw")

	if err := client.Password().Update(context.Background(), "new-admin-pw"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// The client authenticated with the app password and must keep it: the
	// stored credential is tried first during re-authentication.
	if client.SessionPassword() != "app-pw" {
		t.Fatalf("expected client to keep app password, got %q", client.SessionPassword())
	}

	if _, err := client.Password().GetHash(context.Background()); err != nil {
		t.Fatalf("GetHash with app password session: %v", err)
	}
}

// TestSessionReauthOnExpiry verifies the client transparently recovers when
// its session is invalidated between requests (idle timeout, or another
// actor changing the password).
func TestSessionReauthOnExpiry(t *testing.T) {
	mock := newMockPihole("initial")
	client, _ := newTestClient(t, mock, "initial")

	mock.delayedPurges = 1 // next request finds its session dead

	hash, err := client.Password().GetHash(context.Background())
	if err != nil {
		t.Fatalf("expected transparent re-authentication, got: %v", err)
	}
	if hash != "$MOCK-HASH$v=1" {
		t.Fatalf("unexpected hash: %q", hash)
	}
}

// TestPasswordUpdateSurvivesDelayedPurge simulates older FTL versions where a
// password change is followed by a second asynchronous session purge: the
// session obtained by Update's re-authentication dies before the next call.
func TestPasswordUpdateSurvivesDelayedPurge(t *testing.T) {
	mock := newMockPihole("initial")
	client, _ := newTestClient(t, mock, "initial")

	if err := client.Password().Update(context.Background(), "rotated"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	mock.delayedPurges = 1 // the async purge lands now

	hash, err := client.Password().GetHash(context.Background())
	if err != nil {
		t.Fatalf("expected recovery from delayed purge, got: %v", err)
	}
	if hash != "$MOCK-HASH$v=2" {
		t.Fatalf("unexpected hash: %q", hash)
	}
}

// TestSessionReauthFailurePropagates verifies a clear error when the session
// is dead and the stored credential no longer authenticates.
func TestSessionReauthFailurePropagates(t *testing.T) {
	mock := newMockPihole("initial")
	client, _ := newTestClient(t, mock, "initial")

	// Password changed out-of-band: stored credential is now wrong
	mock.mu.Lock()
	mock.adminPassword = "changed-elsewhere"
	mock.sessions = map[string]bool{}
	mock.mu.Unlock()

	_, err := client.Password().GetHash(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "re-authentication failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPasswordUpdateTimesOut(t *testing.T) {
	mock := newMockPihole("initial")
	// Larger than the number of attempts that can happen before the context
	// deadline, so re-authentication never succeeds.
	mock.propagationDelay = 1000
	client, _ := newTestClient(t, mock, "initial")

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()

	err := client.Password().Update(ctx, "rotated")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "re-authentication") {
		t.Fatalf("unexpected error: %v", err)
	}
}
