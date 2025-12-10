package pihole

// DNSRecord represents a local DNS A record
type DNSRecord struct {
	Domain string
	IP     string
}

// CNAMERecord represents a CNAME record
type CNAMERecord struct {
	Domain string
	Target string
}

// Config contains the configuration for creating a Pi-hole client
type Config struct {
	// BaseURL is the Pi-hole server URL (e.g., "http://pi.hole")
	BaseURL string

	// Password is the admin password for authentication
	Password string

	// UserAgent is sent with HTTP requests
	UserAgent string

	// CAFile is an optional path to a CA certificate for TLS
	CAFile string

	// InsecureSkipVerify disables TLS certificate verification.
	// WARNING: This is insecure and should only be used for testing
	// or in environments where you trust the network.
	InsecureSkipVerify bool

	// SessionID can be provided to reuse an existing session
	SessionID string
}
