package provider

import (
	"sync"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

// ProviderMeta holds the Pi-hole client and coordination primitives shared
// across all resource operations within a Terraform run.
type ProviderMeta struct {
	// Client is the Pi-hole API client
	Client pihole.Client

	// mu serializes ALL Pi-hole API operations to prevent race conditions.
	// The Pi-hole API can fail silently when operations occur concurrently,
	// so we use a single mutex to ensure operations happen sequentially.
	// This also ensures ForceNew replacement operations (delete + create)
	// are atomic - no other operation can interleave between the delete
	// and create of the same resource.
	mu sync.Mutex
}

// Lock acquires the global operation mutex. Call this before any Pi-hole API operation.
func (p *ProviderMeta) Lock() {
	p.mu.Lock()
}

// Unlock releases the global operation mutex. Always defer this after Lock().
func (p *ProviderMeta) Unlock() {
	p.mu.Unlock()
}

// getClient extracts the Pi-hole client from the provider meta interface.
// Returns an error diagnostic if the client cannot be loaded.
func getClient(meta interface{}) (pihole.Client, diag.Diagnostics) {
	pm, ok := meta.(*ProviderMeta)
	if !ok {
		return nil, diag.Errorf("could not load Pi-hole client")
	}
	return pm.Client, nil
}

// getProviderMeta extracts the ProviderMeta from the provider meta interface.
// Returns an error diagnostic if it cannot be loaded.
func getProviderMeta(meta interface{}) (*ProviderMeta, diag.Diagnostics) {
	pm, ok := meta.(*ProviderMeta)
	if !ok {
		return nil, diag.Errorf("could not load provider metadata")
	}
	return pm, nil
}
