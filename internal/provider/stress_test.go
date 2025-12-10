package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccStressBulkCreate tests creating many DNS and CNAME records simultaneously.
// This exercises the global mutex to ensure no race conditions during bulk operations.
func TestAccStressBulkCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckLocalDNSDestroy,
		Steps: []resource.TestStep{
			{
				Config: testStressBulkCreateConfig(100),
				Check: resource.ComposeTestCheckFunc(
					// Verify first and last DNS records
					resource.TestCheckResourceAttr("pihole_dns_record.stress_dns_0", "domain", "stress-dns-0.local"),
					resource.TestCheckResourceAttr("pihole_dns_record.stress_dns_0", "ip", "10.0.0.0"),
					resource.TestCheckResourceAttr("pihole_dns_record.stress_dns_99", "domain", "stress-dns-99.local"),
					resource.TestCheckResourceAttr("pihole_dns_record.stress_dns_99", "ip", "10.0.0.99"),
					// Verify first and last CNAME records
					resource.TestCheckResourceAttr("pihole_cname_record.stress_cname_0", "domain", "stress-cname-0.local"),
					resource.TestCheckResourceAttr("pihole_cname_record.stress_cname_0", "target", "target-0.local"),
					resource.TestCheckResourceAttr("pihole_cname_record.stress_cname_99", "domain", "stress-cname-99.local"),
					resource.TestCheckResourceAttr("pihole_cname_record.stress_cname_99", "target", "target-99.local"),
				),
			},
		},
	})
}

// TestAccStressBulkDelete tests deleting many records at once by reducing the count
func TestAccStressBulkDelete(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckLocalDNSDestroy,
		Steps: []resource.TestStep{
			// First create 100 records (200 total: 100 DNS + 100 CNAME)
			{
				Config: testStressBulkCreateConfig(100),
			},
			// Then reduce to 10 - this triggers deletion of 90 DNS and 90 CNAME records (180 deletes)
			{
				Config: testStressBulkCreateConfig(10),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.stress_dns_0", "domain", "stress-dns-0.local"),
					resource.TestCheckResourceAttr("pihole_dns_record.stress_dns_9", "domain", "stress-dns-9.local"),
					resource.TestCheckResourceAttr("pihole_cname_record.stress_cname_0", "domain", "stress-cname-0.local"),
					resource.TestCheckResourceAttr("pihole_cname_record.stress_cname_9", "domain", "stress-cname-9.local"),
				),
			},
		},
	})
}

// TestAccStressForceNew tests ForceNew behavior by changing IP/target values
// This triggers delete+create sequences that must be atomic
func TestAccStressForceNew(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckLocalDNSDestroy,
		Steps: []resource.TestStep{
			// Create initial records
			{
				Config: testStressForceNewConfig("10.0.0.1", "target-a.local"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.forcenew", "ip", "10.0.0.1"),
					resource.TestCheckResourceAttr("pihole_cname_record.forcenew", "target", "target-a.local"),
				),
			},
			// Change values - triggers ForceNew (delete old, create new)
			{
				Config: testStressForceNewConfig("10.0.0.2", "target-b.local"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.forcenew", "ip", "10.0.0.2"),
					resource.TestCheckResourceAttr("pihole_cname_record.forcenew", "target", "target-b.local"),
				),
			},
			// Change again to stress the mutex
			{
				Config: testStressForceNewConfig("10.0.0.3", "target-c.local"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.forcenew", "ip", "10.0.0.3"),
					resource.TestCheckResourceAttr("pihole_cname_record.forcenew", "target", "target-c.local"),
				),
			},
		},
	})
}

