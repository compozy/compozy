---
title: Homebrew distribution switched from cask to formula
type: fix
---

Compozy's Homebrew distribution moves from a cask to a proper formula. This simplifies installation (no separate `brew tap` step), enables `brew test`-driven smoke checks, and aligns the upgrade flow with how CLI tools are normally distributed on Homebrew.

### Install command

```bash
# Before (cask)
brew tap compozy/compozy
brew install --cask compozy

# After (formula)
brew install compozy/compozy/compozy
```

The shorthand auto-taps `compozy/compozy` and installs the `compozy` formula in a single command.

### Upgrade command

`compozy upgrade` (and the `compozy upgrade` flow inside `internal/update/install.go`) now targets the formula instead of the cask:

```bash
brew upgrade compozy/compozy/compozy
```

Existing users on the cask should reinstall via the formula path; both can't coexist on the same prefix.

### Release pipeline

- `.goreleaser.yml` replaces the `homebrew_casks:` block with a `brews:` block:
  - `directory: Formula`
  - `commit_msg_template: "Brew formula update for {{ .ProjectName }} version {{ .Tag }}"`
  - `license: "BSL-1.1"`
  - `test: system "#{bin}/compozy", "--version"` — every published formula now smoke-tests `compozy --version`.
- The release artifact for Homebrew is keyed via `ids: [compozy-archive]` so the formula picks the right archive.
- The archive comment now reads: `Keep the binary at the archive root so Homebrew formulas can install it directly.`

### README

`README.md` is updated with the new one-liner install command.
