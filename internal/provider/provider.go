package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/poindexter12/terraform-provider-pihole/internal/version"
)

func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PIHOLE_PASSWORD", nil),
				Description: "The admin password used to login to the admin dashboard.",
			},
			"url": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PIHOLE_URL", "http://pi.hole"),
				Description: "URL where Pi-hole is deployed",
			},
			"ca_file": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PIHOLE_CA_FILE", nil),
				Description: "Path to a CA certificate file for TLS verification",
			},
			"insecure_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Skip TLS certificate verification. WARNING: This is insecure and should only be used for testing or in trusted networks with self-signed certificates.",
			},
		},

		DataSourcesMap: map[string]*schema.Resource{
			"pihole_cname_records": dataSourceCNAMERecords(),
			"pihole_dns_records":   dataSourceDNSRecords(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"pihole_cname_record": resourceCNAMERecord(),
			"pihole_dns_record":   resourceDNSRecord(),
		},
	}

	provider.ConfigureContextFunc = configure(version.ProviderVersion, provider)

	return provider
}

// configure configures a Pi-hole client to be used for terraform resource requests
func configure(version string, provider *schema.Provider) func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	return func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		// Check if a session ID was passed in externally (for testing or session reuse)
		externalSessionID := os.Getenv("__PIHOLE_SESSION_ID")

		piholeClient, err := Config{
			Password:           d.Get("password").(string),
			URL:                d.Get("url").(string),
			UserAgent:          provider.UserAgent("terraform-provider-pihole", version),
			CAFile:             d.Get("ca_file").(string),
			InsecureSkipVerify: d.Get("insecure_skip_verify").(bool),
			SessionID:          externalSessionID,
		}.Client(ctx)

		if err != nil {
			return nil, diag.FromErr(fmt.Errorf("failed to instantiate client: %w", err))
		}

		// Return ProviderMeta which wraps the client and provides coordination
		return &ProviderMeta{Client: piholeClient}, nil
	}
}
