package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccClientsData(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: `
					resource "pihole_client" "test" {
					  client  = "192.168.50.1"
					  comment = "Test client for data source"
					}

					data "pihole_clients" "all" {
					  depends_on = [pihole_client.test]
					}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.pihole_clients.all", "clients.#"),
				),
			},
		},
	})
}
