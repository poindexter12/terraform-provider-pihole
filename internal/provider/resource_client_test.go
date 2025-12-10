package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
)

// TestAccClient acceptance test for the client resource
func TestAccClient(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckClientDestroy,
		Steps: []resource.TestStep{
			{
				Config: testClientResourceConfig("testclient", "192.168.100.1", "Test client"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_client.testclient", "client", "192.168.100.1"),
					resource.TestCheckResourceAttr("pihole_client.testclient", "comment", "Test client"),
					testCheckClientResourceExists(t, "192.168.100.1", "Test client"),
				),
			},
			// Update comment
			{
				Config: testClientResourceConfig("testclient", "192.168.100.1", "Updated comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_client.testclient", "client", "192.168.100.1"),
					resource.TestCheckResourceAttr("pihole_client.testclient", "comment", "Updated comment"),
					testCheckClientResourceExists(t, "192.168.100.1", "Updated comment"),
				),
			},
		},
	})
}

// TestAccClientEmptyComment tests creating a client with no comment
func TestAccClientEmptyComment(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckClientDestroy,
		Steps: []resource.TestStep{
			{
				Config: testClientResourceConfigNoComment("emptyclient", "192.168.100.2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_client.emptyclient", "client", "192.168.100.2"),
					// Comment defaults to empty string, verified via existence check
					testCheckClientResourceExists(t, "192.168.100.2", ""),
				),
			},
		},
	})
}

// TestAccClientMAC tests creating a client using MAC address
func TestAccClientMAC(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckClientDestroy,
		Steps: []resource.TestStep{
			{
				Config: testClientResourceConfig("macclient", "AA:BB:CC:DD:EE:FF", "MAC address client"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_client.macclient", "client", "AA:BB:CC:DD:EE:FF"),
					resource.TestCheckResourceAttr("pihole_client.macclient", "comment", "MAC address client"),
				),
			},
		},
	})
}

// testClientResourceConfig returns HCL to configure a client resource
func testClientResourceConfig(name, client, comment string) string {
	return fmt.Sprintf(`
		resource "pihole_client" %q {
			client  = %q
			comment = %q
		}
	`, name, client, comment)
}

// testClientResourceConfigNoComment returns HCL to configure a client resource without comment
func testClientResourceConfigNoComment(name, client string) string {
	return fmt.Sprintf(`
		resource "pihole_client" %q {
			client = %q
		}
	`, name, client)
}

// testCheckClientResourceExists checks that the client resource exists in Pi-hole
func testCheckClientResourceExists(_ *testing.T, client, comment string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		pm := testAccProvider.Meta().(*ProviderMeta)

		record, err := pm.Client.ClientManagement().Get(context.Background(), client)
		if err != nil {
			return err
		}

		if record.Comment != comment {
			return fmt.Errorf("requested %s with comment %q does not match: %q", client, comment, record.Comment)
		}

		return nil
	}
}

// TestAccClientStress tests creating many clients simultaneously.
// This verifies the mutex prevents race conditions in the Pi-hole API.
func TestAccClientStress(t *testing.T) {
	const count = 20
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckClientDestroy,
		Steps: []resource.TestStep{
			{
				Config: testClientStressConfig(count),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_client.stress_0", "client", "192.168.200.0"),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_client.stress_%d", count-1), "client", fmt.Sprintf("192.168.200.%d", count-1)),
				),
			},
		},
	})
}

// testClientStressConfig generates config for n clients
func testClientStressConfig(count int) string {
	config := ""
	for i := 0; i < count; i++ {
		config += fmt.Sprintf(`
resource "pihole_client" "stress_%d" {
  client  = "192.168.200.%d"
  comment = "Stress test client %d"
}
`, i, i, i)
	}
	return config
}

// testAccCheckClientDestroy checks that all client resources have been deleted
func testAccCheckClientDestroy(s *terraform.State) error {
	pm := testAccProvider.Meta().(*ProviderMeta)

	for _, r := range s.RootModule().Resources {
		if r.Type != "pihole_client" {
			continue
		}

		if _, err := pm.Client.ClientManagement().Get(context.Background(), r.Primary.ID); err != nil {
			if !errors.Is(err, pihole.ErrClientNotFound) {
				return err
			}
		}
	}
	return nil
}
