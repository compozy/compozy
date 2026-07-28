---
name: writing-tech-post
description: >-
  Authors engineering blog posts end-to-end: launch deep-dives, incident
  postmortems, architecture migrations, performance case studies, tutorials,
  AI/agent system writeups, security disclosures, and research-to-product
  translations. Picks the correct archetype, plans the abstraction ladder,
  enforces an evidence cadence (diagrams, benchmarks, profiles, traces, code,
  ablations), tunes voice against publisher house styles (Datadog, Vercel,
  GitHub, AWS, Meta, Cloudflare, Jane Street), and runs a pre-publish gate for
  narrative momentum and disclosure ethics. Use when drafting a new engineering
  post, restructuring a draft that feels flat, deciding which evidence form
  belongs where, validating that depth and product context are balanced, or
  preparing a postmortem, migration, or performance narrative for external
  publication. Do not use for API reference documentation, README authoring,
  marketing copy, release notes, generic SEO content, ghost-written executive
  thought leadership, or non-engineering long-form essays.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Writing Engineering Posts

SOTA authoring loop for technical blog posts. The philosophy is **archetype-first** (pick the genre), **depth-second** (commit the abstraction ladder), **evidence-third** (every claim attaches to an artifact), **voice & disclosure fourth** (publisher register + ethics), **momentum last** (narrative spine, lede, closer). Inline content in this SKILL.md is a dispatcher; the contract lives in `references/`.

## Required Reading Router

Match the post's archetype (or the active phase) to the row. Read the listed files **in full before** producing the corresponding output. Inline content in this SKILL.md is a pointer, not a substitute.

| Task / Archetype                                                | MUST read                                                                                              |
| --------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| Selecting an archetype (Phase 1, every post)                    | `references/archetypes-and-structure.md`                                                               |
| Planning depth / abstraction ladder (Phase 2, every post)       | `references/depth-and-abstraction.md`                                                                  |
| Writing an incident postmortem                                  | `references/postmortems.md` + `references/voice-and-disclosure.md`                                     |
| Writing an architecture migration narrative                     | `references/migrations.md` + `references/depth-and-abstraction.md`                                     |
| Writing a performance deep-dive                                 | `references/performance-deep-dive.md` + `references/evidence-diagrams-code.md`                         |
| Writing an AI/agent system post                                 | `references/ai-and-agents.md` + `references/evidence-diagrams-code.md`                                 |
| Writing a security or reliability post                          | `references/security-and-reliability.md` + `references/voice-and-disclosure.md`                        |
| Writing a tutorial or research-to-product translation           | `references/archetypes-and-structure.md` + `references/depth-and-abstraction.md`                       |
| Placing diagrams, charts, code, tables, ablations (Phase 3)     | `references/evidence-diagrams-code.md`                                                                 |
| Tuning to a publisher house voice (Phase 4)                     | `references/voice-and-disclosure.md` + `references/publisher-voice-matrix.md`                          |
| Tightening lede, headlines, transitions, closer (Phase 5)       | `references/narrative-and-pacing.md`                                                                   |
| Pre-publish gate (Phase 6)                                      | `references/pre-publish-checklist.md` + `references/anti-patterns.md`                                  |

## Reference Index

