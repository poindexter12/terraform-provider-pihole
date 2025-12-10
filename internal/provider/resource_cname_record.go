package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

// resourceCNAMERecord returns the CNAME Terraform resource management configuration
func resourceCNAMERecord() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages a Pi-hole CNAME record",
		CreateContext: resourceCNAMERecordCreate,
		ReadContext:   resourceCNAMERecordRead,
		DeleteContext: resourceCNAMERecordDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"domain": {
				Description:      "Domain to create a CNAME record for",
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateDiagFunc: validateDomain(),
			},
			"target": {
				Description:      "Value of the CNAME record where traffic will be directed to from the configured domain value",
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateDiagFunc: validateDomain(),
			},
			"force": {
				Description: "Attempt to force record creation. Note: Pi-hole v6 API currently does not implement this for CNAME endpoints, but it is included for forward compatibility with future Pi-hole versions.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
			},
		},
	}
}

// resourceCNAMERecordCreate handles the creation a CNAME record via Terraform
func resourceCNAMERecordCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	domain := d.Get("domain").(string)
	target := d.Get("target").(string)
	force := d.Get("force").(bool)

	// Acquire global mutex to serialize all Pi-hole API operations
	pm.Lock()
	defer pm.Unlock()

	opts := &pihole.CreateOptions{Force: force}
	_, err := pm.Client.LocalCNAME().Create(ctx, domain, target, opts)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(domain)

	return diags
}

// resourceCNAMERecordRead retrieves the CNAME record of the associated domain ID
func resourceCNAMERecordRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	// Read operations also acquire the mutex to prevent reads during writes
	pm.Lock()
	defer pm.Unlock()

	record, err := pm.Client.LocalCNAME().Get(ctx, d.Id())
	if err != nil {
		if errors.Is(err, pihole.ErrCNAMENotFound) {
			d.SetId("")
			return nil
		}

		return diag.FromErr(err)
	}

	if err = d.Set("domain", record.Domain); err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("target", record.Target); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

// resourceCNAMERecordDelete handles the deletion of a CNAME record via Terraform
func resourceCNAMERecordDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	// Acquire global mutex to serialize all Pi-hole API operations
	pm.Lock()
	defer pm.Unlock()

	if err := pm.Client.LocalCNAME().Delete(ctx, d.Id()); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diags
}
