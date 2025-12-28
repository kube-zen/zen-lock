# Release Process

This document describes the release process for zen-lock.

## Versioning

zen-lock follows [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

Current version: **0.1.0-alpha**

## Release Checklist

### Pre-Release

- [ ] All tests pass (`make test`)
- [ ] Code coverage meets threshold (`make coverage`)
- [ ] Security scan passes (`make security-check`)
- [ ] Documentation is up to date
- [ ] CHANGELOG.md is updated
- [ ] Version numbers updated in code
- [ ] Release notes prepared

### Release Steps

1. **Create Release Branch**
   ```bash
   git checkout -b release/v0.1.0-alpha
   ```

2. **Update Version**
   - Update version in `cmd/cli/main.go`
   - Update version in `cmd/webhook/main.go`
   - Update version in `CHANGELOG.md`

3. **Build and Test**
   ```bash
   make build
   make test
   make verify
   ```

4. **Create Tag**
   ```bash
   git tag -a v0.1.0-alpha -m "Release v0.1.0-alpha"
   git push origin v0.1.0-alpha
   ```

5. **Create GitHub Release**
   - Go to GitHub Releases
   - Create new release from tag
   - Copy release notes from CHANGELOG.md
   - Attach binaries if needed

6. **Build and Push Docker Image**
   ```bash
   make build-image
   docker push kube-zen/zen-lock-webhook:v0.1.0-alpha
   docker push kube-zen/zen-lock-webhook:latest
   ```

7. **Merge to Main**
   ```bash
   git checkout main
   git merge release/v0.1.0-alpha
   git push origin main
   ```

## Release Notes Template

```markdown
## [Version] - YYYY-MM-DD

### Added
- Feature 1
- Feature 2

### Changed
- Change 1
- Change 2

### Fixed
- Fix 1
- Fix 2

### Security
- Security fix 1
```

## Post-Release

- [ ] Announce release (if applicable)
- [ ] Update documentation
- [ ] Monitor for issues
- [ ] Plan next release

