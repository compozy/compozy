# BUG-20260910-terminal-agent-identity: Native terminal commands lose the managed agent identity

- **Status:** fixed
- **Impact (user-side):** Task-Block
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-cross-workspace-access, supported agent CLI spawn and coordination read
- **Scenarios:** ET-workspace-access-mode-matrix
- **Found:** 2026-09-10 · **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction and evidence

Managed Cursor session `sess-27d6aca7a434c19b` ran the documented foreign-workspace `compozy spawn` command through native `terminal_exec`. It failed with exit 64 and `identity_required`, saying `COMPOZY_SESSION_ID` was missing. A subsequent coordination CLI read returned exit 0 without that identity, so this result cannot prove agent permission evaluation. Both commands ran once; the session was stopped. Evidence: `cross-cli-spawn-prompt.json`, `cross-cli-spawn-events.json`, and `cross-cli-spawn-stop.json` under the run evidence directory.

Expected: commands launched for the managed agent retain its session/name, so CLI identity validation and workspace permission decisions use that agent. Actual: the terminal process inherits daemon environment plus the optional caller map, without the originating actor's identity.

## Diagnosis and intended correction

The native terminal adapter validates session/run/generation and creates a terminal Actor containing the agent name and session ID. The terminal service's process creation does not project that Actor into environment variables. Provider processes already receive these variables from the session manager, but daemon-native terminals have a separate creation path.

Project the originating agent identity at the terminal service's two process-start boundaries, preserving caller environment and overriding conflicting identity keys. Cover exec, interactive open, and adapter pipe in the existing manager admission/scope suite at the process I/O boundary. This repairs accidental context loss; environment variables are not a security sandbox against arbitrary hostile local code.

## Cross-surface impact

- Native tools/CLI: agent-created terminal processes receive existing `COMPOZY_SESSION_ID`, `COMPOZY_AGENT`, and `COMPOZY_AGENT_NAME`; no new tool IDs, schemas, verbs, or DTOs.
- HTTP/UDS and Web: use the same terminal service and validated Actor; agent-origin commands retain identity. Human-origin behavior remains unchanged.
- Extensibility/hooks/config: pipe consumers keep their environment; agent identity derives from the validated origin. No hook/config/SDK shape changes.
- Workspace data isolation: downstream CLI uses the existing daemon identity resolver and workspace access policy. No persisted shape or migration.
- Official skill/docs: terminal reference explains managed CLI identity propagation; this scenario and bug retain actual verification limits.

Canonical exec/open/pipe regression failed before the repair (`terminal-identity-red.txt`) and passed afterward; the full terminal race suite, ACP terminal adjacency, test-shape check, Go build and Windows cross-compilation passed. No Windows runtime verification. Fresh Cursor session `sess-9ec00faa329ab131` repeated the exact two CLI commands, both exit 0. Independent HTTP confirmed child `sess-d28dfa5a405c8d72` in workspace B with the requested Cursor runtime and parent lineage. Public logs attributed spawn/coordination decisions to the parent agent in approve-all mode. Parent and child were verified stopped. Evidence: `terminal-identity-retest-proof.json` and its named source records. `make gate` passed all affected lanes (`terminal-identity-gate.txt`). The complete cross-workspace mode matrix is not yet verified.

## PR integration correction

PR #624 runtime E2E exposed an ACP-specific identity mismatch: terminal ownership carries the provider session ID, while downstream Compozy CLI identity resolution requires the registered Compozy session. The adapter now prioritizes its daemon-supplied terminal scope when constructing the terminal Actor. ACP ownership checks retain the provider ID. The existing ACP lifecycle conformance walk exercises distinct IDs and a conflicting caller environment value; the launched process must receive the Compozy ID. The original native-terminal Cursor proof remains scoped to that path.
