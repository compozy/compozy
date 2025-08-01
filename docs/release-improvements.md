# Release Process Improvements

This document outlines the improvements made to the Compozy release process, inspired by Charmbracelet's Crush project.

## Key Improvements

### 1. Shell Completions Generation
- Added automatic generation of bash, zsh, and fish completions during the build process
- Completions are included in release archives and installed automatically via package managers
- Graceful fallback if the completion commands are not yet implemented

### 2. Man Page Generation
- Added automatic man page generation in the before hooks
- Man pages are compressed with gzip and included in releases
- Installed automatically by Homebrew, Nix, and AUR packages

### 3. Software Bill of Materials (SBOM)
- Configured SBOM generation for security compliance
- Generates CycloneDX JSON format SBOMs for each binary
- Helps with vulnerability tracking and dependency auditing

### 4. Enhanced Changelog
- Improved changelog categorization with emoji indicators
- Added new categories: Security Updates, Refactoring, Testing
- Better filtering of noise commits (WIP, merge commits)
- Reordered categories by importance (security near the top)

### 5. Multi-Platform Package Support

#### Homebrew (macOS/Linux)
- Already configured with tap repository support
- Enhanced with shell completions and man page installation
- Uses conditional file existence checks for robustness

#### Nix (NixOS)
- Added support for Nix User Repository (NUR)
- Includes shell completions and man pages
- Configurable dependencies
- Requires creating `compozy/nur` repository

#### Arch Linux (AUR)
- Added both source (`compozy`) and binary (`compozy-bin`) packages
- Proper conflict management between package variants
- Includes all shell completions and man pages
- Requires AUR account and SSH key setup

### 6. Workflow Improvements
- Added concurrency control to prevent multiple releases running simultaneously
- Uses `cancel-in-progress: false` to ensure releases complete

## Required Setup

### 1. Repository Creation
Create these repositories under the `compozy` organization:
- `homebrew-compozy` (already planned)
- `nur` (for Nix packages)

### 2. Secrets Configuration
Add these secrets to the main repository:
- `HOMEBREW_TAP_GITHUB_TOKEN` - GitHub PAT with repo access
- `AUR_KEY` - SSH private key for AUR access

### 3. AUR Setup
1. Create an AUR account
2. Request maintainer access for `compozy` and `compozy-bin` packages
3. Generate SSH key and add public key to AUR account
4. Add private key as `AUR_KEY` secret

### 4. Implement Missing Commands
The following commands need to be implemented in the CLI:
```go
// Shell completions
compozy completion bash
compozy completion zsh  
compozy completion fish

// Man page generation
compozy man
```

## Configuration Files Modified

1. `.goreleaser.yml`:
   - Added shell completions and man page generation in before hooks
   - Enhanced archive configuration to include completions and man pages
   - Added SBOM generation
   - Improved changelog with emojis and better categorization
   - Added Nix package configuration
   - Added AUR package configurations (source and binary)
   - Updated Homebrew configuration for completions and man pages

2. `.github/workflows/release.yml`:
   - Added concurrency control
   - Added `AUR_KEY` to environment variables

## Benefits

1. **Better User Experience**: 
   - Shell completions improve CLI usability
   - Man pages provide offline documentation
   - Multiple installation methods for different platforms

2. **Security & Compliance**:
   - SBOM generation for supply chain security
   - Signed releases with cosign
   - Better changelog categorization highlights security updates

3. **Wider Distribution**:
   - Support for major Linux distributions (Arch, NixOS)
   - Professional package management across platforms
   - Consistent installation experience

4. **Developer Experience**:
   - Automated release process reduces manual work
   - Comprehensive changelogs improve communication
   - Concurrent release protection prevents conflicts

## Next Steps

1. Implement the missing CLI commands for completions and man page generation
2. Create the required repositories (NUR)
3. Set up AUR account and packages
4. Configure all required secrets
5. Test the release process with a pre-release version

## Testing

Before the next release:
1. Run `goreleaser release --clean --snapshot` locally to test
2. Verify completions and man pages are generated
3. Check that all package configurations are valid
4. Test installation from each package manager after release