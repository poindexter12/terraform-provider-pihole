# Contributing

Thanks for your interest in improving the Pi-hole Terraform provider! Contributions are welcome via GitHub pull requests; bugs and feature requests via [GitHub issues](https://github.com/poindexter12/terraform-provider-pihole/issues). Security vulnerabilities should instead follow [SECURITY.md](SECURITY.md).

## Requirements for acceptable contributions

All changes land through pull requests against `main` (direct pushes are disabled) and must satisfy the required CI checks before merge:

- **Conventional-commit PR titles** — enforced by CI. Use the type prefixes seen in the history: `feat:`, `fix:`, `docs:`, `test:`, `chore:`. Example: `feat: add pihole_password resource`.
- **Lint** — `make lint` (golangci-lint) must pass with zero issues. Code must be `gofmt`-formatted and pass `go vet`.
- **Tests** — `make test` (unit) and the acceptance suite (`make testall`, run in CI against real Pi-hole instances on the oldest-supported and latest images) must pass. **New functionality must come with tests** — see the existing `*_test.go` files for the acceptance-test pattern, and `internal/pihole/v6/*_test.go` for unit tests against the mock Pi-hole server.
- **Vulnerability scan** — `govulncheck` must report no reachable vulnerabilities.
- **Documentation** — docs in `docs/` are generated; after changing schema descriptions or examples, run `make docs` and commit the result. Do not edit `docs/` by hand.

## Development workflow

```sh
go build .            # build the provider
make test             # unit tests
make docker-run       # start a local Pi-hole for acceptance tests
PIHOLE_URL=http://localhost:8080 PIHOLE_PASSWORD=test make testall
make lint             # lint
make docs             # regenerate documentation
```

See the [README](README.md#provider-development) for local provider installation and further testing details.

## Code style

Match the surrounding code: standard Go idioms, error wrapping with `%w`, and the existing service/resource patterns in `internal/pihole/` and `internal/provider/`. Keep comments to what the code cannot express itself.

## License

By contributing, you agree that your contributions are licensed under the project's [MPL-2.0 license](LICENSE) (inbound = outbound).
