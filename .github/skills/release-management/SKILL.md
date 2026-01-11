---
name: release-management
description: Prepare, package, test, and publish releases of the Mattermost OIDC plugin including version management, release notes, and artifact distribution. Use this skill when preparing new releases or managing the release process.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: development
  tags: [release, versioning, packaging, publishing, ci-cd]
---

# Release Management Skill

## Summary
This skill covers the complete release process for the Mattermost OIDC plugin including version management, building release artifacts, testing, and publishing to GitHub Releases.

## When to Use This Skill
- Preparing new plugin releases
- Creating release candidates
- Publishing to GitHub Releases
- Managing semantic versioning
- Generating release notes
- Validating release artifacts

## Release Process Overview

1. **Version Planning** - Determine version number
2. **Code Freeze** - Stop feature development
3. **Version Bump** - Update version in code
4. **Build & Test** - Create and validate artifacts
5. **Release Notes** - Document changes
6. **Tag & Publish** - Create Git tag and GitHub Release
7. **Validation** - Test published artifacts
8. **Announcement** - Notify users

## Semantic Versioning

Format: `MAJOR.MINOR.PATCH` (e.g., `0.0.2`)

**MAJOR** - Breaking changes
- API changes
- Configuration schema changes
- Minimum Mattermost version changes

**MINOR** - New features (backward compatible)
- New OIDC features
- New configuration options
- Performance improvements

**PATCH** - Bug fixes (backward compatible)
- Security patches
- Bug fixes
- Documentation updates

## Version Bump Process

### Update plugin.json

```bash
# Edit version field
vim plugin.json

# Example change:
# "version": "0.0.2" → "version": "0.0.3"
```

```json
{
  "id": "com.mm.oidc",
  "name": "Mattermost OIDC Bridge",
  "version": "0.0.3",
  ...
}
```

### Update Documentation

```bash
# Update README.md with new version
sed -i 's/v0.0.2/v0.0.3/g' README.md

# Update references in docs
grep -r "0.0.2" docs/ | cut -d: -f1 | sort -u | xargs sed -i 's/0.0.2/0.0.3/g'
```

### Commit Version Bump

```bash
git add plugin.json README.md docs/
git commit -m "Bump version to 0.0.3"
```

## Build Release Artifact

### Clean Build

```bash
# Remove old artifacts
make clean-plugin

# Full clean build
rm -rf server/dist webapp/dist build/

# Build plugin
make package

# Verify artifact created
ls -lh build/plugins/mm-oidc.tar.gz
```

### Verify Archive Contents

```bash
# List contents
tar -tzf build/plugins/mm-oidc.tar.gz

# Should contain:
# com.mm.oidc/
# com.mm.oidc/plugin.json
# com.mm.oidc/server/dist/plugin-linux-amd64
# com.mm.oidc/webapp/dist/main.js

# Extract and verify
mkdir -p /tmp/verify
tar -xzf build/plugins/mm-oidc.tar.gz -C /tmp/verify
cat /tmp/verify/com.mm.oidc/plugin.json | jq '.version'
```

## Pre-Release Testing

### Test Suite Execution

```bash
# Run all tests
make server-test
make webapp-test
make e2e-test

# Check for test failures
echo "Exit code: $?"
```

### Manual Testing Checklist

- [ ] Install in clean Mattermost instance
- [ ] Configure with test Keycloak
- [ ] Complete login flow successfully
- [ ] Verify user provisioning
- [ ] Test admin role promotion
- [ ] Check plugin health endpoint
- [ ] Validate proxy integration (if applicable)
- [ ] Test logout flow
- [ ] Verify error handling
- [ ] Check logs for errors

### Security Scan

```bash
# Scan Go dependencies
cd server
govulncheck ./...

# Scan Node dependencies
cd webapp
yarn audit

# Run CodeQL if available
# Check GitHub Security tab for alerts
```

## Release Notes Generation

### Collect Changes

```bash
# List commits since last release
git log v0.0.2..HEAD --oneline

# Group by type
git log v0.0.2..HEAD --oneline --grep="feat:"
git log v0.0.2..HEAD --oneline --grep="fix:"
git log v0.0.2..HEAD --oneline --grep="docs:"
```

### Release Notes Template

```markdown
# Release v0.0.3

## 🎉 New Features
- Feature 1 description
- Feature 2 description

## 🐛 Bug Fixes
- Fix 1 description
- Fix 2 description

## 📚 Documentation
- Documentation improvements

## 🔒 Security
- Security improvements or patches

## ⚠️ Breaking Changes
- Breaking change description (if any)

## 📦 Installation
Download `mm-oidc.tar.gz` and install via System Console.

## 🔗 Full Changelog
https://github.com/insoln/mm-oidc/compare/v0.0.2...v0.0.3
```

## Create Git Tag

```bash
# Create annotated tag
git tag -a v0.0.3 -m "Release version 0.0.3"

# Push tag to GitHub
git push origin v0.0.3

# Verify tag
git tag -l
```

