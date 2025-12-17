# Release Guide

This guide explains how to create a new release of receipt-engine with automatically built binaries.

## Creating a Release

### Method 1: Using Git Tags (Recommended)

1. **Update version** (if needed):
   ```bash
   # Make any final changes
   git add .
   git commit -m "Prepare for release v1.0.0"
   ```

2. **Create and push a version tag**:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. **GitHub Actions will automatically**:
   - Build binaries for all platforms (Linux, macOS, Windows)
   - Create a GitHub release
   - Attach all binaries to the release

### Method 2: Manual Trigger

1. Go to **Actions** tab in GitHub
2. Select **Build and Release** workflow
3. Click **Run workflow**
4. Enter version tag (e.g., `v1.0.0`)
5. Click **Run workflow**

## Version Format

Use semantic versioning: `vMAJOR.MINOR.PATCH`

Examples:
- `v1.0.0` - First stable release
- `v1.0.1` - Patch release (bug fixes)
- `v1.1.0` - Minor release (new features)
- `v2.0.0` - Major release (breaking changes)

## Release Artifacts

Each release includes binaries for:
- **Linux**: `receipt-engine-linux-amd64`, `receipt-engine-linux-arm64`
- **macOS**: `receipt-engine-darwin-amd64`, `receipt-engine-darwin-arm64`
- **Windows**: `receipt-engine-windows-amd64.exe`

## Verifying a Release

1. Check the **Releases** page on GitHub
2. Verify all platform binaries are attached
3. Test downloading and running a binary:
   ```bash
   wget https://github.com/thereceipt/receipt-engine/releases/download/v1.0.0/receipt-engine-linux-amd64.tar.gz
   tar -xzf receipt-engine-linux-amd64.tar.gz
   ./receipt-engine-linux-amd64
   ```

## Troubleshooting

### Build fails

- Check GitHub Actions logs
- Verify all dependencies are installed in the workflow
- Ensure Go version is compatible

### Missing binaries

- Check that all matrix builds completed successfully
- Verify artifact upload/download steps
- Check release creation step logs

### Version not showing

- Ensure `-ldflags "-X main.Version=${VERSION}"` is set in build step
- Check that the tag format is correct (`v*.*.*`)

