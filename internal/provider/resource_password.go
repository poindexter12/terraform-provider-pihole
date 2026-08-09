package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// passwordResourceID is the singleton ID for the password resource. Pi-hole
// has exactly one admin password, so only one instance of this resource is
// meaningful per provider.
const passwordResourceID = "admin-password"

// resourcePassword returns the admin password Terraform resource management configuration
func resourcePassword() *schema.Resource {
	return &schema.Resource{
		Description: "Manages the Pi-hole admin password. " +
			"Setting the password invalidates all active Pi-hole sessions; the provider transparently re-authenticates. " +
			"Destroying this resource only removes it from state — the password remains set, since the only alternative " +
			"(an empty password) would disable Pi-hole authentication entirely. " +
			"Note: the provider must authenticate with the current password to change it. To rotate the password in a " +
			"single apply, authenticate the provider with a Pi-hole app password.",
		CreateContext: resourcePasswordCreate,
		ReadContext:   resourcePasswordRead,
		UpdateContext: resourcePasswordUpdate,
		DeleteContext: resourcePasswordDelete,
		Importer: &schema.ResourceImporter{
			// The password itself is write-only in the Pi-hole API and cannot
			// be imported; only the hash is adopted (in Read). The first plan
			// after import shows an update asserting the configured password.
			StateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
				d.SetId(passwordResourceID)
				return []*schema.ResourceData{d}, nil
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(2 * time.Minute),
			Update: schema.DefaultTimeout(2 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"password": {
				Description:      "The Pi-hole admin password. Must not be empty: Pi-hole treats an empty password as authentication disabled.",
				Type:             schema.TypeString,
				Required:         true,
				Sensitive:        true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotEmpty),
			},
			"password_hash": {
				Description: "Hash of the password as stored by Pi-hole (webserver.api.pwhash). " +
					"Used to detect out-of-band password changes.",
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

// setPassword applies the configured password and records the resulting hash
func setPassword(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	password := d.Get("password").(string)

	// Acquire global mutex to serialize all Pi-hole API operations
	pm.Lock()
	defer pm.Unlock()

	if err := pm.Client.Password().Update(ctx, password); err != nil {
		return diag.FromErr(err)
	}

	hash, err := pm.Client.Password().GetHash(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("password_hash", hash); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// resourcePasswordCreate handles setting the admin password via Terraform
func resourcePasswordCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if diags := setPassword(ctx, d, meta); diags != nil {
		return diags
	}

	d.SetId(passwordResourceID)

	return nil
}

// resourcePasswordRead detects out-of-band password changes by comparing the
// stored password hash with the live one. The password itself is write-only in
// the Pi-hole API, so the hash is the only observable signal. On mismatch the
// resource is removed from state, causing Terraform to plan a re-create that
// re-asserts the configured password.
func resourcePasswordRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	// Read operations also acquire the mutex to prevent reads during writes
	pm.Lock()
	defer pm.Unlock()

	hash, err := pm.Client.Password().GetHash(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	stored := d.Get("password_hash").(string)

	// A fresh import has no stored hash yet; adopt the live one. The password
	// attribute is empty in that case, so Terraform still plans an update to
	// assert the configured value.
	if stored == "" || stored == hash {
		if err := d.Set("password_hash", hash); err != nil {
			return diag.FromErr(err)
		}
		return nil
	}

	// Hash changed out-of-band; re-create to re-assert the configured password
	d.SetId("")

	return nil
}

// resourcePasswordUpdate handles changing the admin password via Terraform
func resourcePasswordUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return setPassword(ctx, d, meta)
}

// resourcePasswordDelete removes the resource from state without touching the
// password. Clearing it (Pi-hole's only "unset") would disable authentication
// entirely, which a destroy should never do implicitly.
func resourcePasswordDelete(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
	d.SetId("")

	return nil
}