## Publish GitHub Release

### Using GitHub Web UI

1. Navigate to https://github.com/insoln/mm-oidc/releases/new
2. Select tag: `v0.0.3`
3. Release title: `v0.0.3`
4. Paste release notes in description
5. Attach artifact: Upload `build/plugins/mm-oidc.tar.gz`
6. Check "Set as latest release" if applicable
7. Click "Publish release"

### Using GitHub CLI

```bash
# Install gh cli if needed
# https://cli.github.com/

# Create release
gh release create v0.0.3 \
  --title "v0.0.3" \
  --notes-file RELEASE_NOTES.md \
  build/plugins/mm-oidc.tar.gz

# Verify release
gh release view v0.0.3
```

### Using API

```bash
# Create release via GitHub API using gh CLI (auth handled securely, no token in process args)
gh api \
  --method POST \
  -H "Accept: application/vnd.github+json" \
  /repos/insoln/mm-oidc/releases \
  -f tag_name="v0.0.3" \
  -f name="v0.0.3" \
  -f body="$(cat RELEASE_NOTES.md)" \
  -F draft=false \
  -F prerelease=false

# Upload artifact
gh release upload v0.0.3 build/plugins/mm-oidc.tar.gz

# Alternative: If you must use curl, read token from file to avoid process exposure
# echo "$GITHUB_TOKEN" > /tmp/gh-token.txt && chmod 600 /tmp/gh-token.txt
# curl -X POST \
#   -H "Authorization: token $(cat /tmp/gh-token.txt)" \
#   -H "Accept: application/vnd.github.v3+json" \
#   https://api.github.com/repos/insoln/mm-oidc/releases \
#   -d @release-payload.json
# rm -f /tmp/gh-token.txt
```

## Post-Release Validation

### Verify GitHub Release

```bash
# Check release exists
gh release list

# Download published artifact
gh release download v0.0.3 --pattern "mm-oidc.tar.gz"

# Verify checksum matches local build
sha256sum mm-oidc.tar.gz build/plugins/mm-oidc.tar.gz
```

### Test Installation from Release

```bash
# Download from GitHub
curl -LO https://github.com/insoln/mm-oidc/releases/download/v0.0.3/mm-oidc.tar.gz

# Install in test instance
mmctl plugin add mm-oidc.tar.gz

# Verify installation
mmctl plugin list | grep com.mm.oidc

# Test functionality
curl https://test-instance.example.com/plugins/com.mm.oidc/health
```

## Release Checklist

### Pre-Release
- [ ] All tests passing (unit, integration, E2E)
- [ ] No security vulnerabilities
- [ ] Documentation updated
- [ ] Version bumped in plugin.json
- [ ] CHANGELOG.md updated
- [ ] Release notes prepared

### Release
- [ ] Clean build created
- [ ] Artifact verified
- [ ] Git tag created and pushed
- [ ] GitHub Release published
- [ ] Artifact uploaded

### Post-Release
- [ ] Release validated
- [ ] Installation tested
- [ ] Release announced
- [ ] Documentation site updated (if applicable)

## CI/CD Automation

### GitHub Actions Workflow

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Node.js
        uses: actions/setup-node@v3
        with:
          node-version: '20'
          
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'
          
      - name: Build plugin
        run: make package
        
      - name: Run tests
        run: |
          make server-test
          make webapp-test
          
      - name: Create Release
        uses: actions/create-release@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          tag_name: ${{ github.ref }}
          release_name: Release ${{ github.ref }}
          draft: false
          prerelease: false
          
      - name: Upload Release Asset
        uses: actions/upload-release-asset@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          upload_url: ${{ steps.create_release.outputs.upload_url }}
          asset_path: ./build/plugins/mm-oidc.tar.gz
          asset_name: mm-oidc.tar.gz
          asset_content_type: application/gzip
```

## Hotfix Process

### For Critical Bugs

```bash
# 1. Create hotfix branch from release tag
git checkout -b hotfix/0.0.3-fix1 v0.0.3

# 2. Fix the issue
vim server/plugin.go
git commit -m "fix: critical security issue"

# 3. Bump patch version
vim plugin.json  # 0.0.3 → 0.0.3.1 or 0.0.4
git commit -m "Bump version to 0.0.4"

# 4. Build and test
make package
make server-test

# 5. Tag and release
git tag -a v0.0.4 -m "Hotfix release"
git push origin hotfix/0.0.3-fix1
git push origin v0.0.4

# 6. Create GitHub Release
gh release create v0.0.4 --notes "Security hotfix" build/plugins/mm-oidc.tar.gz

# 7. Merge back to main
git checkout main
git merge hotfix/0.0.3-fix1
git push origin main
```

## Related Skills
- plugin-build
- security-audit
- e2e-testing

## References
- [Makefile](../../../Makefile)
- [plugin.json](../../../plugin.json)
- [GitHub Releases](https://github.com/insoln/mm-oidc/releases)
- [Semantic Versioning](https://semver.org/)
