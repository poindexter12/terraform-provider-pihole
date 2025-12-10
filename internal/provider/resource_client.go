package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

// resourceClient returns the Pi-hole client Terraform resource management configuration
func resourceClient() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages a Pi-hole client configuration. Clients can be IP addresses, MAC addresses, hostnames, CIDR ranges, or interface names.",
		CreateContext: resourceClientCreate,
		ReadContext:   resourceClientRead,
		UpdateContext: resourceClientUpdate,
		DeleteContext: resourceClientDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"client": {
				Description: "Client identifier (IP address, MAC address, hostname, CIDR range, or interface name)",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"comment": {
				Description: "Optional comment for the client",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
			},
		},
	}
}

// resourceClientCreate handles the creation of a client record via Terraform
func resourceClientCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	client := d.Get("client").(string)
	comment := d.Get("comment").(string)

	pm.Lock()
	defer pm.Unlock()

	_, err := pm.Client.ClientManagement().Create(ctx, client, comment)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(client)

	return diags
}

// resourceClientRead finds a client record based on the client ID
func resourceClientRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	pm.Lock()
	defer pm.Unlock()

	record, err := pm.Client.ClientManagement().Get(ctx, d.Id())
	if err != nil {
		if errors.Is(err, pihole.ErrClientNotFound) {
			d.SetId("")
			return nil
		}

		return diag.FromErr(err)
	}

	if err = d.Set("client", record.Client); err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("comment", record.Comment); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

// resourceClientUpdate handles updating a client record via Terraform
func resourceClientUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	comment := d.Get("comment").(string)

	pm.Lock()
	defer pm.Unlock()

	_, err := pm.Client.ClientManagement().Update(ctx, d.Id(), comment)
	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

// resourceClientDelete handles the deletion of a client record via Terraform
func resourceClientDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	pm.Lock()
	defer pm.Unlock()

	if err := pm.Client.ClientManagement().Delete(ctx, d.Id()); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diags
}