// TestAccStressMixedOperations tests a mix of creates, updates (ForceNew), and deletes
func TestAccStressMixedOperations(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckLocalDNSDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create 50 DNS and 50 CNAME records (100 total)
			{
				Config: testStressMixedConfig(50, "10.1.0", "target-v1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_0", "ip", "10.1.0.0"),
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_49", "ip", "10.1.0.49"),
					resource.TestCheckResourceAttr("pihole_cname_record.mixed_0", "target", "target-v1-0.local"),
				),
			},
			// Step 2: ForceNew on all 100 records by changing IP base and target prefix (100 deletes + 100 creates)
			{
				Config: testStressMixedConfig(50, "10.2.0", "target-v2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_0", "ip", "10.2.0.0"),
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_49", "ip", "10.2.0.49"),
					resource.TestCheckResourceAttr("pihole_cname_record.mixed_0", "target", "target-v2-0.local"),
				),
			},
			// Step 3: Reduce count (80 deletes) while also changing values (ForceNew on remaining 20)
			{
				Config: testStressMixedConfig(10, "10.3.0", "target-v3"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_0", "ip", "10.3.0.0"),
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_9", "ip", "10.3.0.9"),
					resource.TestCheckResourceAttr("pihole_cname_record.mixed_0", "target", "target-v3-0.local"),
					resource.TestCheckResourceAttr("pihole_cname_record.mixed_9", "target", "target-v3-9.local"),
				),
			},
			// Step 4: Increase count (creates) while changing values (ForceNew on existing)
			{
				Config: testStressMixedConfig(100, "10.4.0", "target-v4"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_0", "ip", "10.4.0.0"),
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_99", "ip", "10.4.0.99"),
					resource.TestCheckResourceAttr("pihole_cname_record.mixed_0", "target", "target-v4-0.local"),
					resource.TestCheckResourceAttr("pihole_cname_record.mixed_99", "target", "target-v4-99.local"),
				),
			},
		},
	})
}

// TestAccStressRapidReplace tests rapid sequential replacements
// This exercises the mutex heavily by doing many ForceNew cycles back to back
func TestAccStressRapidReplace(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckLocalDNSDestroy,
		Steps: []resource.TestStep{
			{
				Config: testStressRapidConfig("10.0.1"),
			},
			{
				Config: testStressRapidConfig("10.0.2"),
			},
			{
				Config: testStressRapidConfig("10.0.3"),
			},
			{
				Config: testStressRapidConfig("10.0.4"),
			},
			{
				Config: testStressRapidConfig("10.0.5"),
			},
		},
	})
}

// testStressBulkCreateConfig generates config for n DNS and n CNAME records
func testStressBulkCreateConfig(count int) string {
	config := ""
	for i := 0; i < count; i++ {
		config += fmt.Sprintf(`
resource "pihole_dns_record" "stress_dns_%d" {
  domain = "stress-dns-%d.local"
  ip     = "10.0.0.%d"
}

resource "pihole_cname_record" "stress_cname_%d" {
  domain = "stress-cname-%d.local"
  target = "target-%d.local"
}
`, i, i, i, i, i, i)
	}
	return config
}

// testStressForceNewConfig generates config for testing ForceNew
func testStressForceNewConfig(ip string, target string) string {
	return fmt.Sprintf(`
resource "pihole_dns_record" "forcenew" {
  domain = "forcenew-dns.local"
  ip     = %q
}

resource "pihole_cname_record" "forcenew" {
  domain = "forcenew-cname.local"
  target = %q
}
`, ip, target)
}

// testStressMixedConfig generates config for mixed operation testing
func testStressMixedConfig(count int, ipBase string, targetPrefix string) string {
	config := ""
	for i := 0; i < count; i++ {
		config += fmt.Sprintf(`
resource "pihole_dns_record" "mixed_%d" {
  domain = "mixed-dns-%d.local"
  ip     = "%s.%d"
}

resource "pihole_cname_record" "mixed_%d" {
  domain = "mixed-cname-%d.local"
  target = "%s-%d.local"
}
`, i, i, ipBase, i, i, i, targetPrefix, i)
	}
	return config
}

// testStressRapidConfig generates config for rapid replacement testing
func testStressRapidConfig(ipBase string) string {
	config := ""
	for i := 0; i < 5; i++ {
		config += fmt.Sprintf(`
resource "pihole_dns_record" "rapid_%d" {
  domain = "rapid-dns-%d.local"
  ip     = "%s.%d"
}

resource "pihole_cname_record" "rapid_%d" {
  domain = "rapid-cname-%d.local"
  target = "%s-target-%d.local"
}
`, i, i, ipBase, i, i, i, ipBase, i)
	}
	return config
}
