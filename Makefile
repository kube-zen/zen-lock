.PHONY: build build-cli build-controller build-image test fmt vet lint clean deploy verify ci-check

# Build both CLI and controller
build: build-cli build-controller

# Build the CLI binary
build-cli:
	@echo "Building zen-lock CLI..."
	@mkdir -p bin
	go build -ldflags="-s -w" -trimpath -o bin/zen-lock ./cmd/cli
	@echo "✅ CLI build complete: bin/zen-lock"
	@ls -lh bin/zen-lock | awk '{print "   Binary size: " $$5}'

# Build the webhook controller binary
build-controller:
	@echo "Building zen-lock webhook controller..."
	@mkdir -p bin
	go build -ldflags="-s -w" -trimpath -o bin/zen-lock-webhook ./cmd/webhook
	@echo "✅ Controller build complete: bin/zen-lock-webhook"
	@ls -lh bin/zen-lock-webhook | awk '{print "   Binary size: " $$5}'

# Build optimized binaries for production
build-release:
	@echo "Building optimized binaries..."
	@mkdir -p bin
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-alpha"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	go build -trimpath \
		-ldflags "-s -w \
			-X 'main.version=$$VERSION' \
			-X 'main.commit=$$COMMIT' \
			-X 'main.buildDate=$$BUILD_DATE'" \
		-o bin/zen-lock ./cmd/cli; \
	go build -trimpath \
		-ldflags "-s -w \
			-X 'main.version=$$VERSION' \
			-X 'main.commit=$$COMMIT' \
			-X 'main.buildDate=$$BUILD_DATE'" \
		-o bin/zen-lock-webhook ./cmd/webhook
	@echo "✅ Optimized builds complete"
	@ls -lh bin/

# Build Docker image for webhook controller
build-image:
	@echo "Building Docker image..."
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-alpha"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	docker build \
		--build-arg VERSION=$$VERSION \
		--build-arg COMMIT=$$COMMIT \
		--build-arg BUILD_DATE=$$BUILD_DATE \
		-t kubezen/zen-lock:$$VERSION \
		-t kubezen/zen-lock:latest .
	@echo "✅ Docker image built: kubezen/zen-lock:$$VERSION"

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic -timeout=10m ./pkg/...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -timeout=5m ./test/integration/...

# Run E2E tests (requires kubebuilder binaries)
# Install kubebuilder: https://kubebuilder.io/docs/getting-started/installation/
test-e2e:
	@echo "Running E2E tests..."
	@if [ -z "$$(go list -f '{{.Dir}}' -m sigs.k8s.io/controller-runtime 2>/dev/null)" ]; then \
		echo "⚠️  E2E tests require envtest. Install: go get sigs.k8s.io/controller-runtime/pkg/envtest"; \
		exit 1; \
	fi
	@if ! command -v kubebuilder >/dev/null 2>&1 && [ ! -d "/usr/local/kubebuilder" ]; then \
		echo "⚠️  E2E tests require kubebuilder binaries."; \
		echo "   Install: https://kubebuilder.io/docs/getting-started/installation/"; \
		echo "   Or set KUBEBUILDER_ASSETS environment variable"; \
		exit 1; \
	fi
	go test -v -tags=e2e -timeout=30m ./test/e2e/...

# Show test coverage
coverage: test-unit
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"
	@echo "Coverage summary:"
	@go tool cover -func=coverage.out | tail -1
	@echo ""
	@echo "Checking coverage threshold (minimum: 40%, target: 75%)..."
	@COVERAGE=$$(go tool cover -func=coverage.out | grep -v "pkg/apis" | grep "total:" | awk '{print $$3}' | sed 's/%//'); \
	if [ -z "$$COVERAGE" ]; then \
		echo "⚠️  Could not determine coverage percentage"; \
	elif [ $$(echo "$$COVERAGE < 40" | bc -l 2>/dev/null || echo "0") -eq 1 ]; then \
		echo "❌ Coverage $$COVERAGE% is below the 40% minimum threshold"; \
		echo "⚠️  Target coverage is 75%, currently at $$COVERAGE%"; \
		exit 1; \
	elif [ $$(echo "$$COVERAGE < 75" | bc -l 2>/dev/null || echo "0") -eq 1 ]; then \
		echo "⚠️  Coverage $$COVERAGE% is below the 75% target (minimum: 40%)"; \
		echo "✅ Coverage $$COVERAGE% meets the 40% minimum threshold"; \
	else \
		echo "✅ Coverage $$COVERAGE% meets the 75% target threshold"; \
	fi

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✅ Code formatted"

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...
	@echo "✅ go vet passed"

