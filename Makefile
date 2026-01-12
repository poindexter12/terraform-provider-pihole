.PHONY: help test testall lint docs docker-run release-snapshot tools

# Tool versions
GOLANGCI_LINT_VERSION ?= v1.64.0

# Local bin directory for tools
BIN_DIR := $(CURDIR)/bin
export PATH := $(BIN_DIR):$(PATH)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

default: help

test: ## Run unit tests
	go test ./...

testall: ## Run all tests including acceptance tests (requires PIHOLE_URL and PIHOLE_PASSWORD)
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

tools: ## Install development tools locally to ./bin
	@mkdir -p $(BIN_DIR)
	GOBIN=$(BIN_DIR) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: $(BIN_DIR)/golangci-lint ## Run linter
	$(BIN_DIR)/golangci-lint run ./... && echo "✓ Lint passed"

$(BIN_DIR)/golangci-lint:
	@$(MAKE) tools

docs: ## Generate Terraform documentation
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.19.4

docker-run: ## Start Pi-hole in Docker for testing
	docker compose up -d --build

release-snapshot: ## Test release build locally (no publish, no signing)
	goreleaser release --snapshot --clean --skip=sign

release: ## Create and push a release tag (prompts for version)
	@read -p "Version (e.g., 1.0.0): " version; \
	git tag "v$$version" && \
	git push origin "v$$version" && \
	echo "Tag v$$version pushed. GitHub Actions will build and publish the release."
