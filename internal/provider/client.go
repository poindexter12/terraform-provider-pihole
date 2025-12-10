package provider

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

// getClient extracts the Pi-hole client from the provider meta interface.
// Returns an error diagnostic if the client cannot be loaded.
func getClient(meta interface{}) (pihole.Client, diag.Diagnostics) {
	client, ok := meta.(pihole.Client)
	if !ok {
		return nil, diag.Errorf("could not load Pi-hole client")
	}
	return client, nil
}
