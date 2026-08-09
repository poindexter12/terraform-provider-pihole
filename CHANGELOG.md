# Changelog

## [1.2.0](https://github.com/poindexter12/terraform-provider-pihole/releases/tag/v1.2.0) (2026-08-08)

### Features

* **New `pihole_password` resource** ([#25](https://github.com/poindexter12/terraform-provider-pihole/issues/25)) - Manage the Pi-hole admin password through Terraform. Updates in place, verifies propagation by polling the auth endpoint instead of sleeping, and detects out-of-band password changes via the stored `webserver.api.pwhash`. Destroy is deliberately a no-op: clearing the password would disable Pi-hole authentication entirely. To rotate the password in a single apply, authenticate the provider with a Pi-hole app password (see the resource documentation for both rotation patterns).

### Bug Fixes

* **Transparent session recovery** - The client re-authenticates and retries once when a request returns 401. This handles Pi-hole invalidating all sessions on any password change (including the delayed second purge observed on older FTL versions such as 2025.03.0), idle session expiry (`webserver.session.timeout`, 30 minutes by default) during long applies, and stale external session IDs.

### Improvements

* Mark the provider `password` argument as sensitive

### Security

* Patch reachable dependency vulnerabilities: `google.golang.org/grpc` → v1.82.1, `golang.org/x/text` → v0.39.0, `github.com/cloudflare/circl` → v1.6.3; govulncheck reports no reachable vulnerabilities
* Add CodeQL, OpenSSF Scorecard, govulncheck, and Go native fuzzing to CI; pin all GitHub Actions to commit SHAs with least-privilege workflow permissions
* Add `SECURITY.md` vulnerability reporting policy and weekly grouped Dependabot updates

### Notes

* Acceptance tests now run against both the oldest supported Pi-hole v6 image (`2025.03.0`) and `latest` on every PR
* Go toolchain follows `go.mod` (currently 1.25) across all CI workflows; golangci-lint migrated to v2

---

## [1.1.0](https://github.com/poindexter12/terraform-provider-pihole/releases/tag/v1.1.0) (2025-12-16)

### Bug Fixes

* **Session cleanup restored** - Fixed session logout that was accidentally removed during the ProviderMeta refactor. Sessions are now properly terminated when Terraform exits, preventing "API seats exceeded" (429) errors from stale session accumulation.

* **Force delete-before-create for DNS records** - The `force` attribute now works correctly for DNS records. When `force = true` and a record already exists, the provider deletes it before creating the new one. This fixes "item already present" errors during `ForceNew` operations.

### Notes

* The `force` flag implements client-side delete-before-create because the Pi-hole v6 API doesn't support a force query parameter for DNS endpoints
* External sessions passed via `__PIHOLE_SESSION_ID` environment variable are not logged out, allowing session reuse across multiple Terraform runs

---

## [1.0.0](https://github.com/poindexter12/terraform-provider-pihole/releases/tag/v1.0.0) (2025-12-10)

First stable release with Pi-hole v6 support.

### Breaking Changes

* **Pi-hole v6 only** - This provider requires Pi-hole v6's REST API and is not compatible with Pi-hole v5

### Features

* Add `pihole_client` resource for managing Pi-hole client configurations
* Add `pihole_clients` data source to list all clients
* Add `force` attribute to DNS and CNAME resources for handling duplicates during ForceNew operations
* Add `insecure_skip_verify` provider option for self-signed certificates
* Add input validation for domains and IP addresses

### Improvements

* Implement internal Pi-hole v6 API client (replaces go-pihole dependency)
* Add retry logic for transient API errors during concurrent operations
* Add global operation mutex for atomic ForceNew operations
* Make delete operations idempotent (404 treated as success)

### Credits

This provider is a fork of [ryanwholey/terraform-provider-pihole](https://github.com/ryanwholey/terraform-provider-pihole). Credit to [@ryanwholey](https://github.com/ryanwholey) for the original implementation.

---

## Previous Releases (upstream)

The following releases are from the original [ryanwholey/terraform-provider-pihole](https://github.com/ryanwholey/terraform-provider-pihole) repository for Pi-hole v5.

### 0.0.11 (2022-02-19)

* Bump go-pihole@0.0.3

### 0.0.10 (2022-02-19)

* Add experimental API token support for DNS and CNAME resources

### 0.0.8 (2022-02-03)

* Handle login and provider configuration errors
* Use correct resource name for domains example

### 0.0.7 (2021-11-24)

* Add list domains data source

### 0.0.6 (2021-11-04)

* On read, unset ID when resource not found

### 0.0.4 (2021-11-02)

* Add initial groups client and list data source

### 0.0.3 (2021-11-01)

* Add CNAME record resource

### 0.0.1 (2021-11-01)

* Initial release
