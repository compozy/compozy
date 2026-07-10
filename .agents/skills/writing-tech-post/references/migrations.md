# Architecture Migrations

The migration archetype's contract: legacy-charity opening, seven-panel canonical structure, phase-completion dates + tracking metrics, named cutover safety mechanisms, "what we'd do differently" honesty, and the seven sub-variants the corpus surfaces.

## Contents

- [Opening move: legacy charity + why-now](#opening-move-legacy-charity--why-now)
- [Seven-panel canonical structure](#seven-panel-canonical-structure)
- [Evidence obligations](#evidence-obligations)
- [Cutover discipline and named safety mechanisms](#cutover-discipline-and-named-safety-mechanisms)
- ["What we'd do differently" honesty pattern](#what-wed-do-differently-honesty-pattern)
- [Seven sub-variants](#seven-sub-variants)
- [Closing move: dated status snapshot](#closing-move-dated-status-snapshot)
- [Silent failure modes (migration slop)](#silent-failure-modes-migration-slop)

## Opening move: legacy charity + why-now

Migration writing characterises the prior state with **charity, not contempt**. Migration posts are read by the engineers who built the prior system; contemptuous framing burns credibility internally and externally.

Exemplars:

- Datadog: *"Like many other organizations, Datadog has long relied on the convenience of a large, shared relational database. This pattern is pervasive in the industry because it works well for many workloads — and continues to work well even at surprisingly large scales. But eventually, the trade-offs start to pile up."*
- Meta WebRTC: *"Permanently forking a big open-source project can result in a common industry trap. It starts with good intentions."*
- Slack SSH→REST: *"It worked. But it came with some potential problems."*

The "why now" pre-empts the reader's natural question. State the trigger explicitly (security pressure, scaling ceiling, cost trajectory, organisational restructuring).

## Seven-panel canonical structure

1. **Empathetic legacy framing** — see above.
2. **Why-now justification** — concrete trigger.
3. **Phased plan (numbered)** — three to seven phases, each named.
4. **Tracking metrics per phase** — the numbers the team watched.
5. **Cutover discipline** — named safety mechanisms (see below).
6. **Where we are now (dated)** — explicit "Phase 1 finished Q1 2025; Phase 2 will continue through 2026."
7. **Forward work** — what comes next, with named applications.

Meta WebRTC walks the variant: *Challenge of monorepo+linker → Shim Layer and Dual-Stack Architecture → patch automation → upgrade cycle → results → Future Work: AI-Driven Maintenance.*

## Evidence obligations

- **Phase-completion dates.** Datadog: *"Phase 1 started in Q1 2024 and finished in the middle of Q1 2025."* Explicit and falsifiable.
- **Per-phase tracking metrics.** Datadog: `postgresql.table.count` aggregated by schema converging toward 0.
- **Quantified scope.** Slack: *"700+ jobs in production… across 8 data regions."* Meta WebRTC: *"over 50 use cases"* and explicit difficulty disclosure (*"thousands of duplicate symbol errors"*).
- **Paired before/after with phase intermediaries** — not just two states. Migration archetypes have a stricter contract than other archetypes: show the *journey*, not just endpoints.
- **Vendor names at R5 (implementation)**, not R1/R2 (framing). See `depth-and-abstraction.md` for the rule.

## Cutover discipline and named safety mechanisms

Named tooling for rollback is the **credibility hinge**:

- Datadog: PG Proxy, shadow tables.
- Meta WebRTC: dual-stack A/B testing per cohort.
- Meta data ingestion: shadow tables with reconciliation.
- Slack: per-region cutover with retained-rollback window.

Generic claims ("we used industry-standard practices") fail the test. Name the tool, name the strategy, name the rollback window.

## "What we'd do differently" honesty pattern

The pattern converts a marketing-shaped draft into a credibility-shaped one. The strong form retains the messiness:

- Meta WebRTC: *"thousands of duplicate symbol errors, hundreds of thousands of lines were modified across thousands of files."*
- Slack EMR: discloses orphaned processes, custom SSH operators, audit complexity — not "we migrated 700 jobs with zero issues."
- Canva (postmortem-derived): *"we'd underestimated the impact of the bug and didn't expedite deploying the fix."*

The "what we'd do differently" paragraph is **load-bearing**, not a courtesy hedge. Skipping it produces a triumphal-shaped post that the genre rejects.

## Seven sub-variants

Each is a legitimate variant, not a deviation:

1. **System rewrite / generational replacement** — Datadog Rust storage (six-generation lineage).
2. **Decomposition / monolith split** — Datadog unwinding shared database.
3. **Stack replacement / protocol modernization** — Slack SSH→REST.
4. **Pipeline replacement** — Meta data ingestion.
5. **Component modernization / hybrid retrofit** — Meta Groups Search, Meta WebRTC fork escape.
6. **Dual-stack variant** — Meta WebRTC. Adds a "Shim Layer" / "Adapter" section; per-cohort cutover.
7. **Maturity-level variant** — Meta PQC's PQ-Unaware → PQ-Enabled. Industry-wide framework instead of phased plan. **Must populate the framework with the publisher's own trajectory** — framework-without-instance is an explicit anti-pattern.

Eighth variant: **Security-driven** — Slack SSH→REST. Why-now leans on security pain (*"we had a massive security surface… not ideal"*).

## Closing move: dated status snapshot

*"Phase 1 started in Q1 2024 and finished in the middle of Q1 2025. Phase 2 started halfway into Q1 2024 and will continue through 2026."* — explicit and falsifiable. Migration posts that omit the dated status snapshot read as if completion is being claimed before earned.

**Forward work** names specific next applications (not "AI will help" or "we plan to optimise further"). Meta WebRTC names build-health automation and conflict-resolution as the two specific applications of the Future Work section.

## Silent failure modes (migration slop)

- **Aspirational language** — *"we are beginning to roll out…"* without a current state.
- **Triumphal claims** — *"700 jobs migrated with zero issues."*
- **Percentage-complete claims without a base** — *"we're 75% migrated"* with no denominator.
- **Toolset-as-story** — the narrative is "here are the tools we used" rather than "here is what changed and why."
- **Over-weighted forward-work** — "AI-Driven Maintenance" close on an otherwise unrelated migration; the bolt-on AI-future anti-pattern.
- **Premature-completion claim** — *"the migration is done"* without explicit "fully deprecated" language.
- **Framework-without-instance** — naming a maturity model or phase taxonomy without showing the publisher's own trajectory through it.
- **Contemptuous predecessor framing** — the most common credibility leak. Charity is mandatory.
