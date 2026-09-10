# BUG-20260910-idle-session-high-cpu: Open sessions continuously consume several CPU cores

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** Keep managed sessions connected while working in CompozyOS
- **Scenarios:** ET-compozy-native-tool-invocation
- **Found:** 2026-09-10
- **Report:** docs/qa/reports/2026-09-10-beta24-daemon-cpu.md

## Reproduction

1. Open managed sessions with hosted MCP tools in beta.24 and a workspace with
   skills and extensions.
2. Leave their tool projection streams connected.
3. Observe the daemon's CPU in macOS Activity Monitor and capture a process sample.

**Expected:** Connected sessions retain current tools and permissions without
continuously rescanning all skills for each extension tool.

**Actual:** The installed daemon sustained approximately 256–384% CPU. Samples
showed repeated workspace scans originating from hosted tool projection refresh.
The user reported two visible sessions; the runtime API reported three active
runtimes. These are distinct counts.

## Cause and fix

Hosted projections intentionally refresh live native availability and policy.
Extension workspace canonicalization incorrectly requested a complete runtime
snapshot for each tool, multiplying filesystem scans at the 100 ms stream poll
interval. A shared identity-only workspace registration operation removes this
unnecessary work while retaining root and identity checks. Native descriptors are
reused after constructor validation, unchanged skill discovery is reused after
filesystem checks, hosted digests are reused after complete projection comparison,
and native policy uses a separate config/agent resolver without skill inventory.
Live availability, permissions, scope precedence and digest compatibility remain
covered by regression tests and real runtime canaries. Review hardening also
validates directory membership and captured filesystem change metadata, protects
agent-config cache publication against invalidation races, and propagates
metadata-preserving skill edits through the resource-catalog watcher.

## Verification

See the owning report for the raw-evidence locations, controlled before/after
measurements, regression suites, and final gate status. The original local gate
and strict QA audit passed. At the user's request, delivery gates for the final
review commit run exclusively in CI. The matching Go 1.26.4 runtime comparison
reduced daemon CPU from 186.867% to 21.157% (88.68%), with two active sessions,
two total sessions, and the same 237-tool catalog. Functional canaries, including
metadata-preserving edits, passed. Both labs report clean teardown with zero
surviving processes; the installed user application has not been replaced.
