#!/bin/bash
# Setup script for zen-lock integration tests with kind or k3d
# Creates a cluster, installs cert-manager, deploys CRDs, RBAC, and zen-lock controller/webhook

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-zen-lock-integration}"
CLUSTER_TYPE="${CLUSTER_TYPE:-kind}"  # kind or k3d
KUBECONFIG_PATH="${KUBECONFIG_PATH:-${HOME}/.kube/${CLUSTER_NAME}-config}"

log_info() {
    echo "[INFO] $*" >&2
}

log_error() {
    echo "[ERROR] $*" >&2
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing=0
    
    if [ "$CLUSTER_TYPE" = "k3d" ]; then
        if ! command -v k3d >/dev/null 2>&1; then
            log_error "k3d is not installed. Install from https://k3d.io/"
            missing=1
        fi
    else
        if ! command -v kind >/dev/null 2>&1; then
            log_error "kind is not installed. Install from https://kind.sigs.k8s.io/"
            missing=1
        fi
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
    log_info "Creating $CLUSTER_TYPE cluster: $CLUSTER_NAME"
    
    if [ "$CLUSTER_TYPE" = "k3d" ]; then
        # Check if k3d cluster exists
        if k3d cluster list | grep -q "^${CLUSTER_NAME}"; then
            log_info "k3d cluster $CLUSTER_NAME already exists"
            return 0
        fi
        
        # Create k3d cluster
        k3d cluster create "$CLUSTER_NAME" || {
            log_error "Failed to create k3d cluster"
            exit 1
        }
        
        # k3d writes kubeconfig automatically, we just need to copy it
        # k3d stores kubeconfig in ~/.config/k3d/kubeconfig-${CLUSTER_NAME}.yaml
        local k3d_kubeconfig="${HOME}/.config/k3d/kubeconfig-${CLUSTER_NAME}.yaml"
        if [ -f "$k3d_kubeconfig" ]; then
            cp "$k3d_kubeconfig" "$KUBECONFIG_PATH" || {
                log_error "Failed to copy kubeconfig"
                exit 1
            }
        else
            # Fallback: write it explicitly
            k3d kubeconfig write "$CLUSTER_NAME" > "$KUBECONFIG_PATH" || {
                log_error "Failed to export kubeconfig"
                exit 1
            }
        fi
        
        log_info "k3d cluster created: $CLUSTER_NAME"
        return 0
    fi
    
    # Default to kind
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        log_info "kind cluster $CLUSTER_NAME already exists"
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
    
    log_info "kind cluster created successfully"
}

