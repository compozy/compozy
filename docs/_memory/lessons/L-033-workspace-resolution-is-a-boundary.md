# L-033 — Workspace resolution is a boundary, not command-family plumbing

**Class:** Architecture / CLI / Workspace isolation
**Date discovered:** 2026-07-28 (workspace-resolution fragmentation audit and isolation regression matrix)
**Evidence sources:** `docs/prompts/20260728-0037_infer-workspace-from-cwd.md`;
`docs/_refacs/20260728-workspace-resolution-fragmentation.md`; native-tool RED run with 12/12
intended isolation assertions failing; CLI inventory of roughly 57 hard-required commands across at
least eight independently implemented families.

## Context

Compozy already had an intended workspace chain, but only a handful of commands used it. Loop,
automation, memory, window-manager, MCP, approvals, task, and config commands independently marked
`--workspace` required, parsed different ID/path subsets, or emitted family-specific errors. The CLI
client and daemon also treated a path as an exact registered root, so a command run in
`<workspace>/internal/...` failed or, in `session new`, silently registered the subdirectory as a new
workspace.

The same convention failure was more severe in native tools. Workspace scope was guarded inside
selected handlers rather than at the shared dispatch boundary. Omitted workspace fields failed
schema validation even when the trusted session already supplied scope, while handlers that forgot
the opt-in guard could read or mutate a foreign workspace.

## Root cause

Workspace identity was represented as repeated flag parsing and handler discipline instead of one
owned boundary. Each new command family copied the visible mechanism (`--workspace` plus a required
check) without inheriting precedence, ref grammar, path discovery, provenance, or isolation. A
cross-cutting security decision therefore depended on every future author remembering every prior
implementation.

This is shotgun surgery plus copy-paste programming: one semantic change required edits across
dozens of commands because the knowledge had many representations. The duplication was not harmless
boilerplate; each copy became an independent authorization and data-routing policy.

## Rule

> A workspace selector is a candidate for the canonical resolver, never a command-local policy.
> CLI commands supply positional/flag candidates to the shared chain; native tools enter through the
> dispatch binder. No family may require a workspace flag, invent a workspace-ref grammar, compare
> raw refs for ownership, or auto-register on a read path.

The CLI precedence is positional ref, `--workspace`, `COMPOZY_WORKSPACE`, validated session identity,
then nearest-enclosing cwd discovery. The public ref grammar is ID, name, or path. `--workspace` is an
override, not a prerequisite. Structured output reports the winning source.

## Operationalization

- `internal/cli/workspace_resolution.go` owns precedence, canonical resolution, fallback behavior,
  deterministic errors, and structured provenance.
- `internal/workspace/discovery.go` owns nearest-enclosing root discovery, nested-root precedence,
  symlink canonicalization, and path-boundary checks. `AdditionalDirs` do not establish ownership.
- `internal/tools/dispatch_input_binding.go` and
  `internal/daemon/native_workspace_input_binder.go` bind trusted session scope before validation,
  rebind after pre-call hooks, and reject foreign non-operator input before handlers execute.
- `TestWorkspaceResolutionBoundary` in `internal/cli/workspace_test.go` walks the complete Cobra tree
  and fails when `--workspace` becomes required, its help stops describing an override, or
  `--workspace-id`/`--scope-id` and required-workspace residue reappear.
- Read-only, preview, extension, MCP, bundle, automation-sync, and consolidation paths resolve
  existing workspaces only. Registration remains explicit except for `session new` on a genuinely
  unregistered project directory.

## Detection signals

- A new command calls `mustMarkFlagRequired` for workspace or emits its own “workspace required”
  error.
- A CLI flag is named `--workspace-id` or `--scope-id`, or accepts fewer than ID/name/path.
- A path below a registered root returns not found or creates another workspace registration.
- A native handler reads `workspace` directly without the dispatch binder, or a bound session must
  echo its own workspace back into tool input.
- Ownership compares raw user refs instead of canonical resolved workspace IDs.

## Source

- Handoff and verified inventory:
  `docs/prompts/20260728-0037_infer-workspace-from-cwd.md`.
- Architectural coupling analysis:
  `docs/_refacs/20260728-workspace-resolution-fragmentation.md`.
- Canonical CLI boundary: `internal/cli/workspace_resolution.go`.
- Daemon discovery boundary: `internal/workspace/discovery.go`.
- Native isolation boundary: `internal/tools/dispatch_input_binding.go` and
  `internal/daemon/native_workspace_input_binder.go`.
- CI guardrail: `internal/cli/workspace_test.go` (`TestWorkspaceResolutionBoundary`).