# Run linter (requires golangci-lint)
lint:
	@echo "Running golangci-lint..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "⚠️  golangci-lint not found. Installing..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest; \
	fi
	golangci-lint run
	@echo "✅ Linting passed"

# Check formatting
check-fmt:
	@echo "Checking code formatting..."
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "❌ Code is not formatted. Run 'make fmt'"; \
		gofmt -s -d .; \
		exit 1; \
	fi
	@echo "✅ Code formatting check passed"

# Check go mod tidy
check-mod:
	@echo "Checking go.mod..."
	@go mod tidy
	@if ! git diff --exit-code go.mod go.sum >/dev/null 2>&1; then \
		echo "❌ go.mod or go.sum needs updates. Run 'go mod tidy'"; \
		git diff go.mod go.sum; \
		exit 1; \
	fi
	@echo "✅ go.mod check passed"

# Verify code compiles
verify: check-fmt check-mod vet
	@echo "Verifying code compiles..."
	go build ./...
	@echo "✅ Code compiles successfully"

# Security checks
security-check:
	@echo "Running security checks..."
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "Installing govulncheck..."; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	govulncheck ./...
	@echo "✅ Security check passed"

# CI check (runs all checks)
ci-check: verify lint test-unit security-check
	@echo "✅ All CI checks passed"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/ coverage.out coverage.html
	@echo "✅ Clean complete"

# Deploy CRD
deploy-crd:
	@echo "Deploying CRD..."
	kubectl apply -f config/crd/bases/
	@echo "✅ CRD deployed"

# Deploy all manifests
deploy: deploy-crd
	@echo "Deploying manifests..."
	kubectl apply -f config/webhook/
	kubectl apply -f config/rbac/
	@echo "✅ Manifests deployed"

# Run webhook locally (requires kubeconfig)
run:
	@echo "Running webhook locally..."
	go run ./cmd/webhook

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies installed"

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest; \
	fi
	@echo "✅ Development tools installed"

# Generate CRD manifests
generate:
	@echo "Generating CRD manifests..."
	@if ! command -v controller-gen >/dev/null 2>&1; then \
		echo "Installing controller-gen..."; \
		go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest; \
	fi
	controller-gen rbac:roleName=zen-lock-manager crd webhook paths="./pkg/apis/..." output:crd:artifacts:config=config/crd/bases
	@echo "✅ CRD manifests generated"

# Helm chart operations
helm-lint:
	@echo "Linting Helm chart..."
	@if ! command -v helm >/dev/null 2>&1; then \
		echo "❌ Helm not installed. Install from https://helm.sh/docs/intro/install/"; \
		exit 1; \
	fi
	helm lint charts/zen-lock
	@echo "✅ Helm chart lint passed"

helm-package:
	@echo "Packaging Helm chart..."
	@if ! command -v helm >/dev/null 2>&1; then \
		echo "❌ Helm not installed. Install from https://helm.sh/docs/intro/install/"; \
		exit 1; \
	fi
	@mkdir -p .helm-packages
	helm package charts/zen-lock -d .helm-packages/
	@echo "✅ Helm chart packaged"

helm-test:
	@echo "Testing Helm chart rendering..."
	@if ! command -v helm >/dev/null 2>&1; then \
		echo "❌ Helm not installed. Install from https://helm.sh/docs/intro/install/"; \
		exit 1; \
	fi
	helm template test-release charts/zen-lock --debug > /dev/null
	@echo "✅ Helm chart renders successfully"

helm-repo-index:
	@echo "Generating Helm repository index..."
	@if ! command -v helm >/dev/null 2>&1; then \
		echo "❌ Helm not installed. Install from https://helm.sh/docs/intro/install/"; \
		exit 1; \
	fi
	@if [ ! -d ".helm-packages" ] || [ -z "$$(ls -A .helm-packages/*.tgz 2>/dev/null)" ]; then \
		echo "❌ No packaged charts found. Run 'make helm-package' first"; \
		exit 1; \
	fi
	helm repo index .helm-packages --url https://kube-zen.github.io/zen-lock
	@echo "✅ Helm repository index generated"

helm-all: helm-lint helm-test helm-package helm-repo-index
	@echo "✅ All Helm tasks complete"

