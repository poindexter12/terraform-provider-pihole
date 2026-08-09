package provider

import (
	"net"
	"testing"

	"github.com/hashicorp/go-cty/cty"
)

// FuzzValidateDomain exercises the domain validator with arbitrary input.
// Invariants: never panics, and anything it accepts satisfies the documented
// length bounds.
func FuzzValidateDomain(f *testing.F) {
	f.Add("example.lan")
	f.Add("")
	f.Add("-leading.dash")
	f.Add("a.very.deeply.nested.sub.domain.example.com")
	f.Add("under_score.lan")
	f.Add("💥.emoji")

	validate := validateDomain()

	f.Fuzz(func(t *testing.T, domain string) {
		diags := validate(domain, cty.Path{})

		if !diags.HasError() {
			if len(domain) == 0 || len(domain) > 253 {
				t.Fatalf("validator accepted %q with invalid length %d", domain, len(domain))
			}
		}
	})
}

// FuzzValidateIPAddress checks the IP validator never panics and always
// agrees with net.ParseIP.
func FuzzValidateIPAddress(f *testing.F) {
	f.Add("192.168.1.1")
	f.Add("::1")
	f.Add("999.999.999.999")
	f.Add("")
	f.Add("1.2.3.4.5")

	validate := validateIPAddress()

	f.Fuzz(func(t *testing.T, ip string) {
		diags := validate(ip, cty.Path{})

		parseable := net.ParseIP(ip) != nil
		accepted := !diags.HasError()

		if parseable != accepted {
			t.Fatalf("validator (%v) disagrees with net.ParseIP (%v) for %q", accepted, parseable, ip)
		}
	})
}
