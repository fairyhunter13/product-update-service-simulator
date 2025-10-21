SHELL := /bin/bash

# Defaults
COVERAGE_THRESHOLD ?= 80.0
GOBIN ?= $(shell go env GOPATH)/bin
BIN_DIR := $(CURDIR)/bin
GOLANGCI_VERSION ?= 2.5.0
XUNIT_VIEWER_VERSION ?= 10

.PHONY: tools tools-security fmt fmt-check lint lint-all vet docs-validate test-unit test-non-integration \
	coverage-enforce docker-build compose-up compose-itest compose-down compose-integration \
	security-govulncheck security-gosec security-trivy-fs security-trivy-image security-gitleaks \
	reports-unit-junit reports-integration-junit reports-html security-hadolint

# Install local tooling
tools:
	@echo "Installing developer tools..."
	GO111MODULE=on go install mvdan.cc/gofumpt@latest
	GO111MODULE=on go install github.com/jstemmer/go-junit-report/v2@latest

# Security tools (Go-based)
tools-security:
	@echo "Installing security tools..."
	GO111MODULE=on go install golang.org/x/vuln/cmd/govulncheck@latest
	GO111MODULE=on go install github.com/securego/gosec/v2/cmd/gosec@latest

# Formatting
fmt:
	$(GOBIN)/gofumpt -w .

fmt-check:
	@out=$$($(GOBIN)/gofumpt -l .); \
	echo "$$out"; \
	test -z "$$out"

# Linting (requires golangci-lint installed locally)
golangci-download:
	@mkdir -p $(BIN_DIR)
	@echo "Downloading golangci-lint v$(GOLANGCI_VERSION)..."
	@curl -sSL -o $(BIN_DIR)/golangci-lint.tgz https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_VERSION)/golangci-lint-$(GOLANGCI_VERSION)-linux-amd64.tar.gz
	@tar -xzf $(BIN_DIR)/golangci-lint.tgz -C $(BIN_DIR)
	@cp $(BIN_DIR)/golangci-lint-$(GOLANGCI_VERSION)-linux-amd64/golangci-lint $(BIN_DIR)/golangci-lint
	@chmod +x $(BIN_DIR)/golangci-lint
	@rm -rf $(BIN_DIR)/golangci-lint-$(GOLANGCI_VERSION)-linux-amd64 $(BIN_DIR)/golangci-lint.tgz

lint:
	@set -e; \
	if [ -x "$(GOBIN)/golangci-lint" ]; then LINT_BIN="$(GOBIN)/golangci-lint"; \
	elif command -v golangci-lint >/dev/null 2>&1; then LINT_BIN="golangci-lint"; \
	elif [ -x "$(BIN_DIR)/golangci-lint" ]; then LINT_BIN="$(BIN_DIR)/golangci-lint"; \
	else $(MAKE) golangci-download; LINT_BIN="$(BIN_DIR)/golangci-lint"; fi; \
	echo "Using $${LINT_BIN} for linting"; \
	"$${LINT_BIN}" run --timeout=5m

# Containerized golangci-lint (matches CI)
lint-ci:
	docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:v2.5.0 golangci-lint run --timeout=5m

# Aggregate all linters (code + Dockerfile) and vet
lint-all: fmt-check vet lint-ci security-hadolint

# Vet
vet:
	go vet ./...

# Docs validation: ensure README.md has required sections and links
docs-validate:
	@echo "Validating README.md required sections..."
	@grep -q "^## Setup Instructions" README.md
	@grep -q "^## Design Choices" README.md
	@grep -q "^## Production Considerations" README.md
	@grep -q "^## Troubleshooting Strategies" README.md
	@grep -q "^## API" README.md
	@grep -q "^## Reports (GitHub Pages)" README.md
	@grep -q "/openapi.yaml" README.md
	@grep -q "/docs" README.md
	@grep -q "Test%20Reports-GitHub%20Pages" README.md

# Testing
# Unit tests over internal packages with race and coverage output
test-unit:
	go test ./internal/... -race -covermode=atomic -coverprofile=coverage.out

# Race tests for all non-integration packages
# Excludes root-level test/integration
test-non-integration:
	pkgs=$$(go list ./... | grep -v '^.*/test/integration$$'); \
	go test $$pkgs -race

