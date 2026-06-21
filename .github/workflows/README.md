# GitHub Actions Workflow for Building and Releasing mdict-go-web

This directory contains GitHub Actions workflows to build and release the `mdict-go-web` binary for multiple platforms.

## Workflow: Build and Release

The `build-release.yml` workflow builds the `mdict-go-web` binary for the following platforms:
- Linux AMD64
- Windows AMD64
- macOS AMD64 (Intel)
- macOS ARM64 (Apple Silicon)

### How to Trigger

1. Go to the "Actions" tab in your GitHub repository
2. Select the "Build and Release" workflow
3. Click "Run workflow"
4. Provide the following inputs:
   - **Tag for the release**: A version tag (e.g., `v1.0.0`)
   - **Release name**: Optional release name (defaults to "Release")

### What the Workflow Does

1. **Build Phase**: 
   - Uses an Ubuntu runner with Go 1.26
   - Cross-compiles the binary for each target platform using Go's cross-compilation
   - Adds `.exe` extension for Windows binaries
   - Uploads each binary as an artifact

2. **Release Phase**:
   - Downloads all artifacts from the build phase
   - Creates a GitHub release with the specified tag
   - Attaches all binaries to the release

### Supported Platforms

| Platform | File Name |
|----------|-----------|
| Linux AMD64 | mdict-go-web-linux-amd64 |
| Windows AMD64 | mdict-go-web-windows-amd64.exe |
| macOS AMD64 (Intel) | mdict-go-web-darwin-amd64 |
| macOS ARM64 (Apple Silicon) | mdict-go-web-darwin-arm64 |

### Requirements

- The repository must have the `contents: write` permission for the workflow to create releases
- Tags should follow semantic versioning (e.g., `v1.0.0`, `v2.1.3-beta`)