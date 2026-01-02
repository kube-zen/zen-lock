#!/bin/bash
# Setup script for zen-lock integration tests with kind
# Creates a kind cluster, deploys CRDs, RBAC, and zen-lock controller/webhook

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-zen-lock-integration}"
KUBECONFIG_PATH="${KUBECONFIG_PATH:-${HOME}/.kube/zen-lock-integration-config}"

log_info() {
    echo "[INFO] $*" >&2
}

log_error() {
    echo "[ERROR] $*" >&2
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing=0
    
    if ! command -v kind >/dev/null 2>&1; then
        log_error "kind is not installed. Install from https://kind.sigs.k8s.io/"
        missing=1
    fi
    
    if ! command -v kubectl >/dev/null 2>&1; then
        log_error "kubectl is not installed"
        missing=1
    fi
    
    if ! command -v docker >/dev/null 2>&1 && ! command -v podman >/dev/null 2>&1; then
        log_error "docker or podman is required"
        missing=1
    fi
    
    if [ $missing -eq 1 ]; then
        exit 1
    fi
    
    log_info "All prerequisites met"
}

create_cluster() {
    log_info "Creating kind cluster: $CLUSTER_NAME"
    
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        log_info "Cluster $CLUSTER_NAME already exists"
        return 0
    fi
    
    kind create cluster \
        --name "$CLUSTER_NAME" \
        --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 8080
    protocol: TCP
  - containerPort: 443
    hostPort: 8443
    protocol: TCP
EOF
    
    log_info "Cluster created successfully"
}

export_kubeconfig() {
    log_info "Exporting kubeconfig..."
    kind get kubeconfig --name "$CLUSTER_NAME" > "$KUBECONFIG_PATH"
    log_info "Kubeconfig exported to: $KUBECONFIG_PATH"
    log_info "To use this cluster, run: export KUBECONFIG=$KUBECONFIG_PATH"
}

install_crds() {
    log_info "Installing CRDs..."
    kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f "$PROJECT_ROOT/config/crd/bases/"
    log_info "Waiting for CRDs to be established..."
    kubectl wait --kubeconfig="$KUBECONFIG_PATH" --for=condition=established --timeout=60s \
        crd/zenlocks.security.kube-zen.io || true
    log_info "CRDs installed"
}

install_rbac() {
    log_info "Installing RBAC..."
    # Create namespace first (required for ServiceAccounts)
    kubectl create namespace zen-lock-system --kubeconfig="$KUBECONFIG_PATH" --dry-run=client -o yaml | \
        kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f -
    kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f "$PROJECT_ROOT/config/rbac/"
    log_info "RBAC installed"
}

build_and_load_image() {
    log_info "Building zen-lock image..."
    
    local image_name="kubezen/zen-lock:integration-test"
    
    # Build image
    docker build -t "$image_name" "$PROJECT_ROOT" || {
        log_error "Failed to build image"
        exit 1
    }
    
    # Load into kind
    log_info "Loading image into kind cluster..."
    kind load docker-image "$image_name" --name "$CLUSTER_NAME" || {
        log_error "Failed to load image into kind"
        exit 1
    }
    
    log_info "Image built and loaded: $image_name"
    echo "$image_name"
}

