# QA Run Report — 2026-08-05 — Issue 314 native managed skill loading

- **Scope:** Issue #314 — a managed Codex session loads an installed skill omitted from its prompt catalog through the native seam; the CLI remains an operator surface
- **Cadence tier:** targeted
- **Build:** working tree · **Environment:** fresh isolated runtime and provider homes from the QA bootstrap manifest
- **Started:** 2026-08-06T00:18:26Z · **Completed:** 2026-08-06T00:31:51Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-managed-session-skill-loading |

## Flows in Scope

- `J-load-skill-in-managed-session` — load an omitted skill through the native seam and verify the operator CLI independently (`../journeys/J-load-skill-in-managed-session.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-managed-session-skill-loading | J-load-skill-in-managed-session / ET-managed-session-skill-loading | Ada | Feature Tour | Pass | [cold hosted-tool bind](../bugs/BUG-20260805-hosted-mcp-cold-start-nonce-expiry.md) | PR #323 remediation commit |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-managed-session-skill-loading

- The fresh workspace contained 220 `aa-context-fill-*` skills and `zz-native-seam-marker`. The
  persisted 75,618-byte provider prompt contained 418 catalog entries, including the fillers, but no
  target catalog entry or body. The target name occurred once only in the user's request.
- The first and only provider session, `sess-08797076cb99e02f`, started through a wrapper that delayed
  Codex ACP by 35 seconds. Initialize took 36,958 ms; hosted-MCP activation then completed in 0 ms,
  `session/new` in 443 ms, and bind in 79 ms without nonce expiry or retry.
- The agent resolved `compozy__skill_view` with `compozy__tool_info` at event 11, called
  `compozy__skill_view` at event 15, and received the exact marker-bearing body at event 16. Events
  18–21 returned the exact marker and heliotrope sentence to the operator.
- The normalized native result and independent operator `compozy skill view` result were both 173
  bytes with SHA-256 `9eb83bd03cfa8e4763ff8c1808dda9f5305fae0f066a88e606fa07bb08e4ebca`.
- The persisted tool calls contain no shell, terminal, operator CLI, or direct skill-file read.
- All twelve `compozy skill` verbs failed with the supported-path error when managed environment
  markers were present. The target remained enabled and active, no new skill was created, and the
  independent operator command still succeeded.
- Teardown completed with `clean: true`, no survivors, and the runtime socket removed. It escalated the
  registered daemon after the graceful-stop window and removed the remaining lab-referenced provider
  process.

## What Was Fixed

The unsafe managed CLI capability transport and caller-supplied identity contract were removed. A
managed session loads skills only through the daemon-native tool; `compozy skill view` remains an
operator-shell command. The CLI now has an early supported-path guard when managed environment markers
are present. It prevents accidental unsupported use; it is not a security boundary against arbitrary
same-UID code.

## Paper Cuts

None.

## Runtime Errors Observed

None. The intentionally delayed first launch bound hosted MCP without nonce expiry.

## Human Verifications Needed

None.

## Decisions for a Human

The earlier request to prove that an arbitrary same-UID provider cannot open the operator UDS was
withdrawn by the approved hard cut. Without an OS sandbox, a dedicated UID, or a non-inherited
credential, the provider and operator can emit identical socket requests. This change therefore makes
no such security claim: managed skill loading uses only the native seam, while the normal UDS and
`compozy skill view` remain operator surfaces.

## Learnings

Behavioral acceptance must inspect persisted managed events, not merely invoke the native registry in
isolation. Delaying provider startup beyond the former TTL proved that nonce lifetime must begin when
the runtime makes binding possible, not while an external provider initializes.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` runs after this final report mutation; the PR
  records the resulting current gate evidence.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** truncated catalog, deliberately delayed first Codex launch, native tool discovery/view,
  exact-body parity, persisted event inspection, all skill CLI verbs under the supported-path guard,
  operator CLI, and clean teardown
- **Verdict:** Behavioral Pass; workstream completion remains conditional on the final current
  `make gate-full` record.