- `references/archetypes-and-structure.md` — Eight canonical archetypes (launch deep-dive, postmortem, migration, performance, tutorial, research-to-product translation, AI/agent, security/reliability) with their 9-column contract (opening / sequence / closing / length / byline / evidence / voice / hybrid partner / silent failure mode) plus the 9-question decision tree and hybrid-disclosure rules.
- `references/depth-and-abstraction.md` — Five-rung abstraction ladder (R1 user → R5 measurement) with three traversal patterns (staircase, yo-yo, spiral), the rung-whiplash diagnostic, the anchor-N rule, and per-archetype default depth profiles.
- `references/postmortems.md` — Canonical postmortem section sequence (Summary → Background → Incident → Timeline → Contributing factors → Mitigation → Action items), blameless register three rules, UTC-timeline + named-artifact obligations, failed-mitigation requirement.
- `references/migrations.md` — Five migration sub-types (rewrite, decomposition, stack replacement, pipeline replacement, component modernization, dual-stack, maturity-level), seven-panel canonical structure, empathetic strategic voice, "what we'd do differently" honesty, dated-status-snapshot closing.
- `references/performance-deep-dive.md` — Detective-arc structure (hypothesis → measurement → reveal → next bottleneck), distribution-shift evidence contract (percentile + sample size + window + environment), iterative bottleneck-peeling, partial-victory disclosure between fixes.
- `references/ai-and-agents.md` — Paper-link-first attribution, named-benchmark + ablation evidence contract, named-checker idiom (debugging-agent + leakage-checker + usage-checker), "AI handles the long tail" closing motif and its slop variant (bolted-on AI-future paragraphs).
- `references/security-and-reliability.md` — Threat-model opening, layered-defense walkthrough, coordinated-disclosure four-panel (threat-model → background → defense → mitigations), probabilistic register for adversary capabilities, CVE + upstream-commit citation contract.
- `references/evidence-diagrams-code.md` — Twelve-form evidence taxonomy (architecture diagram, sequence/flow, timeline, dashboard, distribution chart, flamegraph/profile, trace view, results table, code listing, config/manifest, ablation matrix, benchmark plot), captioning conventions (caption states the finding), `claim → artifact → reading` cadence rule, code-length thresholds (1–15 / 16–30 / >30).
- `references/voice-and-disclosure.md` — Blameless register, coordinated-disclosure four-panel, paper-link-first attribution, "what we'd do differently" honesty pattern, vendor-naming conventions, charity toward predecessor systems, disclaimer-paragraph genre signal.
- `references/narrative-and-pacing.md` — Five-lede taxonomy (result-first / mystery / stakes-first / 3-W summary / paper-link-first / shipping-status), H2-as-question-resolution discipline, story-shape catalogue (detective / migration / blameless / paper-link-first / tutorial arcs), closer multiple-choice gate, first-200 + last-200 callback-coupling test.
- `references/publisher-voice-matrix.md` — Cross-publisher matrix (Datadog, Vercel, GitHub, AWS, Meta, Cloudflare, Jane Street) by six surface features (byline weight, sentence length, vendor density, evidence reflex, opening register, closing register), plus banned moves per publisher.
- `references/anti-patterns.md` — Banned moves catalogue: buried lede, rung whiplash, evidence-free percent claims, blame-by-implication, code-without-context, depth-without-product-framing, paper-name-dropping, false-precision metrics, hedged ledes, exciting-announcement templates, coordinated-disclosure violations, AI-eval-as-anecdote, archetype-bait headlines, framework-without-instance, bolted-on AI-future close.
- `references/pre-publish-checklist.md` — Archetype-conditional checklist (postmortem-only / migration-only / performance-only / AI-only / security-only rows), disclosure-blocker list, lint-thresholds, publishable / hold-for-review / rework rubric.

## Bundled Path Rule

Resolve every bundled helper relative to the directory containing this `SKILL.md`. When the command below appears as `scripts/<name>`, treat the actual invocation as `<writing-tech-post-dir>/scripts/<name>` — expand `<writing-tech-post-dir>` to the absolute skill directory before running. Likewise expand `assets/<name>` and `references/<name>` to the bundled paths under the skill directory.

## Operating Loop (6 Phases)

Each phase ends in a STOP directive. The inline content in each phase is a gist tripwire — enough to detect violations during scanning — not the contract. The reference file holds the contract.

### Phase 1 — Archetype Selection & Audience Framing