deploy_zen_lock() {
    log_info "Deploying zen-lock..."
    
    local image_name="${1:-kubezen/zen-lock:integration-test}"
    local namespace="zen-lock-system"
    
    # Create namespace
    kubectl create namespace "$namespace" --kubeconfig="$KUBECONFIG_PATH" --dry-run=client -o yaml | \
        kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f -
    
    # Generate test private key if not set
    if [ -z "${ZEN_LOCK_PRIVATE_KEY:-}" ]; then
        log_info "Generating test private key..."
        # Use a simple test key for integration tests
        export ZEN_LOCK_PRIVATE_KEY="AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE"
    fi
    
    # Create Secret with private key
    kubectl create secret generic zen-lock-master-key \
        --from-literal=key.txt="$ZEN_LOCK_PRIVATE_KEY" \
        --namespace="$namespace" \
        --kubeconfig="$KUBECONFIG_PATH" \
        --dry-run=client -o yaml | \
        kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f -
    
    # Create Deployment manifest
    cat <<EOF | kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-lock-webhook
  namespace: $namespace
  labels:
    app.kubernetes.io/name: zen-lock
    app.kubernetes.io/component: webhook
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: zen-lock
      app.kubernetes.io/component: webhook
  template:
    metadata:
      labels:
        app.kubernetes.io/name: zen-lock
        app.kubernetes.io/component: webhook
    spec:
      serviceAccountName: zen-lock-webhook
      containers:
      - name: webhook
        image: $image_name
        imagePullPolicy: Never
        command: ["/zen-lock-webhook"]
        args:
        - --enable-webhook=true
        - --enable-controller=false
        env:
        - name: ZEN_LOCK_PRIVATE_KEY
          valueFrom:
            secretKeyRef:
              name: zen-lock-master-key
              key: key.txt
        ports:
        - containerPort: 9443
          name: webhook-server
          protocol: TCP
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 256Mi
        readinessProbe:
          httpGet:
            path: /readyz
            port: 9443
            scheme: HTTPS
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /healthz
            port: 9443
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 30
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-lock-controller
  namespace: $namespace
  labels:
    app.kubernetes.io/name: zen-lock
    app.kubernetes.io/component: controller
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: zen-lock
      app.kubernetes.io/component: controller
  template:
    metadata:
      labels:
        app.kubernetes.io/name: zen-lock
        app.kubernetes.io/component: controller
    spec:
      serviceAccountName: zen-lock-controller
      containers:
      - name: controller
        image: $image_name
        imagePullPolicy: Never
        command: ["/zen-lock-webhook"]
        args:
        - --enable-webhook=false
        - --enable-controller=true
        - --leader-election-id=zen-lock-controller-leader-election
        env:
        - name: ZEN_LOCK_PRIVATE_KEY
          valueFrom:
            secretKeyRef:
              name: zen-lock-master-key
              key: key.txt
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 256Mi
EOF
    
    log_info "Waiting for deployments to be ready..."
    kubectl wait --kubeconfig="$KUBECONFIG_PATH" \
        --for=condition=available \
        --timeout=120s \
        deployment/zen-lock-webhook \
        deployment/zen-lock-controller \
        -n "$namespace" || {
        log_error "Deployments failed to become ready"
        kubectl get pods --kubeconfig="$KUBECONFIG_PATH" -n "$namespace"
        exit 1
    }
    
    log_info "zen-lock deployed successfully"
}

install_webhook() {
    log_info "Installing webhook configuration..."
    kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f "$PROJECT_ROOT/config/webhook/"
    log_info "Webhook installed"
}

wait_for_ready() {
    log_info "Waiting for zen-lock to be ready..."
    local namespace="zen-lock-system"
    
    # Wait for webhook pod
    kubectl wait --kubeconfig="$KUBECONFIG_PATH" \
        --for=condition=ready \
        --timeout=60s \
        pod -l app.kubernetes.io/name=zen-lock,app.kubernetes.io/component=webhook \
        -n "$namespace" || {
        log_error "Webhook pod not ready"
        kubectl logs --kubeconfig="$KUBECONFIG_PATH" \
            -l app.kubernetes.io/name=zen-lock,app.kubernetes.io/component=webhook \
            -n "$namespace" || true
        exit 1
    }
    
    # Wait for controller pod
    kubectl wait --kubeconfig="$KUBECONFIG_PATH" \
        --for=condition=ready \
        --timeout=60s \
        pod -l app.kubernetes.io/name=zen-lock,app.kubernetes.io/component=controller \
        -n "$namespace" || {
        log_error "Controller pod not ready"
        kubectl logs --kubeconfig="$KUBECONFIG_PATH" \
            -l app.kubernetes.io/name=zen-lock,app.kubernetes.io/component=controller \
            -n "$namespace" || true
        exit 1
    }
    
    log_info "zen-lock is ready"
}

cleanup_cluster() {
    log_info "Cleaning up cluster..."
    kind delete cluster --name "$CLUSTER_NAME" || true
    rm -f "$KUBECONFIG_PATH"
    log_info "Cleanup complete"
}

print_usage() {
    cat <<EOF
Usage: $0 [COMMAND]

Commands:
    create      Create kind cluster and deploy zen-lock
    delete      Delete kind cluster
    kubeconfig  Show kubeconfig export command
    help        Show this help message

Environment Variables:
    CLUSTER_NAME       Name of the kind cluster (default: zen-lock-integration)
    KUBECONFIG_PATH    Path to kubeconfig file (default: ~/.kube/zen-lock-integration-config)
    ZEN_LOCK_PRIVATE_KEY  Private key for zen-lock (will generate if not set)

Examples:
    # Create cluster and deploy zen-lock
    $0 create

    # Delete cluster
    $0 delete

    # Export kubeconfig
    export KUBECONFIG=\$($0 kubeconfig)
EOF
}

main() {
    case "${1:-help}" in
        create)
            check_prerequisites
            create_cluster
            export_kubeconfig
            install_crds
            install_rbac
            image_name=$(build_and_load_image)
            deploy_zen_lock "$image_name"
            install_webhook
            wait_for_ready
            log_info "✅ zen-lock integration test environment is ready!"
            log_info "Export kubeconfig: export KUBECONFIG=$KUBECONFIG_PATH"
            ;;
        delete)
            cleanup_cluster
            ;;
        kubeconfig)
            echo "$KUBECONFIG_PATH"
            ;;
        help|--help|-h)
            print_usage
            ;;
        *)
            log_error "Unknown command: $1"
            print_usage
            exit 1
            ;;
    esac
}

main "$@"

