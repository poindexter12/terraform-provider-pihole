package provider

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourceClients returns a schema resource for listing Pi-hole clients
func dataSourceClients() *schema.Resource {
	return &schema.Resource{
		Description: "List all Pi-hole client configurations",
		ReadContext: dataSourceClientsRead,
		Schema: map[string]*schema.Schema{
			"clients": {
				Description: "List of Pi-hole client configurations",
				Type:        schema.TypeSet,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"client": {
							Description: "Client identifier (IP address, MAC address, hostname, CIDR range, or interface name)",
							Type:        schema.TypeString,
							Computed:    true,
						},
						"comment": {
							Description: "Comment for the client",
							Type:        schema.TypeString,
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

// dataSourceClientsRead lists all Pi-hole client configurations
func dataSourceClientsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	pm, diags := getProviderMeta(meta)
	if diags != nil {
		return diags
	}

	pm.Lock()
	defer pm.Unlock()

	clientList, err := pm.Client.ClientManagement().List(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	list := make([]map[string]interface{}, len(clientList))
	idRef := ""

	for i, c := range clientList {
		idRef = fmt.Sprintf("%s|%s|", idRef, c.Client)

		list[i] = map[string]interface{}{
			"client":  c.Client,
			"comment": c.Comment,
		}
	}

	if err := d.Set("clients", list); err != nil {
		return diag.FromErr(err)
	}

	hash := sha256.Sum256([]byte(idRef))
	d.SetId(fmt.Sprintf("%x", hash[:]))

	return diags
}