1. Identify the artifact's purpose against the eight archetypes (launch deep-dive, incident postmortem, architecture migration, performance case study, developer tutorial, research-to-product translation, AI/agent system writeup, security/reliability post). When the post straddles two, name the primary as load-bearing and the secondary as the absorbed archetype; the primary's contract wins.
2. Name the audience by abstraction rung target (`product-user` / `engineer-adopter` / `peer-engineer-deep` / `infra-or-research-peer`). The lede must speak to that rung.
3. Capture archetype + audience in the draft's frontmatter before any prose. Treat this commitment as a contract — every later phase derives its rules from it.

Gist tripwires (the moves the archetype demands, in one line each):

- Postmortems open with **date + scope + impact in the first two sentences**, never internal cause.
- Migrations open with **empathetic legacy framing + why-now**, never contemptuous-of-predecessor.
- Performance posts open with **felt experience + stakes**, never a headline number.
- Launches open with **scale-then-headline-number** and land the number above the first H2.
- AI/agent posts open with **one-paragraph capability claim + paper/repo link in the first scroll**.
- Security posts open with **threat-model + adversary capability**, never the fix.

**STOP. Read `references/archetypes-and-structure.md` in full before drafting the outline.** That file holds the 9-column contract per archetype (opening / sequence / closing / length / byline / evidence / voice / hybrid partner / silent failure mode), the 9-question decision tree, the archetype-straddle rules, and the per-archetype silent failure modes. The tripwires above are pointers, not the contract.

### Phase 2 — Outline & Depth Planning

1. Sketch every section and tag it with its abstraction rung: **R1 (product / user experience) → R2 (system shape) → R3 (component design) → R4 (mechanism / algorithm) → R5 (measurement / proof)**.
2. Commit a four-tuple `(opening rung, body residency band, closing rung, traversal)`. Pick the traversal: *staircase* (R1→R5 monotonic), *yo-yo* (R3↔R5 oscillation around a probe), or *spiral* (R1→R3→R1→R4→R1→R5, restating product context between depth dives).
3. Load the archetype's outline skeleton from `assets/outline.<archetype>.md`. Mark sections that the archetype requires versus optional.
4. Apply the **anchor-N rule**: if more than four consecutive R4/R5 paragraphs appear without surfacing to R2/R3, insert an anchor paragraph or open a new section.

Gist tripwires:

- Every section gets a rung tag (R1–R5); jumping >2 rungs without an anchor is **rung whiplash**.
- Tutorials live at R1–R3; deep-dives live at R3–R5; both still need an **R1 anchor in the lede**.
- Vendor names appear at **R5 (implementation)**, never at R1/R2 (framing).

**STOP. Read `references/depth-and-abstraction.md` in full before fixing the outline.** That file holds the rung definitions with corpus examples, the three traversal patterns with worked walkthroughs, the rung-whiplash diagnostic, and the per-archetype default depth profile. **Also STOP and read the archetype's deep-dive reference file** (`references/postmortems.md`, `migrations.md`, `performance-deep-dive.md`, `ai-and-agents.md`, or `security-and-reliability.md`) when the archetype matches one of those five. The outline skeleton in `assets/` is a starting point, not a contract.

### Phase 3 — Draft with Evidence Cadence

1. For each outlined section, pre-declare the evidence form it will carry (one of: architecture diagram, sequence/flow diagram, timeline, dashboard screenshot, distribution/percentile plot, flamegraph/profile, trace/span view, table-of-results, code listing, config/manifest, ablation matrix, named-benchmark plot). Sections without evidence stay prose-only; flag them explicitly.
2. Enforce the **`claim → artifact → reading` cadence**: every artifact is preceded by a prose claim and followed by a prose reading. Captions state the *finding*, never the artifact name.
3. Apply the per-archetype mandatory evidence-form set (postmortems require UTC timeline + named-artifact root cause; migrations require phase-completion dates + per-phase tracking metrics + named cutover safety mechanism; performance requires distribution-shift charts not means; AI requires named benchmark + ablation + named guardrails; security requires CVE + upstream commit + coordinated-disclosure timeline).
4. Write code blocks at the right rung: tutorial code is runnable end-to-end with prerequisites stated; deep-dive code is the **minimal disclosing slice with elision markers** (`// ...` patterns and a one-line note of what was cut). Warn when a snippet crosses 30 lines — split with prose, elide, or replace with a source link plus a load-bearing slice.

