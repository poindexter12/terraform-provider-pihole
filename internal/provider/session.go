package provider

import (
	"context"
	"sync"
	"time"

	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

// logoutTimeout bounds how long session cleanup may take during shutdown.
//
// When Terraform is done with a plugin it closes the connection and then waits
// roughly two seconds for the process to exit before sending SIGKILL (see
// Client.Kill in hashicorp/go-plugin). Cleanup has to fit inside that window,
// so we deliberately leave a margin instead of claiming the whole budget.
const logoutTimeout = 1500 * time.Millisecond

// sessionRegistry tracks the Pi-hole clients whose sessions this process
// opened and is therefore responsible for closing.
//
// Pi-hole allows a limited number of concurrent sessions
// (webserver.api.max_sessions, 16 by default) and keeps them until
// webserver.session.timeout elapses (30 minutes by default). A session left
// behind by every plan and every apply eventually locks the user out of both
// the API and the admin dashboard, so every session we open must be closed.
//
// A registry is needed rather than a single client because the provider is
// configured once per provider instance: aliases and multiple provider blocks
// each authenticate separately within a single run.
type sessionRegistry struct {
	mu      sync.Mutex
	clients []pihole.Client
}

// sessions is the process-wide registry, drained on shutdown by LogoutAll.
var sessions sessionRegistry

// add records a client whose session should be terminated on shutdown.
func (r *sessionRegistry) add(client pihole.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.clients = append(r.clients, client)
}

// drain returns the registered clients and empties the registry. Shutdown can
// be reached from more than one path, so draining is what makes logging out
// idempotent: whichever path runs first takes the clients, the rest find none.
func (r *sessionRegistry) drain() []pihole.Client {
	r.mu.Lock()
	defer r.mu.Unlock()

	clients := r.clients
	r.clients = nil

	return clients
}

// logoutAll terminates every registered session concurrently and waits for the
// results. Errors are ignored because cleanup is best effort: a failed logout
// only leaves the session to expire on its own. Logout honours ctx, so a
// deadline on ctx bounds the total time spent here no matter how many sessions
// are registered.
func (r *sessionRegistry) logoutAll(ctx context.Context) {
	var wg sync.WaitGroup

	for _, client := range r.drain() {
		wg.Add(1)

		go func(client pihole.Client) {
			defer wg.Done()

			_ = client.Logout(ctx)
		}(client)
	}

	wg.Wait()
}

// LogoutAll terminates every Pi-hole session opened by this process.
//
// It is called from main once plugin.Serve returns, which is the only hook
// that runs on a normal Terraform run: the SDK stop context backing
// cleanupOnStop is cancelled only when Terraform interrupts the provider.
// Calling it more than once is safe, and it does nothing when no sessions were
// opened.
func LogoutAll() {
	ctx, cancel := context.WithTimeout(context.Background(), logoutTimeout)
	defer cancel()

	sessions.logoutAll(ctx)
}

// cleanupOnStop terminates registered sessions once Terraform signals a stop.
// This covers interrupts (Ctrl-C), where Terraform may kill the process before
// plugin.Serve returns and main gets a chance to clean up.
func cleanupOnStop(stopCtx context.Context) {
	<-stopCtx.Done()

	// stopCtx is cancelled by the time we get here, so LogoutAll builds its
	// own context rather than deriving from it.
	LogoutAll()
}
