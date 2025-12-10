package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

// resourceDNSRecord returns the local DNS Terraform resource management configuration
func resourceDNSRecord() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages a Pi-hole DNS record",
		CreateContext: resourceDNSRecordCreate,
		ReadContext:   resourceDNSRecordRead,
		DeleteContext: resourceDNSRecordDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"domain": {
				Description:      "DNS record domain",
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateDiagFunc: validateDomain(),
			},
			"ip": {
				Description:      "IP address to route traffic to from the DNS record domain",
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateDiagFunc: validateIPAddress(),
			},
			"force": {
				Description: "Attempt to force record creation. Note: Pi-hole v6 API currently does not implement this for DNS endpoints, but it is included for forward compatibility with future Pi-hole versions.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
			},
		},
	}
}

// resourceDNSRecordCreate handles the creation a local DNS record via Terraform
func resourceDNSRecordCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	domain := d.Get("domain").(string)
	ip := d.Get("ip").(string)
	force := d.Get("force").(bool)

	// Acquire global mutex to serialize all Pi-hole API operations
	pm.Lock()
	defer pm.Unlock()

	opts := &pihole.CreateOptions{Force: force}
	_, err := pm.Client.LocalDNS().Create(ctx, domain, ip, opts)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(domain)

	return diags
}

// resourceDNSRecordRead finds a local DNS record based on the associated domain ID
func resourceDNSRecordRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	// Read operations also acquire the mutex to prevent reads during writes
	pm.Lock()
	defer pm.Unlock()

	record, err := pm.Client.LocalDNS().Get(ctx, d.Id())
	if err != nil {
		if errors.Is(err, pihole.ErrDNSNotFound) {
			d.SetId("")
			return nil
		}

		return diag.FromErr(err)
	}

	if err = d.Set("domain", record.Domain); err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("ip", record.IP); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

// resourceDNSRecordDelete handles the deletion of a local DNS record via Terraform
func resourceDNSRecordDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	// Acquire global mutex to serialize all Pi-hole API operations
	pm.Lock()
	defer pm.Unlock()

	if err := pm.Client.LocalDNS().Delete(ctx, d.Id()); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diags
}