export_kubeconfig() {
    log_info "Exporting kubeconfig..."
    
    if [ "$CLUSTER_TYPE" = "k3d" ]; then
        # k3d writes kubeconfig automatically, we just need to copy it
        local k3d_kubeconfig="${HOME}/.config/k3d/kubeconfig-${CLUSTER_NAME}.yaml"
        if [ -f "$k3d_kubeconfig" ]; then
            cp "$k3d_kubeconfig" "$KUBECONFIG_PATH" || {
                log_error "Failed to copy kubeconfig"
                exit 1
            }
        else
            # Fallback: write it explicitly
            k3d kubeconfig write "$CLUSTER_NAME" > "$KUBECONFIG_PATH" || {
                log_error "Failed to export kubeconfig"
                exit 1
            }
        fi
        log_info "Kubeconfig exported to: $KUBECONFIG_PATH"
    else
        kind get kubeconfig --name "$CLUSTER_NAME" > "$KUBECONFIG_PATH"
        log_info "Kubeconfig exported to: $KUBECONFIG_PATH"
    fi
    
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

install_cert_manager() {
    log_info "Installing cert-manager..."
    
    # Check if cert-manager is already installed
    if kubectl get crd certificates.cert-manager.io --kubeconfig="$KUBECONFIG_PATH" >/dev/null 2>&1; then
        log_info "cert-manager already installed"
        return 0
    fi
    
    # Install cert-manager using kubectl
    kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml || {
        log_error "Failed to install cert-manager"
        exit 1
    }
    
    # Wait for cert-manager to be ready
    log_info "Waiting for cert-manager to be ready..."
    kubectl wait --kubeconfig="$KUBECONFIG_PATH" --for=condition=ready pod \
        -l app.kubernetes.io/instance=cert-manager \
        -n cert-manager \
        --timeout=300s || {
        log_error "cert-manager failed to become ready"
        exit 1
    }
    
    log_info "cert-manager installed and ready"
}

install_certificate() {
    log_info "Installing certificate and issuer..."
    
    local namespace="zen-lock-system"
    
    # Create namespace first
    kubectl create namespace "$namespace" --kubeconfig="$KUBECONFIG_PATH" --dry-run=client -o yaml | \
        kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f -
    
    # Apply certificate and issuer
    kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f "$PROJECT_ROOT/config/webhook/certificate.yaml" || {
        log_error "Failed to apply certificate"
        exit 1
    }
    
    # Wait for certificate to be ready
    log_info "Waiting for certificate to be ready..."
    kubectl wait --kubeconfig="$KUBECONFIG_PATH" --for=condition=ready \
        certificate/zen-lock-webhook-cert -n "$namespace" \
        --timeout=120s || {
        log_error "Certificate failed to become ready"
        exit 1
    }
    
    log_info "Certificate installed and ready"
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
    
    # Build image from project root (Dockerfile expects zen-lock/ and zen-sdk/ directories)
    # PROJECT_ROOT is zen-lock/, so parent is zen/ workspace
    local workspace_root="$(cd "$PROJECT_ROOT/.." && pwd)"
    if [ ! -d "$workspace_root/zen-sdk" ]; then
        log_error "zen-sdk directory not found at $workspace_root/zen-sdk"
        log_error "Dockerfile requires both zen-lock and zen-sdk to be in the same parent directory"
        exit 1
    fi
    
    # Build from workspace root
    docker build -f "$PROJECT_ROOT/Dockerfile" -t "$image_name" "$workspace_root" || {
        log_error "Failed to build image"
        exit 1
    }
    
    # Load into cluster based on type
    if [ "$CLUSTER_TYPE" = "k3d" ]; then
        log_info "Loading image into k3d cluster..."
        k3d image import "$image_name" -c "$CLUSTER_NAME" || {
            log_error "Failed to load image into k3d cluster"
            exit 1
        }
    else
        log_info "Loading image into kind cluster..."
        kind load docker-image "$image_name" --name "$CLUSTER_NAME" || {
            log_error "Failed to load image into kind"
            exit 1
        }
    fi
    
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
    
    # Apply webhook deployment from manifests.yaml (includes cert-manager secret mount)
    kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f "$PROJECT_ROOT/config/webhook/manifests.yaml"
    
    # Update image and imagePullPolicy
    kubectl set image deployment/zen-lock-webhook webhook="$image_name" -n "$namespace" --kubeconfig="$KUBECONFIG_PATH"
    kubectl patch deployment zen-lock-webhook -n "$namespace" --kubeconfig="$KUBECONFIG_PATH" \
        --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/imagePullPolicy", "value": "Never"}]' || true
    
    # Apply controller deployment
    kubectl apply --kubeconfig="$KUBECONFIG_PATH" -f "$PROJECT_ROOT/config/webhook/controller-manifests.yaml"
    kubectl set image deployment/zen-lock-controller controller="$image_name" -n "$namespace" --kubeconfig="$KUBECONFIG_PATH"
    kubectl patch deployment zen-lock-controller -n "$namespace" --kubeconfig="$KUBECONFIG_PATH" \
        --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/imagePullPolicy", "value": "Never"}]' || true
    
    log_info "Waiting for deployments to be ready..."
    kubectl wait --kubeconfig="$KUBECONFIG_PATH" \
        --for=condition=available \
        --timeout=180s \
        deployment/zen-lock-webhook \
        deployment/zen-lock-controller \
        -n "$namespace" || {
        log_error "Deployments failed to become ready"
        kubectl get pods --kubeconfig="$KUBECONFIG_PATH" -n "$namespace"
        kubectl describe pod -n "$namespace" --kubeconfig="$KUBECONFIG_PATH" -l app=zen-lock-webhook || true
        exit 1
    }
    
    log_info "zen-lock deployed successfully"
}

wait_for_ready() {
    log_info "Waiting for zen-lock to be ready..."
    local namespace="zen-lock-system"
    
    # Wait for webhook pod
    kubectl wait --kubeconfig="$KUBECONFIG_PATH" \
        --for=condition=ready \
        --timeout=120s \
        pod -l app=zen-lock-webhook \
        -n "$namespace" || {
        log_error "Webhook pod not ready"
        kubectl logs --kubeconfig="$KUBECONFIG_PATH" \
            -l app=zen-lock-webhook \
            -n "$namespace" || true
        exit 1
    }
    
    # Wait for controller pod
    kubectl wait --kubeconfig="$KUBECONFIG_PATH" \
        --for=condition=ready \
        --timeout=120s \
        pod -l app=zen-lock-controller \
        -n "$namespace" || {
        log_error "Controller pod not ready"
        kubectl logs --kubeconfig="$KUBECONFIG_PATH" \
            -l app=zen-lock-controller \
            -n "$namespace" || true
        exit 1
    }
    
    log_info "zen-lock is ready"
}

delete_cluster() {
    log_info "Deleting $CLUSTER_TYPE cluster: $CLUSTER_NAME"
    
    if [ "$CLUSTER_TYPE" = "k3d" ]; then
        if ! k3d cluster list | grep -q "^${CLUSTER_NAME}"; then
            log_info "k3d cluster $CLUSTER_NAME does not exist"
            return 0
        fi
        
        k3d cluster delete "$CLUSTER_NAME" || {
            log_error "Failed to delete k3d cluster"
            exit 1
        }
        
        # Clean up kubeconfig
        if [ -f "$KUBECONFIG_PATH" ]; then
            rm -f "$KUBECONFIG_PATH"
        fi
        
        log_info "k3d cluster deleted: $CLUSTER_NAME"
        return 0
    fi
    
    # Default to kind
    if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        log_info "kind cluster $CLUSTER_NAME does not exist"
        return 0
    fi
    
    kind delete cluster --name "$CLUSTER_NAME" || {
        log_error "Failed to delete kind cluster"
        exit 1
    }
    
    # Clean up kubeconfig
    if [ -f "$KUBECONFIG_PATH" ]; then
        rm -f "$KUBECONFIG_PATH"
    fi
    
    log_info "kind cluster deleted: $CLUSTER_NAME"
}

print_usage() {
    cat <<EOF
Usage: $0 [COMMAND]

Commands:
    create      Create cluster and deploy zen-lock
    delete      Delete cluster
    kubeconfig  Show kubeconfig export command
    help        Show this help message

Environment Variables:
    CLUSTER_NAME       Name of the cluster (default: zen-lock-integration)
    CLUSTER_TYPE       Type of cluster: kind or k3d (default: kind)
    KUBECONFIG_PATH    Path to kubeconfig file (default: ~/.kube/\${CLUSTER_NAME}-config)
    ZEN_LOCK_PRIVATE_KEY  Private key for zen-lock (will generate if not set)

Examples:
    # Create kind cluster and deploy zen-lock
    $0 create

    # Create k3d cluster
    CLUSTER_TYPE=k3d CLUSTER_NAME=astesterole $0 create

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
            install_cert_manager
            install_certificate
            install_rbac
            image_name=$(build_and_load_image)
            deploy_zen_lock "$image_name"
            wait_for_ready
            log_info "✅ zen-lock integration test environment is ready!"
            log_info "Export kubeconfig: export KUBECONFIG=$KUBECONFIG_PATH"
            ;;
        delete)
            delete_cluster
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
