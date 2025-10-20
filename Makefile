SHELL := /bin/bash

# Defaults
COVERAGE_THRESHOLD ?= 80.0
GOBIN ?= $(shell go env GOPATH)/bin

.PHONY: tools tools-security fmt fmt-check lint vet test-unit test-non-integration \
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
lint:
	golangci-lint run --timeout=5m

# Vet
vet:
	go vet ./...

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

# Image scan (expects docker-build already executed)
security-trivy-image:
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
	# Combined dashboard
	npx --yes junit-viewer --results=reports --save _site/index.html
	# Unit-only and Integration-only pages
	npx --yes junit-viewer --results=reports/unit --save _site/unit.html
	npx --yes junit-viewer --results=reports/integration --save _site/integration.html
	# Versioned history folder: use tag if present, else 'latest'
	VERSION=$$(git describe --tags --exact-match 2>/dev/null || echo latest); \
	mkdir -p _site/$$VERSION; \
	cp _site/index.html _site/$$VERSION/index.html; \
	cp _site/unit.html _site/$$VERSION/unit.html; \
	cp _site/integration.html _site/$$VERSION/integration.html; \
	cp -r reports _site/
