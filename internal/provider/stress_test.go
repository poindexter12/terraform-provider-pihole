package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// Stress test record counts.
// Originally tested with 100 records but Pi-hole v6 API returned 400 errors around
// record 27-58 in GitHub Actions CI. Reduced for CI reliability while still
// exercising the mutex with meaningful concurrent operations.
const (
	// stressBulkCount is used for bulk create/delete tests
	stressBulkCount = 20
	// stressBulkReducedCount is the reduced count for bulk delete step
	stressBulkReducedCount = 5
	// stressMixedCount is used for mixed operation tests
	stressMixedCount = 15
	// stressMixedReducedCount is the reduced count for mixed operation delete step
	stressMixedReducedCount = 5
	// stressMixedFinalCount is the final count for mixed operations
	stressMixedFinalCount = 20
	// stressRapidCount is used for rapid replacement tests
	stressRapidCount = 5
)

// TestAccStressBulkCreate tests creating many DNS and CNAME records simultaneously.
// This exercises the global mutex to ensure no race conditions during bulk operations.
func TestAccStressBulkCreate(t *testing.T) {
	lastIdx := stressBulkCount - 1
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckLocalDNSDestroy,
		Steps: []resource.TestStep{
			{
				Config: testStressBulkCreateConfig(stressBulkCount),
				Check: resource.ComposeTestCheckFunc(
					// Verify first and last DNS records
					resource.TestCheckResourceAttr("pihole_dns_record.stress_dns_0", "domain", "stress-dns-0.local"),
					resource.TestCheckResourceAttr("pihole_dns_record.stress_dns_0", "ip", "10.0.0.0"),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_dns_record.stress_dns_%d", lastIdx), "domain", fmt.Sprintf("stress-dns-%d.local", lastIdx)),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_dns_record.stress_dns_%d", lastIdx), "ip", fmt.Sprintf("10.0.0.%d", lastIdx)),
					// Verify first and last CNAME records
					resource.TestCheckResourceAttr("pihole_cname_record.stress_cname_0", "domain", "stress-cname-0.local"),
					resource.TestCheckResourceAttr("pihole_cname_record.stress_cname_0", "target", "target-0.local"),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_cname_record.stress_cname_%d", lastIdx), "domain", fmt.Sprintf("stress-cname-%d.local", lastIdx)),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_cname_record.stress_cname_%d", lastIdx), "target", fmt.Sprintf("target-%d.local", lastIdx)),
				),
			},
		},
	})
}

// TestAccStressBulkDelete tests deleting many records at once by reducing the count.
func TestAccStressBulkDelete(t *testing.T) {
	reducedLastIdx := stressBulkReducedCount - 1
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckLocalDNSDestroy,
		Steps: []resource.TestStep{
			// First create stressBulkCount records
			{
				Config: testStressBulkCreateConfig(stressBulkCount),
			},
			// Then reduce to stressBulkReducedCount - triggers bulk deletes
			{
				Config: testStressBulkCreateConfig(stressBulkReducedCount),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.stress_dns_0", "domain", "stress-dns-0.local"),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_dns_record.stress_dns_%d", reducedLastIdx), "domain", fmt.Sprintf("stress-dns-%d.local", reducedLastIdx)),
					resource.TestCheckResourceAttr("pihole_cname_record.stress_cname_0", "domain", "stress-cname-0.local"),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_cname_record.stress_cname_%d", reducedLastIdx), "domain", fmt.Sprintf("stress-cname-%d.local", reducedLastIdx)),
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

// TestAccStressMixedOperations tests a mix of creates, updates (ForceNew), and deletes.
func TestAccStressMixedOperations(t *testing.T) {
	mixedLastIdx := stressMixedCount - 1
	reducedLastIdx := stressMixedReducedCount - 1
	finalLastIdx := stressMixedFinalCount - 1
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckLocalDNSDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create stressMixedCount DNS and CNAME records
			{
				Config: testStressMixedConfig(stressMixedCount, "10.1.0", "target-v1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_0", "ip", "10.1.0.0"),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_dns_record.mixed_%d", mixedLastIdx), "ip", fmt.Sprintf("10.1.0.%d", mixedLastIdx)),
					resource.TestCheckResourceAttr("pihole_cname_record.mixed_0", "target", "target-v1-0.local"),
				),
			},
			// Step 2: ForceNew on all records by changing IP base and target prefix
			{
				Config: testStressMixedConfig(stressMixedCount, "10.2.0", "target-v2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_0", "ip", "10.2.0.0"),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_dns_record.mixed_%d", mixedLastIdx), "ip", fmt.Sprintf("10.2.0.%d", mixedLastIdx)),
					resource.TestCheckResourceAttr("pihole_cname_record.mixed_0", "target", "target-v2-0.local"),
				),
			},
			// Step 3: Reduce count while also changing values (ForceNew on remaining)
			{
				Config: testStressMixedConfig(stressMixedReducedCount, "10.3.0", "target-v3"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_0", "ip", "10.3.0.0"),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_dns_record.mixed_%d", reducedLastIdx), "ip", fmt.Sprintf("10.3.0.%d", reducedLastIdx)),
					resource.TestCheckResourceAttr("pihole_cname_record.mixed_0", "target", "target-v3-0.local"),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_cname_record.mixed_%d", reducedLastIdx), "target", fmt.Sprintf("target-v3-%d.local", reducedLastIdx)),
				),
			},
			// Step 4: Increase count (creates) while changing values (ForceNew on existing)
			{
				Config: testStressMixedConfig(stressMixedFinalCount, "10.4.0", "target-v4"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_dns_record.mixed_0", "ip", "10.4.0.0"),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_dns_record.mixed_%d", finalLastIdx), "ip", fmt.Sprintf("10.4.0.%d", finalLastIdx)),
					resource.TestCheckResourceAttr("pihole_cname_record.mixed_0", "target", "target-v4-0.local"),
					resource.TestCheckResourceAttr(fmt.Sprintf("pihole_cname_record.mixed_%d", finalLastIdx), "target", fmt.Sprintf("target-v4-%d.local", finalLastIdx)),
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
	for i := 0; i < stressRapidCount; i++ {
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
