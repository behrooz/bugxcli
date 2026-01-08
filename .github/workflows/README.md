# GitHub Actions Workflows

This directory contains GitHub Actions workflows for the BugX CLI project.

## Workflows

### CI (`ci.yml`)
Runs on every push and pull request to main/master/develop branches:
- Runs tests
- Builds the project
- Checks code formatting
- Runs `go vet`

### Release (`release.yml`)
Creates releases when:
- A tag starting with `v` is pushed (e.g., `v1.0.0`)
- Manually triggered via workflow_dispatch

**Builds for:**
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

**Creates:**
- GitHub Release with all binaries
- SHA256 checksums file

## How to Create a Release

### Automatic (via tag):
```bash
git tag v1.0.0
git push origin v1.0.0
```

### Manual (via GitHub UI):
1. Go to Actions tab
2. Select "Build and Release" workflow
3. Click "Run workflow"
4. Enter version (e.g., `v1.0.0`)
5. Click "Run workflow"

## Release Artifacts

Each release includes:
- Binaries for all platforms and architectures
- `checksums.txt` with SHA256 checksums
- Release notes with installation instructions