# Enforce coverage threshold using coverage.out
coverage-enforce:
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ {print substr($$NF, 1, length($$NF)-1)}'); \
	echo "Total coverage: $$total%"; \
	awk -v t="$(COVERAGE_THRESHOLD)" -v c="$$total" 'BEGIN { if (c+0 < t+0) { exit 1 } }'

# Docker

docker-build:
	docker build -f build/Dockerfile -t product-update-service-simulator:ci .

# Compose
compose-up:
	docker compose up -d app

compose-itest:
	docker compose run --rm itest

compose-down:
	docker compose down -v

compose-integration: compose-up compose-itest compose-down

# Security - Go
security-govulncheck:
	$(GOBIN)/govulncheck ./...

security-gosec:
	$(GOBIN)/gosec ./...

# Security - Container and repo scanning via Trivy (using Docker image)
# Filesystem scan of the repo
security-trivy-fs:
	docker run --rm -v $(PWD):/src -w /src aquasec/trivy:latest fs --no-progress --exit-code 1 --severity HIGH,CRITICAL .

# Image scan (build image first so it exists on the runner/local machine)
security-trivy-image: docker-build
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:latest image --no-progress --exit-code 1 --severity HIGH,CRITICAL product-update-service-simulator:ci

# Secret scanning via gitleaks (optional but recommended)
security-gitleaks:
	docker run --rm -v $(PWD):/repo zricethezav/gitleaks:latest detect --source=/repo --no-git -v --exit-code 1

# Dockerfile lint via hadolint (containerized)
security-hadolint:
	docker run --rm -v $(PWD):/workspace hadolint/hadolint hadolint /workspace/build/Dockerfile

# Reports (JUnit + HTML)
reports-unit-junit:
	mkdir -p reports/unit
	set -o pipefail; go test -v ./internal/... 2>&1 | $(GOBIN)/go-junit-report -set-exit-code -out reports/unit/unit.xml

reports-integration-junit:
	mkdir -p reports/integration
	$(MAKE) compose-up
	set -o pipefail; docker compose run --rm itest 2>&1 | $(GOBIN)/go-junit-report -set-exit-code -out reports/integration/integration.xml
	$(MAKE) compose-down

reports-html:
    mkdir -p _site
    npx --yes xunit-viewer@$(XUNIT_VIEWER_VERSION) -r reports -o _site/index.html
    npx --yes xunit-viewer@$(XUNIT_VIEWER_VERSION) -r reports/unit -o _site/unit.html
    npx --yes xunit-viewer@$(XUNIT_VIEWER_VERSION) -r reports/integration -o _site/integration.html
    VERSION=$$(git describe --tags --exact-match 2>/dev/null || echo latest); \
      mkdir -p _site/$$VERSION; \
      cp _site/index.html _site/$$VERSION/index.html; \
      cp _site/unit.html _site/$$VERSION/unit.html; \
      cp _site/integration.html _site/$$VERSION/integration.html; \
      cp -r reports _site/

# Publish OpenAPI + Swagger UI to Pages
pages-openapi:
	mkdir -p _site/api
	cp internal/http/openapi/openapi.yaml _site/api/openapi.yaml
	printf '%s\n' \
	  '<!doctype html>' \
	  '<html>' \
	  '  <head>' \
	  '    <meta charset="utf-8" />' \
	  '    <meta name="viewport" content="width=device-width, initial-scale=1" />' \
	  '    <title>Product Update Service API Docs</title>' \
	  '    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />' \
	  '    <style> body { margin: 0; } </style>' \
	  '  </head>' \
	  '  <body>' \
	  '    <div id="swagger-ui"></div>' \
	  '    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>' \
	  '    <script>' \
	  '      window.onload = () => {' \
	  '        window.ui = SwaggerUIBundle({' \
	  '          url: '\''openapi.yaml'\'', ' \
	  '          dom_id: '\''#swagger-ui'\'', ' \
	  '          presets: [SwaggerUIBundle.presets.apis],' \
	  '        });' \
	  '      };' \
	  '    </script>' \
	  '  </body>' \
	  '</html>' > _site/api/index.html
	# Versioned copy
	VERSION=$$(git describe --tags --exact-match 2>/dev/null || echo latest); \
	mkdir -p _site/$$VERSION/api; \
	cp _site/api/openapi.yaml _site/$$VERSION/api/openapi.yaml; \
	cp _site/api/index.html _site/$$VERSION/api/index.html