Gist tripwires:

- Every figure caption states the **finding**, not the artifact name. Captions starting with "Figure showing…" or "Diagram of…" are rejected.
- Every percent claim points to a chart, table, or named benchmark. Mean-only performance charts are rejected.
- Code blocks declare their **elision marker** and what was cut.

**STOP. Read `references/evidence-diagrams-code.md` in full before placing any chart, diagram, code block, or table.** That file holds the twelve-form evidence taxonomy with selection guide, the captioning conventions, the prose↔evidence cadence rule, the elision-marker code standard, the distribution-shift evidence pattern (mandatory for any performance claim), the named-benchmark + ablation contract (mandatory for AI/agent capability claims), and the alt-text-as-prose discipline.

### Phase 4 — Voice & Disclosure Pass

1. Pick the publisher house voice the post targets — Datadog (systems-pragmatic), Vercel (product-tight), GitHub (team-narrative), AWS (deliberate-measured), Meta (cross-organisational), Cloudflare (technical-confident), or Jane Street (precise-academic). Rewrite the lede and closer against the chosen register.
2. Apply the disclosure layer matching the archetype:
   - **Postmortems** → blameless register (system-subject sentences; polarity-correct passive/active; first-person plural ownership). Engineers' names never appear in causality.
   - **Security posts** → coordinated-disclosure four-panel (threat-model opening → background → layered-defense walkthrough → mitigations close). Probabilistic register for adversary capabilities.
   - **AI/agent posts** → paper-link-first attribution. Named benchmarks. Capability claims declarative, limitations hedged conditional.
   - **Migrations** → empathetic strategic voice; "what we'd do differently" honesty paragraph.
3. Apply vendor-naming conventions: partners named verbatim with shared responsibility; own products named as instruments; competitors named specifically when load-bearing; redaction reasons disclosed when scope-limited.

Gist tripwires:

- Active third-person team voice. **"We" or "the system" — never "you" except in tutorials.**
- Postmortems use the **blameless register**; no person's name attached to causation.
- AI posts cite the **paper before claiming the result**.
- Migrations include **"what we'd do differently"**. Triumphal language ("successfully", "smoothly", "without issue") at any density is a regression.

**STOP. Read `references/voice-and-disclosure.md` in full before the voice pass.** Disclosure ethics, blameless register rules, paper-link-first attribution conventions, coordinated-disclosure constraints, and vendor-naming conventions live there. **STOP and additionally read `references/publisher-voice-matrix.md`** when targeting a specific publisher voice; that file holds the seven-publisher matrix (byline weight, sentence length, vendor density, evidence reflex, opening register, closing register) and the per-publisher banned moves.

### Phase 5 — Narrative & Momentum Pass

1. Audit the lede against the five-lede taxonomy. It must promise either a *result* (numbers/outcome), a *mystery* (the unexplained behaviour), a *stakes frame* (what was at risk), a *3-W summary* (postmortem), or a *paper-link-first* attribution (AI). Hedged "we want to share some thoughts on…" ledes are rejected.
2. Audit the H2 chain against the **H2-as-question-resolution** discipline. Each H2 must answer the question the previous section left open. List the open questions; if any H2 starts without resolving one, restructure.
3. Apply the archetype-specific story arc:
   - **Detective-arc** (performance): hypothesis → measurement → reveal → next bottleneck. Require partial-victory paragraphs between fixes.
   - **Migration-arc**: why now → what stayed fixed → what changed → cutover → results → what we'd do differently.
   - **Blameless-arc** (postmortem): impact → timeline → causality trace → recovery → prevention. Require at least one failed-mitigation paragraph.
   - **Paper-link-first arc** (AI/agent): paper → gap → system → eval → ablation → limits.
   - **Tutorial-arc**: prerequisites → primer → stepwise → verify → pitfalls → next.
