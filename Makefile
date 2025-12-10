.PHONY: help test testall lint docs docker-run release-snapshot

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

default: help

test: ## Run unit tests
	go test ./...

testall: ## Run all tests including acceptance tests (requires PIHOLE_URL and PIHOLE_PASSWORD)
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

lint: ## Run linter
	golangci-lint run ./...

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
