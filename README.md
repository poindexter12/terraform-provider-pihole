# terraform-provider-pihole

[![Tests](https://github.com/poindexter12/terraform-provider-pihole/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/poindexter12/terraform-provider-pihole/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/poindexter12/terraform-provider-pihole)](https://goreportcard.com/report/github.com/poindexter12/terraform-provider-pihole)
[![Terraform Registry](https://img.shields.io/badge/terraform-registry-blueviolet)](https://registry.terraform.io/providers/poindexter12/pihole/latest)

> **Note:** This is a fork of [ryanwholey/terraform-provider-pihole](https://github.com/ryanwholey/terraform-provider-pihole).
> Credit to [@ryanwholey](https://github.com/ryanwholey) for the original implementation.

> **Pi-hole v6 Support:** This fork has been updated to work with Pi-hole v6's new REST API. It is not compatible with Pi-hole v5.

[Pi-hole](https://pi-hole.net/) is an ad blocking application which acts as a DNS proxy that returns empty responses when DNS requests for known advertisement domains are made from your devices. It has a number of additional capabilities like optional DHCP server capabilities, specific allow/deny profiles for specific clients, and a neat UI with a ton of information regarding your internet traffic.

Pi-hole is an open source project and can be found at https://github.com/pi-hole/pi-hole.

## Usage

Install the provider from the [Terraform Registry](https://registry.terraform.io/providers/poindexter12/pihole/latest):

```tf
terraform {
  required_providers {
    pihole = {
      source  = "poindexter12/pihole"
      version = "~> 1.1"
    }
  }
}
```

Configure the provider with credentials, or pass environment variables:

```tf
provider "pihole" {
  url       = "https://pihole.domain.com" # PIHOLE_URL
  password  = var.pihole_password         # PIHOLE_PASSWORD

  # Optional TLS settings
  # ca_file              = "/path/to/ca.crt"  # PIHOLE_CA_FILE
  # insecure_skip_verify = false              # Skip TLS verification (not recommended)
}
```

## Provider Development

There are a few ways to configure local providers. See the somewhat obscure [Terraform plugin installation documentation](https://www.terraform.io/docs/cli/commands/init.html#plugin-installation) for a potential recommended way.

One way to run a local provider is to build the project, move it to the Terraform plugins directory and then use a `required_providers` block to note the address and version.

> [!NOTE]
> Note the `/darwin_arm64/` path portion targets a Mac with an ARM64 processor,
> see https://github.com/poindexter12/terraform-provider-pihole/blob/main/.goreleaser.yml#L18-L27
> for possible supported combinations.

```sh
# from the project root
go build .

mkdir -p ~/.terraform.d/plugins/terraform.local/local/pihole/0.0.1/darwin_arm64/

cp terraform-provider-pihole ~/.terraform.d/plugins/terraform.local/local/pihole/0.0.1/darwin_arm64/terraform-provider-pihole_v0.0.1
```

In the Terraform workspace, use a `required_providers` block to target the locally built provider

```tf
terraform {
  required_providers {
    pihole = {
      source  = "terraform.local/local/pihole"
      version = "0.0.1"
    }
  }
}
```

### Testing

Testing a Terraform provider comes in several forms. This chapter will attempt to explain the differences, where to find documentation, and how to contribute.

> [!NOTE]
> For the current tests in this repository the SDKv2 is used.

#### Unit testing
```sh
make test
```

#### Acceptance testing

The `make testall` command is prefixed with the `TF_ACC=1`. This tells go to include the tests that utilise the `helper/resource.Test()` functions.

For further reading, please see Hashicorp's [documenation](https://developer.hashicorp.com/terraform/plugin/sdkv2/testing/acceptance-tests) on acceptance tests.

To setup a proper environment combining an instance of Pihole in a docker container with tests, some environment variables need to be set for the tests to make their requests to the correct location.

Run the following commands to test against a local Pi-hole server via [docker](https://docs.docker.com/engine/install/)
```sh
# Set the local Terraform provider environment variables
export PIHOLE_URL=http://localhost:8080
export PIHOLE_PASSWORD=test

# Start the pi-hole server
make docker-run

# Run Terraform tests against the server
make testall
```

To test against a specific Pi-hole image tag, specify the tag via the `TAG` env var

```sh
TAG=nightly make docker-run
```

For further reading about Terraform acceptance tests, see Hashicorp's [documenation](https://developer.hashicorp.com/terraform/plugin/sdkv2/testing/acceptance-tests) on acceptance tests.


### Docs

Documentation is auto-generated via [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs) from description fields within the provider package, as well as examples and templates from the `examples/` and `templates/` folders respectively.

To generate the docs run

```sh
make docs
```
