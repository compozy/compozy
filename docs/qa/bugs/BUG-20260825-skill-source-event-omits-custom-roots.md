# BUG-20260825-skill-source-event-omits-custom-roots: Applied-source events omit custom roots

- **Status:** verified <!-- open | fixed | verified | wont-fix | invalid -->
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-operate-skill-sources-headless, audit a live source-policy change
- **Scenarios:** ET-skill-source-observe-ledger; ET-live-skill-source-reload
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

After Dora applies a workspace policy with eight custom source roots, the durable
`skills.sources.applied` event reports only `compozy`, `agents`, and `claude`. All eight custom
sources are absent from `root_counts`, even though the settings and source diagnostics surfaces show
them active in the same generation. The event therefore cannot prove which roots that generation
actually applied.

## Reproduction

- **Charter:** CH-skill-sources-agent-plane · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US · isolated lab daemon `127.0.0.1:55384`, build `a79537e6f`

1. Set workspace `skills.custom_sources` to eight roots, including `huge`, `locked`, `links`, two
   collision roots, an ecosystem root, an installer-layout root, and an absent root.
2. Read `compozy skill sources --workspace <id> -o json`; all eight custom sources are present.
3. Read `compozy logs --workspace <id> --type skills.sources.applied --component skill -o json`.

**Expected:** `root_counts` contains every effective source and its physical root count.
**Actual:** `root_counts` is `{"agents":1,"claude":1,"compozy":1}`; every custom source is omitted.

## Evidence

- `<lab>/qa-artifacts/qa/charter-diagnostics-sources.json` — the effective source projection.
- `<lab>/qa-artifacts/qa/charter-diagnostics-ledger.json` — the contradictory durable event.

## Fix

- **Root cause:** source-event correlation carried owner and actor identity only. At commit,
  `writeSkillSourcesApplied` rebuilt `root_counts` from `RegistryConfig.GlobalSkillRoots`, which is
  intentionally only the daemon's user-level registry configuration during profile/workspace
  applies. The staged profile/workspace projection — including custom roots — was therefore
  invisible to the event writer.
- **Fix commit:** `e7dffdb74`
- **Regression tests:** `TestSkillsSectionDiagnostics/Should correlate source applies with every
  effective root` proves settings measures the exact post-write projection; and
  `TestRegistryConfigGenerationFence/Should keep the winning profile generation on every catalog
  surface` proves the event serializes those correlated counts.

## Verification

- **Retested:** 2026-08-25, same persona/journey · **Report:** docs/qa/reports/2026-08-25-skill-sources.md
- **Result:** generation 18 recorded all twelve effective sources: two physical roots each for
  `compozy`, `agents`, and `claude`, plus one root for every custom source (`huge`, `locked`,
  `links`, `collide-a`, `collide-b`, `ecosystem`, `installer`, `inst2`, and `missing`). Evidence:
  `<lab>/qa-artifacts/qa/charter-ledger-root-counts-{apply,event}.json`.
