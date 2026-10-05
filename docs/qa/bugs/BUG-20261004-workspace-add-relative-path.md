# BUG-20261004-workspace-add-relative-path: Relative workspace registration misses path resolution

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Lea
- **Journey Step:** J-scope-global-across-workspaces, register a folder from the CLI
- **Scenarios:** RT-home-workspace-not-registrable
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

A relative folder passed to workspace add reaches the transport without being resolved against
the invoking directory. The CLI returns an untyped absolute-path error instead of registering
a project or returning the canonical home-folder refusal.

## Reproduction

- **Charter:** CH-profile-global-phase-zero · **Tour:** Feature Tour
- **Environment:** real CLI and isolated daemon, unchanged runtime PID 77401.

1. From the operator home's parent directory, run workspace add with the home's relative name.
2. Observe exit 65 with `udsapi: root_dir must be an absolute path` and no typed code.
3. Repeat the equivalent absolute home path: it returns workspace_home_forbidden and the
   instruction to choose a project folder. Both attempts leave the catalog unchanged.

**Expected:** The CLI resolves its path argument relative to the invoking directory before
transport; the daemon still owns symlink canonicalization, home refusal and registration.
**Actual:** The relative spelling fails at transport validation before that shared boundary.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/: home-guard-cli-relative.json and its catalog read,
home-guard-cli-canonical.json, home-guard-http-canonical.json, home-guard-uds-canonical.json,
and home-guard-lea-ended.json. Trailing slash, symlink, shell tilde and expanded HOME all receive
the canonical typed refusal. A real absolute project registers on the CLI and reads identically
through HTTP/UDS; its owned empty registration is removed and the full baseline is restored.

The collector's KeyError after the relative response is retained as a driver assertion error;
it does not change the recorded production response. The session ends before source inspection.
Registry search found no existing owner for this symptom.

## Fix

- **Root cause:** newWorkspaceAddCommand only trims its positional path, while the public create
  contract requires an absolute root. Other CLI workspace entry points resolve their path boundary.
- **Correction:** Resolve relative registration roots against the injected CLI working directory;
  leave absolute roots and daemon-owned validation unchanged. Reuse currentWorkingDirectory.
- **Fix commit:** pending.
- **Regression test:** internal/cli/workspace_test.go, TestWorkspaceAddBuildsRequest.
  Invariant: relative registration roots reach the daemon as the same absolute candidate and a
  working-directory failure prevents registration. The CLI command owns this conversion; the
  real daemon replay owns canonical home refusal and persisted project registration.

This is a bounded CLI repair with no schema, wire shape, dependency or permission change.
It adds no command-local home policy, fallback workspace or alternate registration path.

## Verification

The existing request suite reproduces all three new cases before repair. The complete focused
TestWorkspace selection passes with the race detector after repair, and the changed test file
passes the convention checker. A separate real CLI build has SHA256
55a23562a9a40174da02a188f08c1f3bfcd0d28146ad906f56042859be4868f8.

Fresh Lea replay refuses relative home name, dot from home and a relative symlink with the same
workspace_home_forbidden code; every independent catalog read remains unchanged. Relative project
registration succeeds and HTTP/UDS return the same canonical record. Repeating the absolute
spelling correctly refuses a duplicate without changing the row. Remove the owned empty
registration and confirm complete catalog equality on both transports, unchanged PID 77401,
zero active sessions and current config.

The collector's literal Dev/dev path comparison is recorded separately: filesystem samefile
confirms the same directory and all public representations agree. No path/case failure is
established. Evidence: workspace-relative-root-{red,green,build,build-identity}.json and
home-guard-replay-lea-ended.json with its linked replay receipts.
Required delivery gate and fix commit remain pending.
