# Helm Repository Setup

This repository hosts a Helm chart repository via GitHub Pages, served from the `main` branch.

## Repository URL

The Helm repository is available at:
```
https://kube-zen.github.io/zen-lock
```

## Setup Instructions

### Initial Setup (One-time)

1. **Enable GitHub Pages** in repository settings:
   - Go to Settings → Pages
   - Source: Deploy from a branch
   - Branch: `main` / `/docs`
   - Save

2. **The GitHub Actions workflow** will automatically:
   - Package the Helm chart when changes are pushed to `main` or on releases
   - Generate/update `index.yaml` in `docs/`
   - Commit and push to the `main` branch
   - GitHub Pages will serve the repository from the `docs/` folder

### Manual Setup (if needed)

If you need to set up the repository manually:

```bash
# Package the chart
make helm-package

# Create docs directory (if it doesn't exist)
mkdir -p docs

# Copy packaged chart
cp .helm-packages/*.tgz docs/

# Generate index
cd docs
helm repo index . --url https://kube-zen.github.io/zen-lock
cd ..

# Commit and push
git add docs/*.tgz docs/index.yaml
git commit -m "chore: Initial Helm repository setup"
git push origin main
```

## Using the Repository

Users can add and use the repository:

```bash
# Add repository
helm repo add zen-lock https://kube-zen.github.io/zen-lock
helm repo update

# Install chart
helm install zen-lock zen-lock/zen-lock \
  --namespace zen-lock-system \
  --create-namespace

# Upgrade chart
helm upgrade zen-lock zen-lock/zen-lock \
  --namespace zen-lock-system

# List available versions
helm search repo zen-lock/zen-lock --versions
```

## Artifact Hub Integration

The chart is also available on Artifact Hub:

1. **Add repository to Artifact Hub**:
   - Visit: https://artifacthub.io/add-repo
   - Repository URL: `https://kube-zen.github.io/zen-lock`
   - Repository type: `Helm charts`
   - Submit

2. **Artifact Hub will automatically**:
   - Index charts from the GitHub Pages repository
   - Display chart metadata from `Chart.yaml`
   - Show installation instructions
   - Track chart versions

3. **Users can install from Artifact Hub**:
   ```bash
   # Add via Artifact Hub (if configured)
   helm repo add zen-lock https://kube-zen.github.io/zen-lock
   helm repo update
   helm install zen-lock zen-lock/zen-lock
   ```

## Local Development

To test packaging locally:

```bash
# Lint chart
make helm-lint

# Test chart rendering
make helm-test

# Package chart
make helm-package

# Generate repository index
make helm-repo-index

# Or run all Helm tasks
make helm-all
```

## Workflow

The GitHub Actions workflow (`.github/workflows/publish-helm-chart.yml`) automatically:

1. **Triggers on**:
   - Push to `main` branch (when `charts/` changes)
   - Release publication
   - Manual workflow dispatch

2. **Process**:
   - Lints the Helm chart
   - Packages the chart (`.tgz` file)
   - Copies packaged chart to `docs/` directory
   - Generates/updates `index.yaml` in `docs/`
   - Commits and pushes changes to `main`

3. **GitHub Pages**:
   - Serves files from `docs/` directory
   - Makes charts available at `https://kube-zen.github.io/zen-lock`

## Chart Versioning

Chart versions follow semantic versioning:
- **Major**: Breaking changes
- **Minor**: New features (backward compatible)
- **Patch**: Bug fixes

Chart version is defined in `charts/zen-lock/Chart.yaml`:
```yaml
version: 0.0.1-alpha
appVersion: "0.0.1-alpha"
```

When releasing a new version:
1. Update `Chart.yaml` version
2. Update `Chart.yaml` appVersion (if application version changed)
3. Update `values.yaml` default image tag (if needed)
4. Commit and push to `main`
5. GitHub Actions will automatically package and publish

## Troubleshooting

### Chart Not Available

1. **Check GitHub Pages**:
   - Verify Pages is enabled in repository settings
   - Check that `docs/` directory exists with `index.yaml`
   - Verify `docs/` contains `.tgz` chart files

2. **Check Workflow**:
   - View Actions tab for workflow runs
   - Verify workflow completed successfully
   - Check for errors in workflow logs

3. **Manual Verification**:
   ```bash
   # Test repository URL
   curl https://kube-zen.github.io/zen-lock/index.yaml
   
   # Should return index.yaml content
   ```

### Artifact Hub Not Showing Chart

1. **Verify Repository URL**:
   - Ensure URL is correct: `https://kube-zen.github.io/zen-lock`
   - Repository must be publicly accessible

2. **Check Chart Metadata**:
   - Verify `Chart.yaml` has required fields
   - Check that chart is properly packaged

3. **Wait for Indexing**:
   - Artifact Hub may take a few minutes to index
   - Check Artifact Hub repository status

## See Also

- [Chart README](charts/zen-lock/README.md)
- [Helm Documentation](https://helm.sh/docs/)
- [Artifact Hub Documentation](https://artifacthub.io/docs/)

