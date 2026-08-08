package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"github.com/poindexter12/terraform-provider-pihole/internal/provider"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: provider.Provider,
	})

	// plugin.Serve returns once Terraform closes the plugin connection, which
	// makes this the only cleanup hook that runs on a successful Terraform
	// run: the SDK stop context is cancelled on interrupt only. Terraform
	// allows roughly two seconds before SIGKILL, and LogoutAll stays inside
	// that window.
	provider.LogoutAll()
}
