---
title: Isolated Compozy homes with COMPOZY_HOME
type: feature
---

Compozy now honors a `COMPOZY_HOME` environment variable as an opt-in override for the home root that everything home-scoped resolves against. When set to a non-empty value, Compozy uses that path instead of the implicit `$HOME/.compozy`.

### Why

The daemon is a singleton per home: every workspace that talks to `~/.compozy/daemon/daemon.sock` serializes through one engine. Operators running several independent projects in parallel previously had no first-class way to isolate them (the workaround was a fragile "mirror home" of symlinks).

`COMPOZY_HOME` is the official escape hatch. Point one shell at one home and another shell at a different home, and each gets its **own daemon, socket, lock, state, and global database**:

```bash
COMPOZY_HOME=~/.compozy-projectA compozy tasks run feature-a
COMPOZY_HOME=~/.compozy-projectB compozy tasks run feature-b
```

### What it covers

The override is honored consistently across every home-scoped consumer, not just the daemon socket:

- Home path resolution and layout (`ResolveHomeDir` / `ResolveHomePaths` / `EnsureHomeLayout`) and daemon startup.
- Global config loading and global workspace-marker detection.
- Extension discovery and enablement.
- Global reusable-agent discovery.

`~` and `~/` prefixes inside `COMPOZY_HOME` are expanded against the current user's home, so `COMPOZY_HOME=~/alt-compozy` works. When the variable is unset or empty, behavior is unchanged (`$HOME/.compozy`).

### Scope

This delivers the isolation escape hatch; a dedicated CLI `--home` flag and true parallel runs across workspaces inside a single daemon are intentionally out of scope and can layer on top later.
