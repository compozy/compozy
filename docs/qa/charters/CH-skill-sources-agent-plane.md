# CH-skill-sources-agent-plane: Do the operator's whole job with no screen, then prove it from the ledger

```yaml
charter:
  id: CH-skill-sources-agent-plane
  mission: "As Ada, read the sources, change the policy, expose a skill, and verify the outcome using only structured surfaces — then prove from the durable ledger that each lifecycle path left exactly its own event, that a discarded generation was never recorded as applied, and that the shipped documentation describes the tool surface that actually exists."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-skill-sources-headless
  scenarios: [ET-skill-source-agent-parity, ET-skill-source-observe-ledger, ET-compozy-native-tool-invocation, ET-compozy-official-skill-discovery]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Start with the documented-versus-shipped question, because a wrong answer poisons everything an agent builds on it. The official skill's configuration reference states that compozy__config_set and compozy__config_unset deny skills.sources and skills.custom_sources with config_trust_root_forbidden. Call both tools on both keys at user, exact profile, and workspace scope and record what actually happens. Then read the shipped reference and the site config page against that result and file the mismatch with both sides cited — the fix (deny in code, or correct the documentation) is a human's decision, not this session's."
      - "Read the same persisted state through compozy skill sources -o json, GET /api/settings/skills over HTTP, the same route over the socket, compozy__skill_list and compozy__skill_view, and the extension Host API skills/list, and compare field names and values directly rather than by eye. Confirm origin and owner_scope are present, owner_scope stays inside its enum, skill_view carries exposures[] with the four statuses, and a Compozy-native skill reports an explicit empty origin."
      - "Exercise the workspace-parameter asymmetry on purpose: GET /api/skills takes workspace, GET /api/skills/{name} takes canonical workspace_id only and must refuse the other form by naming the canonical one, and expose/unexpose carry workspace_id in the body with the resolved id echoed back. A generated client that gets this wrong fails silently, so read the refusal, do not assume it."
      - "Write both keys at all three scopes, confirm the response states live semantics and returns the refreshed read model, and confirm the workspace tri-state — absent means untouched, null clears, an array sets. Match each refusal as a code, not a sentence: unknown_skill_source with its valid list and suggestion, duplicate_skill_source, invalid_source_path, workspace_scope_field_forbidden. Confirm a failed expose returns exactly one expose_failed envelope whether one target failed or several."
      - "Finish on the ledger. Drive each lifecycle path once, then read compozy logs --type <event> --component skill -o json and GET /api/logs and confirm one event per path with its correlation keys, that overlapping writes produced a superseded record and no second applied record, that the durable append preceded the broadcast, and that per-suppression decisions are absent by design — a skills.injection.suppressed record in the ledger is a contract violation, not extra coverage."
    must_avoid:
      - "Opening the web UI for any step. The point of this session is that the agent-manageability claim holds without it; reaching for a page to confirm something is itself a finding about the structured surface."
      - "Accepting near-parity. Byte-equivalent field names across transports is the contract; a renamed or reordered field is a finding even when both responses are individually correct."
      - "Trying to fix the documentation discrepancy inside the session. Record it with both citations and escalate."
```

## Selection rationale

The Feature Tour is the right lens here precisely because the headline promise *is* the advertised
path: CLAUDE.md's core premise is that every capability must be manageable by agents through
CLI/HTTP/UDS with structured output, so this session's job is to walk what the product claims and
check the claim, not to break it. Safety Invariant 7 and ADR-017 underwrite the read model — a root's
identity is derived from resource scope, stable profile id, stable workspace id, root kind, and
canonical directory, never from a display name or list position — and ADR-001, ADR-003, and ADR-004
define the two-key surface, the always-on `compozy` preset, and custom sources as the vocabulary the
structured surfaces have to expose consistently.

The session leads with the trust-root question because this cycle's planning pass found a concrete
contradiction: `skills/compozy/references/configuration.md` tells agents both keys are denied to
`compozy__config_set` with `config_trust_root_forbidden`, while the code puts both paths in the
agent-mutable allowlist (`internal/config/tool_surface.go:155-156`) whose lookup returns before the
trust-root check (`:278-281` versus `:294-296`). Only one of those can be true. It is planned as a
walk rather than filed as a bug because the bug registry requires reproduction and evidence from a
session, and a code read is neither.

`ET-compozy-native-tool-invocation` and `ET-compozy-official-skill-discovery` ride along for the same
reason they were reset and left untested: the skill tool descriptors, their required fields, and the
recorded catalog digests changed in this cycle, and task_07 rewrote two of the bundled skill's
references. Both are read through the same structured surfaces this session is already driving.

Ada owns it as the non-human actor QA role-plays — zero patience for ambiguous or non-parseable
output, which is exactly the standard the byte-equivalence and matchable-code assertions need.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
