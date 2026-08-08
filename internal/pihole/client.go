package pihole

import "context"

// Client is the interface for Pi-hole API operations.
// Implementations exist for different Pi-hole versions (v5, v6).
type Client interface {
	// LocalDNS returns the service for managing local DNS A records
	LocalDNS() LocalDNSService

	// LocalCNAME returns the service for managing CNAME records
	LocalCNAME() LocalCNAMEService

	// ClientManagement returns the service for managing Pi-hole clients
	ClientManagement() ClientManagementService

	// Password returns the service for managing the Pi-hole admin password
	Password() PasswordService

	// SessionID returns the current session ID (for reuse across provider instances)
	SessionID() string

	// Logout terminates the current session with Pi-hole.
	// This should be called when the provider is done to free up session slots.
	Logout(ctx context.Context) error
}

// CreateOptions contains optional parameters for record creation
type CreateOptions struct {
	// Force attempts to force record creation. Note: Pi-hole v6 API currently
	// does not implement this for DNS/CNAME endpoints, but it's included for
	// forward compatibility with future Pi-hole versions.
	Force bool
}

// LocalDNSService manages local DNS A records (domain -> IP mappings)
type LocalDNSService interface {
	Create(ctx context.Context, domain, ip string, opts *CreateOptions) (*DNSRecord, error)
	Get(ctx context.Context, domain string) (*DNSRecord, error)
	List(ctx context.Context) ([]DNSRecord, error)
	Delete(ctx context.Context, domain string) error
}

// LocalCNAMEService manages CNAME records (domain -> target domain mappings)
type LocalCNAMEService interface {
	Create(ctx context.Context, domain, target string, opts *CreateOptions) (*CNAMERecord, error)
	Get(ctx context.Context, domain string) (*CNAMERecord, error)
	List(ctx context.Context) ([]CNAMERecord, error)
	Delete(ctx context.Context, domain string) error
}

// PasswordService manages the Pi-hole admin password.
type PasswordService interface {
	// Update sets the admin password. Pi-hole invalidates all sessions
	// (including the caller's) when the password changes, so implementations
	// must re-establish the client session before returning. Blocks until the
	// change is verified against the auth endpoint or ctx expires.
	Update(ctx context.Context, newPassword string) error

	// GetHash returns the current password hash (webserver.api.pwhash).
	// The hash is stable for an unchanged password, making it usable for
	// out-of-band change detection.
	GetHash(ctx context.Context) (string, error)
}

// ClientManagementService manages Pi-hole client configurations.
// Clients can be IP addresses, MAC addresses, hostnames, CIDR ranges, or interface names.
type ClientManagementService interface {
	Create(ctx context.Context, client, comment string) (*ClientRecord, error)
	Get(ctx context.Context, client string) (*ClientRecord, error)
	List(ctx context.Context) ([]ClientRecord, error)
	Update(ctx context.Context, client, comment string) (*ClientRecord, error)
	Delete(ctx context.Context, client string) error
}
