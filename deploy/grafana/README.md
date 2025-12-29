# zen-lock Grafana Dashboard

This directory contains the Grafana dashboard for monitoring zen-lock.

## Installation

### Option 1: Import via Grafana UI

1. Open Grafana and navigate to **Dashboards** → **Import**
2. Copy the contents of `dashboard.json`
3. Paste into the import dialog
4. Select your Prometheus data source
5. Click **Import**

### Option 2: Apply via ConfigMap (Kubernetes)

```bash
kubectl create configmap zen-lock-dashboard \
  --from-file=dashboard.json \
  -n monitoring \
  --dry-run=client -o yaml | \
  kubectl apply -f -
```

Then configure Grafana to load dashboards from ConfigMaps (if using Grafana Operator or similar).

## Dashboard Panels

The dashboard includes the following panels:

1. **Reconciliations by Result** - Success vs error counts
2. **Webhook Injections by Result** - Success, error, and denied counts
3. **Decryption Operations by Result** - Success vs error counts
4. **Reconciliation Rate** - Reconciliations per second
5. **Webhook Injection Rate Over Time** - Time series of injection rates
6. **Webhook Injection Duration** - P95 and P50 latency
7. **Reconciliation Duration** - P95 and P50 latency
8. **Decryption Duration** - P95 and P50 latency
9. **Error Rate Over Time** - All error types over time
10. **Top ZenLocks by Injection Count** - Most frequently injected secrets
11. **Top ZenLocks by Decryption Count** - Most frequently decrypted secrets

## Prerequisites

- Prometheus scraping metrics from zen-lock webhook controller
- Grafana configured with Prometheus as a data source
- Metrics endpoint accessible at `http://zen-lock-webhook:8080/metrics`

## Metrics Endpoint

The zen-lock controller exposes metrics on port `8080` at the `/metrics` endpoint.