4. Run the **first-200 + last-200 callback-coupling check**: extract the first 200 words and the last 200 words; pair them. If they do not describe the same thread, the post has lost its spine — restructure.

Gist tripwires:

- The lede promises a **result, mystery, stakes, 3-W summary, or paper-link-first**. Not "we want to share."
- Each H2 **resolves a question** the previous section opened.
- The **first 200 and last 200 words** must thread back to each other.

**STOP. Read `references/narrative-and-pacing.md` in full before finalizing the lede, headlines, transitions, and closer.** That file holds the five lede types with archetype pairings, the H2-as-question-resolution rule with worked examples, the story-shape catalogue, the momentum-stall diagnostics, the closer multiple-choice gate (call-to-build / call-to-adopt / open-question / shipping-status-roadmap / prevention-list), and the headline-as-compressed-lede check.

### Phase 6 — Pre-publish Gate

1. Run `python3 <writing-tech-post-dir>/scripts/lint-post.py <draft-path>` (read-only). It scans for slop signatures: triumphal-vocabulary density, hedged-lede patterns, uncaptioned figures, evidence-free percent claims, blame language inside a postmortem context, missing rung tags in outline comments, code blocks over 30 lines without elision markers.
2. Walk the archetype-conditional checklist in `references/pre-publish-checklist.md` row-by-row against the draft. Each row is a hard gate; warnings are not optional.
3. Run the **headline-as-compressed-lede check**: compare the title's commitment (number, decision, surprise, system-name, stakes) against the body's evidence load. A "60x" title with no 60x evidence is **archetype-bait**; reject it.
4. Run the **closer multiple-choice gate**: the closer must be one of the five forward-pointing shapes (call-to-build / call-to-adopt / open-question / shipping-status-roadmap / prevention-list). Summarising closers are a regression.

Gist tripwires:

