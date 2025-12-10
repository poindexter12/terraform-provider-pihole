package pihole

import "errors"

var (
	// ErrDNSNotFound is returned when a DNS record is not found
	ErrDNSNotFound = errors.New("local DNS record not found")

	// ErrCNAMENotFound is returned when a CNAME record is not found
	ErrCNAMENotFound = errors.New("local CNAME record not found")

	// ErrAuthFailed is returned when authentication fails
	ErrAuthFailed = errors.New("authentication failed")

	// ErrSessionNotFound is returned when session ID is not in auth response
	ErrSessionNotFound = errors.New("session ID not found in response")

	// ErrClientNotFound is returned when a client record is not found
	ErrClientNotFound = errors.New("client not found")
)
