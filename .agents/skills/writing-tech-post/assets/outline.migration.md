# [MIGRATION TITLE — decision-narrative or system-name framing]

<!--
archetype: architecture-migration
depth-tuple: (R2 legacy charity, R4 dominant + R3 phase metrics + R5 named tooling, R2 dated status snapshot, Anchor-and-dive)
length-band: 5,000–8,000
byline-norm: multi-author (2–9) + named acknowledgements paragraph
-->

## [Why we relied on the predecessor — empathetic legacy framing]

<!-- R2 — Charity toward predecessor system. Migration posts are read by the engineers who built the prior system; contemptuous framing burns credibility. -->

Like many other organizations, [COMPANY] has long relied on [PREDECESSOR]. This pattern is pervasive because [WHY IT WORKED WELL]. But eventually, the trade-offs started to pile up.

## Why now

<!-- R2 — Concrete trigger. Pre-empt the reader's natural question. -->

[The specific trigger — scaling ceiling / security pressure / cost trajectory / organisational restructuring.]

## The before state

<!-- R3 + R4 — System shape with measurable properties. -->

[Architecture diagram before. Quantified scope (e.g., 700+ jobs across 8 regions).]

## Constraints we kept fixed

<!-- R2 — What we refused to change. -->

[The constraints that bounded the migration — backwards compatibility, SLAs, customer interfaces.]

## The shape we picked (numbered phases)

<!-- R4 — The plan, named phases. -->

### Phase 1: [Logical separation]

[What changed in this phase; tracking metric for completion; named safety mechanism.]

### Phase 2: [Access reduction]

[...]

### Phase 3: [Physical separation]

[...]

## How we cut over (named safety mechanisms)

<!-- R4 + R5 — Named tooling: PG Proxy / shadow tables / dual-stack A/B / per-region cutover with retained-rollback window. -->

[Cutover discipline — per-phase, per-region, or per-cohort. Name the rollback window and how it was validated.]

## Results & costs

<!-- R3 — Per-phase tracking metrics; quantified scope. -->

[Concrete numbers: phase-completion dates, tracking metric trajectory, customer-facing impact (or absence thereof).]

## What we'd do differently

<!-- R2 + R3 — Honest disclosure of difficulties. Retains the messiness. -->

[What surprised us; what we under-estimated; what we'd skip or do earlier next time. *"thousands of duplicate symbol errors"* / *"we'd underestimated the impact"* style.]

## Where we are now (dated status snapshot)

<!-- R2 — Explicit and falsifiable. -->

Phase 1 started in [Q1 2024] and finished in [middle of Q1 2025]. Phase 2 started [halfway into Q1 2024] and will continue through [2026].

## Forward work

<!-- R2 — Specific applications, not generic "AI will help." -->

[Two or more specific next-direction commitments.]
