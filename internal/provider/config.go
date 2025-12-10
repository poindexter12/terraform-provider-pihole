package provider

import (
	"context"

	"github.com/poindexter12/terraform-provider-pihole/internal/pihole"
	v6 "github.com/poindexter12/terraform-provider-pihole/internal/pihole/v6"
)

// Config defines the configuration options for the Pi-hole client
type Config struct {
	// The Pi-hole URL
	URL string

	// The Pi-hole admin password
	Password string

	// UserAgent for requests
	UserAgent string

	// Custom CA file
	CAFile string

	// InsecureSkipVerify disables TLS certificate verification
	InsecureSkipVerify bool

	// SessionID can be passed to reduce the number of requests against the /api/auth endpoint
	SessionID string
}

func (c Config) Client(ctx context.Context) (pihole.Client, error) {
	return v6.NewClient(ctx, pihole.Config{
		BaseURL:            c.URL,
		Password:           c.Password,
		UserAgent:          c.UserAgent,
		CAFile:             c.CAFile,
		InsecureSkipVerify: c.InsecureSkipVerify,
		SessionID:          c.SessionID,
	})
}