- No uncaptioned figures, no evidence-free percentages, no blame language in postmortems.
- Disclosure layer matches the archetype (blameless / threat-model / paper-link-first / coordinated-disclosure / what-we'd-do-differently).
- Headline does not promise an archetype the body does not deliver.

**STOP. Read `references/pre-publish-checklist.md` in full before declaring the post publishable.** That file holds the archetype-conditional checklist (postmortem-only / migration-only / performance-only / AI-only / security-only rows), the disclosure-blocker list, the lint-threshold rationale, and the publishable / hold-for-review / rework rubric. **STOP and additionally read `references/anti-patterns.md`** to confirm no banned move slipped in.

## Anti-Patterns (gist tripwires)

The full elaborated list with examples lives in `references/anti-patterns.md`. The list below catches the most common slop during scanning.

- **Buried lede.** Three paragraphs of context before the result, mystery, or stakes.
- **Rung whiplash.** Jumping R1→R5 mid-section with no restated product anchor.
- **Evidence-free percent claims.** "30% faster" with no chart, table, or methodology block.
- **Blame-by-implication in postmortems.** Naming a team, a person, or "the on-call engineer."
- **Code without context.** A 60-line listing with no surrounding prose claim and no elision marker.
- **Depth without product framing.** A deep-dive that never returns to R1 — peer-impressive, user-illegible.
- **Paper-name-dropping.** Citing a paper title without summarising its contribution or linking it.
- **False-precision metrics.** "p99 = 47.3ms" with no measurement window, sample size, or environment.
- **Hedged ledes.** "We wanted to share some thoughts on…", "In this post we'll explore…".
- **Exciting-announcement template.** "We are excited to announce that we are excited to announce."
- **Coordinated-disclosure violations.** Naming the vulnerable version before mitigation deployment.
- **AI-eval-as-anecdote.** "It works great in our tests" without a named benchmark or ablation.
- **Archetype-bait headlines.** Title promises archetype A; body delivers archetype B.
- **Framework-without-instance.** Migration post that names a maturity model but never populates the publisher's own trajectory through it.
- **Bolted-on AI-future close.** Generic "AI handles the long tail" paragraph on a non-AI post.

**STOP. Read `references/anti-patterns.md` in full before any review pass.** The bullets above are tripwires, not the full catalogue.

## Error Handling

- **Unknown or hybrid archetype.** When the artifact does not match exactly one of the eight archetypes, declare it as `hybrid: <primary>+<secondary>` and follow the primary's structural contract. Add a one-sentence note in the post's frontmatter declaring the hybridisation. Do not invent a ninth archetype.
- **No measurable data for a performance claim.** When a performance claim cannot be backed by a chart or table, either (a) downgrade the claim to qualitative language ("noticeably faster on cold start") or (b) cut the claim. Never inflate with placeholder numbers. The post may still be publishable as a launch deep-dive or migration narrative — but **not** as a performance deep-dive.
- **Security disclosure constraints.** When CVE coordination, embargo, or legal review limits what may be disclosed, generate the post with explicit `[REDACTED: pending disclosure]` markers and produce a "what can be shared now / what comes later" addendum. Do not publish until `references/security-and-reliability.md`'s disclosure-blocker checklist clears.
- **Multi-author conflicts.** When two contributors disagree on archetype framing, generate two outlines side-by-side (not two drafts) and surface the disagreement as a decision request. Do not silently pick a side or average them. Tag the diff: archetype-level / rung-level / voice-level / evidence-level.
- **No engineering depth available.** When the underlying work is not engineering-substantive (pure rebrand, OKR recap, executive vision), reject the request and redirect to marketing or thought-leadership channels. This skill does not produce non-engineering long-form.
- **Postmortem before remediation closure.** When remediations are not yet shipped, generate the postmortem with a "what we're doing next" section that names the remediations as in-flight with owners and ETAs. Do not omit the remediations section.
- **AI/agent post with no public benchmark.** When no named benchmark applies, build an ablation matrix against the system's own prior version and label it explicitly as "internal ablation, no external benchmark." Do not name-drop a benchmark that was not run.
- **Lint failure from `scripts/lint-post.py`.** When the lint script flags slop signatures, treat each as a blocker, not a warning. The pre-publish gate does not pass with open lint findings.
- **Metadata validation failure (`scripts/validate-metadata.py`).** Re-read the failure (NAME ERROR / DESCRIPTION ERROR / STYLE WARNING) and rewrite the offending field. Do not bypass the validator.

## When NOT To Use

- API reference documentation — use Diátaxis reference patterns instead.
- README authoring — use `crafting-effective-readmes`.
- Marketing copy, landing pages, pricing pages — use `copywriting`.
- Release notes, changelogs.
- Generic SEO content with no engineering substance.
- Ghost-written executive thought leadership — sibling genre, redirect elsewhere.
- Non-engineering long-form essays — strategic essays (Stripe-style) are a sibling genre; label honestly rather than disguise.
- Internal-only design documents — use `creating-spec`.

## Verification

Before declaring the skill output publishable, confirm:

1. **Archetype committed.** The draft's frontmatter names a primary archetype (and an absorbed archetype when hybrid).
2. **Depth four-tuple committed.** `(opening rung, body residency band, closing rung, traversal)` is recorded.
3. **Outline gate passed.** Every H2 resolves the previous section's open question.
4. **Evidence cadence honoured.** Every artifact is preceded by a claim and followed by a reading; every percent claim attaches to an artifact.
5. **Voice and disclosure pass run.** Publisher register applied; archetype-specific disclosure layer present.
6. **First-200 + last-200 callback coupling.** Threaded back to each other.
7. **Lint clean.** `scripts/lint-post.py` exits 0; no open findings.
8. **Pre-publish checklist passed.** Archetype-conditional gate cleared.

Each verification step has a corresponding gate in `references/pre-publish-checklist.md`. The skill is not done until every gate clears.
