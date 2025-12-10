# Changelog

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
