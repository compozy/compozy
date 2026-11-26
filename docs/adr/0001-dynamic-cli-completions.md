# ADR 0001 — Live shell completion values for the CLI

- Status: Accepted
- Date: 2026-11-26
- Deciders: operator, oracle review (sub-894e784ad098a0ae)
- Scope: `internal/cli` completion surface

## Context

`compozy completion <shell>` already emits Cobra's static command and flag tree. The
identifier arguments and flags that operators type most (`session <id>`, `--workspace`,
`--profile`, `--agent`, `--provider`, `--model`, `folder`, `task`, `loop`, `automation`,
`worktree`, `extension`, `computer`, vault refs and toolsets) had no completion, so every
call required a manual lookup through a `list` command.

Cobra supports live values through `ValidArgsFunction` and `RegisterFlagCompletionFunc`,
invoked in a short-lived process by the shell's `__complete` path. That process may run on
every TAB, so the design constraints are latency and side-effect freedom, not throughput.

## Decision

Add live value completion backed by the existing read-only daemon list APIs.

- One explicit `map[commandPath]completionSpec` plus an explicit opt-out set. No inference
  from `Use` placeholders: several commands reuse `<id>`/`<name>` for new values, operation
  IDs or run IDs.
- File and directory arguments keep Cobra's default file completion. Only identifier
  arguments return `ShellCompDirectiveNoFileComp`.
- One 400 ms context budget per completion call, covering client construction, workspace
  scoping and the list request. No cache.
- Failures and timeouts return an empty candidate list with exit 0 and no stderr output.
  Completion never starts the daemon and never mutates state.
- Workspace scoping uses only the explicit `--workspace` value, `COMPOZY_WORKSPACE`, or a
  single `ListWorkspaces` + cwd match. It does not reuse the interactive workspace/profile
  resolution chain, which mutates command context and prints warnings.
- Candidates render as `value\tdescription`. Identifiers use the canonical value the
  command accepts: session IDs, workspace names (ID when the name is ambiguous or
  path-like), agent names, provider IDs, model IDs, profile names, task/automation/worktree
  IDs, session and workspace folder IDs, loop names, extension/computer/toolset names, and
  vault refs (never secret values).

## Alternatives considered

- **On-disk cache of identifiers.** Rejected: stale IDs in completion are worse than an
  empty list, and invalidation has no cheap correctness story.
- **File completion fallback when the daemon is unavailable.** Rejected: suggesting
  unrelated paths for an ID argument hides the failure and invites wrong input.
- **Static identifier lists generated at build time.** Rejected: sessions, tasks and
  workspaces change per machine and per minute.
- **Reusing the interactive workspace resolution chain.** Rejected: it costs several
  sequential daemon calls, mutates the command context and can print warnings that break
  PowerShell completion parsing.

## Consequences

- The completion table and opt-out set must be updated when a command gains an identifier
  argument; a census test fails when a placeholder is neither mapped nor opted out.
- Completion latency depends on the local UDS daemon; a stopped or slow daemon degrades to
  an empty list within 400 ms.
- No new daemon route or public contract is introduced; the change is additive on the
  existing `compozy completion <shell>` surface.
- `provider models set` needs a provider before models can be suggested; without one the
  model position stays empty rather than guessing across providers.
