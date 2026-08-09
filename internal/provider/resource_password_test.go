package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TestAccPassword exercises the pihole_password resource against the current
// admin password. It intentionally sets the SAME password as PIHOLE_PASSWORD:
// even a same-value set goes through the full code path (config PATCH,
// session invalidation, re-authentication), while leaving the instance
// usable by the rest of the acceptance suite.
func TestAccPassword(t *testing.T) {
	password := os.Getenv("PIHOLE_PASSWORD")

	// Any password set invalidates every Pi-hole session, including the one
	// cached in __PIHOLE_SESSION_ID that the rest of the suite reuses. The
	// provider's client re-authenticates itself, so refresh the cached
	// session from it once this test is done.
	t.Cleanup(func() { refreshSharedSession(t) })

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPasswordSurvivesDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPasswordResourceConfig(password),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("pihole_password.admin", "id", "admin-password"),
					resource.TestCheckResourceAttr("pihole_password.admin", "password", password),
					resource.TestCheckResourceAttrSet("pihole_password.admin", "password_hash"),
					// The apply above killed every session, including the one
					// cached in __PIHOLE_SESSION_ID that the framework's
					// post-apply refresh will configure a fresh provider with.
					// The provider's own client re-authenticated during the
					// apply; propagate its live session before the refresh.
					testAccAdoptProviderSession,
				),
			},
			// Re-applying the same config must be a clean no-op
			{
				Config:   testPasswordResourceConfig(password),
				PlanOnly: true,
			},
		},
	})
}

// TestAccPasswordRotation is a client-level integration test for a real
// password rotation. It bypasses the Terraform test framework because the
// framework re-configures the provider from PIHOLE_PASSWORD between steps,
// which cannot follow a rotated password. The original password is restored
// even if the test fails partway.
func TestAccPasswordRotation(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set, skipping integration test")
	}

	url := os.Getenv("PIHOLE_URL")
	original := os.Getenv("PIHOLE_PASSWORD")
	if url == "" || original == "" {
		t.Fatal("PIHOLE_URL and PIHOLE_PASSWORD must be set")
	}

	ctx := context.Background()

	client, err := Config{URL: url, Password: original}.Client(ctx)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	const rotated = "tf-acc-rotation-test-pw"

	restored := false
	defer func() {
		if restored {
			return
		}
		if err := client.Password().Update(ctx, original); err != nil {
			t.Errorf("FAILED TO RESTORE ORIGINAL PASSWORD, subsequent tests will fail: %v", err)
		}
	}()

	hashBefore, err := client.Password().GetHash(ctx)
	if err != nil {
		t.Fatalf("GetHash before rotation: %v", err)
	}

	if err := client.Password().Update(ctx, rotated); err != nil {
		t.Fatalf("rotation: %v", err)
	}

	hashAfter, err := client.Password().GetHash(ctx)
	if err != nil {
		t.Fatalf("GetHash after rotation (session should have been refreshed): %v", err)
	}
	if hashAfter == hashBefore {
		t.Error("expected password hash to change after rotation")
	}

	// The old password must no longer authenticate
	if _, err := (Config{URL: url, Password: original}).Client(ctx); err == nil {
		t.Error("expected authentication with the old password to fail after rotation")
	}

	// Restore and verify the original password authenticates again
	if err := client.Password().Update(ctx, original); err != nil {
		t.Fatalf("restore: %v", err)
	}
	restored = true

	restoredClient, err := Config{URL: url, Password: original}.Client(ctx)
	if err != nil {
		t.Fatalf("authentication with restored password failed: %v", err)
	}
	if err := restoredClient.Logout(ctx); err != nil {
		t.Logf("logout of verification session failed: %v", err)
	}

	if err := client.Logout(ctx); err != nil {
		t.Logf("logout failed: %v", err)
	}

	// This test rotated the password twice, killing all sessions each time,
	// including the shared __PIHOLE_SESSION_ID used by the rest of the suite.
	refreshSharedSession(t)
}

// testPasswordResourceConfig returns HCL to configure the password resource
func testPasswordResourceConfig(password string) string {
	return fmt.Sprintf(`
		resource "pihole_password" "admin" {
			password = %q
		}
	`, password)
}

// testAccCheckPasswordSurvivesDestroy verifies the no-op delete contract:
// after destroy, the password must still be set and usable.
func testAccCheckPasswordSurvivesDestroy(*terraform.State) error {
	pm := testAccProvider.Meta().(*ProviderMeta)

	hash, err := pm.Client.Password().GetHash(context.Background())
	if err != nil {
		return fmt.Errorf("expected password to survive destroy, API call failed: %w", err)
	}
	if hash == "" {
		return fmt.Errorf("expected password to survive destroy, but no password hash is set")
	}

	return nil
}

// testAccAdoptProviderSession copies the configured provider client's live
// session ID into __PIHOLE_SESSION_ID so that subsequently configured
// provider instances (post-apply refresh, later steps) reuse a valid session.
func testAccAdoptProviderSession(*terraform.State) error {
	pm := testAccProvider.Meta().(*ProviderMeta)

	sid := pm.Client.SessionID()
	if sid == "" {
		return fmt.Errorf("provider client has no session ID to adopt")
	}

	return os.Setenv("__PIHOLE_SESSION_ID", sid)
}

// refreshSharedSession replaces the cached __PIHOLE_SESSION_ID (invalidated
// by any password set) with a freshly authenticated one so that acceptance
// tests running after a password test keep working.
func refreshSharedSession(t *testing.T) {
	t.Helper()

	if os.Getenv("__PIHOLE_SESSION_ID") == "" {
		return
	}

	client, err := Config{
		URL:      os.Getenv("PIHOLE_URL"),
		Password: os.Getenv("PIHOLE_PASSWORD"),
	}.Client(context.Background())
	if err != nil {
		t.Fatalf("failed to refresh shared session: %v", err)
	}

	if err := os.Setenv("__PIHOLE_SESSION_ID", client.SessionID()); err != nil {
		t.Fatalf("failed to update __PIHOLE_SESSION_ID: %v", err)
	}
}
