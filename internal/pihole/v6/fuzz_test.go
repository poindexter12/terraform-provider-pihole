package v6

import (
	"strings"
	"testing"
)

// FuzzParseDNSHosts exercises the "IP domain" parser that consumes strings
// from Pi-hole API responses. Invariants: never panics, never returns more
// records than inputs, and every record round-trips to an entry that was in
// the input.
func FuzzParseDNSHosts(f *testing.F) {
	f.Add("192.168.1.1 example.lan")
	f.Add("::1 v6.example.lan")
	f.Add("no-space-entry")
	f.Add("")
	f.Add("1.2.3.4 host with extra spaces")

	f.Fuzz(func(t *testing.T, entry string) {
		records := parseDNSHosts([]string{entry})

		if len(records) > 1 {
			t.Fatalf("one input produced %d records", len(records))
		}

		for _, r := range records {
			if r.IP+" "+r.Domain != entry {
				t.Fatalf("record %+v does not round-trip to input %q", r, entry)
			}
			if strings.Contains(r.IP, " ") {
				t.Fatalf("IP %q contains a space", r.IP)
			}
		}

		// Entries without a separator must be dropped, not mangled
		if !strings.Contains(entry, " ") && len(records) != 0 {
			t.Fatalf("entry %q without separator produced a record", entry)
		}
	})
}

// FuzzParseCNAMEs exercises the "domain,target" parser with the same
// invariants as FuzzParseDNSHosts.
func FuzzParseCNAMEs(f *testing.F) {
	f.Add("alias.lan,target.lan")
	f.Add("no-comma-entry")
	f.Add("")
	f.Add("a,b,c")
	f.Add(",")

	f.Fuzz(func(t *testing.T, entry string) {
		records := parseCNAMEs([]string{entry})

		if len(records) > 1 {
			t.Fatalf("one input produced %d records", len(records))
		}

		for _, r := range records {
			if r.Domain+","+r.Target != entry {
				t.Fatalf("record %+v does not round-trip to input %q", r, entry)
			}
			if strings.Contains(r.Domain, ",") {
				t.Fatalf("domain %q contains a comma", r.Domain)
			}
		}

		if !strings.Contains(entry, ",") && len(records) != 0 {
			t.Fatalf("entry %q without separator produced a record", entry)
		}
	})
}
