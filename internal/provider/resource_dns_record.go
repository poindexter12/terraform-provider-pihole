package provider

import (
	"context"
	"errors"
	"sync"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

// dnsMutex serializes DNS create/delete operations to work around a race
// condition in the Pi-hole API. When multiple DNS records are modified
// concurrently, some operations silently fail, leaving orphaned or missing records.
// This mutex ensures DNS mutations happen sequentially.
// See: https://github.com/ryanwholey/terraform-provider-pihole/issues/68
var dnsMutex sync.Mutex

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
		},
	}
}

// resourceDNSRecordCreate handles the creation a local DNS record via Terraform
func resourceDNSRecordCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	client, diags := getClient(meta)
	if diags != nil {
		return diags
	}

	domain := d.Get("domain").(string)
	ip := d.Get("ip").(string)

	dnsMutex.Lock()
	defer dnsMutex.Unlock()

	_, err := client.LocalDNS().Create(ctx, domain, ip)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(domain)

	return diags
}

// resourceDNSRecordRead finds a local DNS record based on the associated domain ID
func resourceDNSRecordRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	client, diags := getClient(meta)
	if diags != nil {
		return diags
	}

	record, err := client.LocalDNS().Get(ctx, d.Id())
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
	client, diags := getClient(meta)
	if diags != nil {
		return diags
	}

	dnsMutex.Lock()
	defer dnsMutex.Unlock()

	if err := client.LocalDNS().Delete(ctx, d.Id()); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diags
}
