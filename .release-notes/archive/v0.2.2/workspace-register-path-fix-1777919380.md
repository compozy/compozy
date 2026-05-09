---
title: Workspace register/resolve path fixes
type: fix
---

Two long-standing workspace-discovery papercuts are fixed. `compozy workspaces register` and `resolve` now accept relative paths the same way every other Compozy command does, and workspace auto-discovery no longer treats the home-scoped `~/.compozy/` runtime directory as a project-local workspace marker.

Closes #139.

### Relative paths now work for `register` / `resolve`

Before, the API client sent paths through unchanged after `strings.TrimSpace`. A relative path like `.` or `./my-project` was forwarded to the daemon as-is, where it resolved against the daemon's working directory instead of the caller's, producing confusing "workspace not found" errors or registering the wrong directory.

The client now normalizes the argument before sending it:

```go
// internal/api/client/operator.go
func normalizeWorkspacePathArg(path string) (string, error) {
    trimmed := strings.TrimSpace(path)
    if trimmed == "" {
        return "", nil
    }
    if filepath.IsAbs(trimmed) {
        return filepath.Clean(trimmed), nil
    }
    absolutePath, err := filepath.Abs(trimmed)
    if err != nil {
        return "", fmt.Errorf("resolve workspace path %q: %w", path, err)
    }
    return filepath.Clean(absolutePath), nil
}
```

This normalization runs for both `RegisterWorkspace` and `ResolveWorkspace`, so:

```bash
cd ~/code/my-feature
compozy workspaces register .            # now registers /Users/you/code/my-feature
compozy workspaces resolve ./sub-project  # resolves against the caller's CWD
```

### `~/.compozy/` is no longer auto-detected as a workspace

`discoverWorkspaceRootFromStart` walks up the filesystem looking for a `.compozy/` marker directory. When `compozy` was invoked from anywhere under `$HOME` that did not contain its own `.compozy/`, the walk would eventually find `~/.compozy/` — the home-scoped daemon runtime root — and register the user's home directory (or some ancestor) as a workspace.

The discovery loop now resolves the global Compozy marker once and skips it during the walk, so only project-local `.compozy/` directories are treated as workspace roots:

```go
// internal/core/workspace/config.go
globalMarkerDir, hasGlobalMarker := discoverGlobalWorkspaceMarkerDir()
// ...
if err == nil && info.IsDir() {
    // The home-scoped Compozy directory stores global runtime/config state.
    // It must not redefine arbitrary paths under HOME as local workspaces.
    if !hasGlobalMarker || !sameWorkspaceMarkerDir(candidate, globalMarkerDir) {
        return current, nil
    }
}
```

Comparison is symlink-aware (`filepath.EvalSymlinks` on both sides), so installs that symlink `~/.compozy/` are still correctly excluded.

### Coverage

New tests pin the behavior end-to-end:

- `internal/api/client/client_transport_test.go` — relative paths are normalized before transport.
- `internal/cli/operator_commands_integration_test.go` — `register` / `resolve` from a relative CWD produce absolute paths in the registry.
- `internal/core/workspace/config_test.go` — discovery skips `~/.compozy/` even when started from `$HOME`.
- `internal/store/globaldb/registry_test.go` — registry insert/lookup is consistent with the normalized paths.
