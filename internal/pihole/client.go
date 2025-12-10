package pihole

import "context"

// Client is the interface for Pi-hole API operations.
// Implementations exist for different Pi-hole versions (v5, v6).
type Client interface {
	// LocalDNS returns the service for managing local DNS A records
	LocalDNS() LocalDNSService

	// LocalCNAME returns the service for managing CNAME records
	LocalCNAME() LocalCNAMEService

	// SessionID returns the current session ID (for reuse across provider instances)
	SessionID() string
}

// LocalDNSService manages local DNS A records (domain -> IP mappings)
type LocalDNSService interface {
	Create(ctx context.Context, domain, ip string) (*DNSRecord, error)
	Get(ctx context.Context, domain string) (*DNSRecord, error)
	List(ctx context.Context) ([]DNSRecord, error)
	Delete(ctx context.Context, domain string) error
}

// LocalCNAMEService manages CNAME records (domain -> target domain mappings)
type LocalCNAMEService interface {
	Create(ctx context.Context, domain, target string) (*CNAMERecord, error)
	Get(ctx context.Context, domain string) (*CNAMERecord, error)
	List(ctx context.Context) ([]CNAMERecord, error)
	Delete(ctx context.Context, domain string) error
}
