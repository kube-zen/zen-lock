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
		-t kube-zen/zen-lock-webhook:$$VERSION \
		-t kube-zen/zen-lock-webhook:latest .
	@echo "✅ Docker image built: kube-zen/zen-lock-webhook:$$VERSION"

# Run all tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic -timeout=10m ./pkg/...

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

# CI check (runs all checks)
ci-check: verify lint test
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

