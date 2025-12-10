package provider

import (
	"fmt"
	"net"
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// domainRegex matches valid domain names.
// Allows alphanumeric characters, hyphens, underscores, and dots.
// Each label must start and end with alphanumeric characters.
var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`)

// validateDomain returns a schema validation function for domain names.
func validateDomain() schema.SchemaValidateDiagFunc {
	return validation.ToDiagFunc(validation.All(
		validation.StringLenBetween(1, 253),
		validation.StringMatch(domainRegex, "must be a valid domain name"),
	))
}

// validateIPAddress returns a schema validation function for IP addresses.
// Accepts both IPv4 and IPv6 addresses.
func validateIPAddress() schema.SchemaValidateDiagFunc {
	return validation.ToDiagFunc(func(v interface{}, k string) (warnings []string, errors []error) {
		value := v.(string)
		if ip := net.ParseIP(value); ip == nil {
			errors = append(errors, fmt.Errorf("%q is not a valid IP address: %s", k, value))
		}
		return warnings, errors
	})
}
