# Archetypes and Structure

The eight canonical archetypes of engineering blog posts, each with its 9-column contract, plus the decision tree for picking the right archetype and the rules for legitimate hybridisation.

## Contents

- [Why archetype selection is load-bearing](#why-archetype-selection-is-load-bearing)
- [A — Launch deep-dive](#a--launch-deep-dive)
- [B — Incident postmortem](#b--incident-postmortem)
- [C — Architecture migration](#c--architecture-migration)
- [D — Performance case study](#d--performance-case-study)
- [E — Developer tutorial](#e--developer-tutorial)
- [F — Research-to-product translation](#f--research-to-product-translation)
- [G — AI / agent post (2026 wave specialty surface)](#g--ai--agent-post-2026-wave-specialty-surface)
- [H — Security / reliability post (specialty surface)](#h--security--reliability-post-specialty-surface)
- [Archetype-straddle rules (legitimate hybrids)](#archetype-straddle-rules-legitimate-hybrids)
- [Decision tree — pick the archetype](#decision-tree--pick-the-archetype)
- [Per-archetype silent failure modes (slop catalogue)](#per-archetype-silent-failure-modes-slop-catalogue)

## Why archetype selection is load-bearing

The corpus converges on eight archetypes: six canonical (launch, postmortem, migration, performance, tutorial, research-to-product translation) plus two specialty surfaces that have hardened into sub-genres (AI/agent, security/reliability). An archetype is the recognisable *shape* of a post: opening move, H2 progression, obligatory evidence form, closing move, length norm, multi-author signature, and the hybridisation rules that decide which shape dominates when a post straddles two genres.

Archetype selection gates every other dimension: voice rules, abstraction-ladder mechanics, evidence forms, and lede architecture all attach to a specific archetype. **Get the archetype wrong and the other dimensions misfire** — a tutorial in postmortem voice loses readers in paragraph one; a migration with launch closing moves reads as wishful marketing.

## A — Launch deep-dive

| Column | Contract |
|--------|----------|
| Opening move | Scale-then-headline-number. Quantified scaling problem in the first sentences, headline result above the first H2. |
| Section sequence | Problem → Architecture overview → Component walkthroughs → Results → Roadmap. |
| Closing move | Forward roadmap with named next steps. "Looking ahead: smarter routing and integrated indexing." Declarative, scoped, never aspirational. |
| Length norm | 5,000–8,000 words. |
| Byline norm | Multi-author. |
| Obligatory evidence | Headline before/after numbers in the lede, with both baseline and slice. |
| Voice register | Confident technical "we". First-person plural, declarative, present tense for live state. |
| Dominant hybrid partner | Migration (lineage narrative inside a launch). |
| Silent failure mode | Burying the headline number. The marketing instinct to "set up" the number before revealing it is the genre's most common credibility leak. |

**Exemplar:** Datadog Rust storage launch (`raw/articles/016-datadoghq-com-evolving-our-real-time-timeseries-storage-again-built-in-rust-for-performance.md`) opens with the volume/cardinality scaling problem and lands "60x ingestion / 5x query" above the first H2. Six-generation lineage hybrid; 5,977 words; four-author byline.

## B — Incident postmortem

| Column | Contract |
|--------|----------|
| Opening move | Date + scope + impact in the first two sentences. The 3 W's + impact (when, what, where, how-bad) inside the first 200 words. Delay reads as evasive. |
| Section sequence | Summary → Background → The incident → Timeline → Contributing factors → Mitigation (including failed attempts) → Action items → Optional lessons → Optional acknowledgments. |
| Closing move | Prevention commitments per category, OR a forward-looking design statement for reliability essays. |
| Length norm | 3,000–12,000 words (standalone 3,000–4,000; paired diptychs 11,000+). |
| Byline norm | Single-author and senior (principal SRE, platform lead). Reliability essays drift to 2–3 authors. |
| Obligatory evidence | UTC timestamps with defined granularity + a specific named artifact (commit, package version, kernel function) for root cause. Vagueness here is fatal. |
| Voice register | Blameless retrospective. System-subject for causality. Organisational "we" for ownership. Passive isolates fault; active claims learning. Engineers' names never appear in the causality section. |
| Dominant hybrid partner | Reliability essay (postmortem zoomed out 6–18 months later). |
| Silent failure modes | Naming a specific engineer; defensive register; "we have you covered" closures; over-redacted ("a critical microservice"); 24-hour postmortems with placeholder action items; missing timeline; "five whys without the answer". |

**Exemplars:**

- Datadog 2023-03-08 (`raw/articles/034-…platform-level-impact.md`) — single senior author; dormant-fault causality back to 2020 systemd PRs `#17477` and `#19287`.
- Canva API gateway outage (`raw/articles/050-canva-dev-canva-incident-report-api-gateway-outage…md`) — explicit Cloudflare quote; "We attempted to work around this issue… Unfortunately, it didn't mitigate" failure-disclosure language; per-category action items.

## C — Architecture migration

| Column | Contract |
|--------|----------|
| Opening move | Empathetic legacy framing + why-now justification. The legacy charity is deliberate — migration posts are read by the engineers who built the prior system. |
| Section sequence | Empathetic legacy framing → Why-now → Phased plan (numbered) → Tracking metrics per phase → Cutover discipline → Where we are now (dated) → Forward work. |
| Closing move | Dated status snapshot + forward work. Explicit and falsifiable: "Phase 1 finished Q1 2025; Phase 2 will continue through 2026." |
| Length norm | 5,000–8,000 words. |
| Byline norm | Multi-author (2–9). Named acknowledgements paragraph common for cross-team work. |
| Obligatory evidence | Phase-completion dates + per-phase tracking metrics + named cutover safety mechanisms (PG Proxy, shadow tables, dual-stack A/B testing) + quantified scope. |
| Voice register | Empathetic strategic "we"; probabilistic for risks; first-person plural across teams. Rejects heroic-engineer and post-hoc-clean registers; retains the messiness. |
| Sub-variants | System rewrite, decomposition, stack replacement, pipeline replacement, component modernization, dual-stack, maturity-level (industry-wide), security-driven. |
| Silent failure modes | Aspirational ("we are beginning to roll out…"); triumphal ("700 jobs migrated with zero issues"); percentage-complete claims without a base ("we're 75% migrated"); toolset-as-story; framework-without-instance; premature-completion. |

**Exemplars:**

- Datadog unwinding shared database (`raw/articles/036-…unwinding-shared-database.md`) — decomposition reference. "Shared database limps along… and then some" lede.
- Meta WebRTC fork escape (`raw/articles/199-…escaping-the-fork.md`) — dual-stack variant; explicit difficulty disclosure ("thousands of duplicate symbol errors").
- Meta PQC migration (`raw/articles/113-…pqc-migration.md`) — maturity-level variant; PQ-Unaware → PQ-Enabled ladder; formal disclaimer paragraph.
- Slack SSH→REST (`raw/articles/211-…ssh-to-rest.md`) — security-driven variant; "We had a massive security surface. Not ideal."

## D — Performance case study

| Column | Contract |
|--------|----------|
| Opening move | Felt experience + stakes, not a number. Promise the arc in the second paragraph by naming the multi-fix structure to come. |
| Section sequence | Baseline metric definition → Bottleneck identification → Architectural moves → Distribution shift evidence → Tradeoffs / next bottleneck. Middle sections repeat: hypothesis → instrument → distribution-shift graph → transition. |
| Closing move | Two species: (a) honest-recap close (bulleted chain of fixes + monitoring change); (b) lessons-from-the-trenches close (one transferable lesson + the tool that surfaced it). Both refuse the wrapped-bow conclusion. |
| Length norm | 1,500–5,500 words (cluster 3,000–4,500; depth-chases extend longer). |
| Byline norm | Two authors. The genre rewards co-authorship because the investigations were collaborative. |
| Obligatory evidence | Distribution-shift charts, **not means**. Per-fix percentile graphs on the same axes so the reader can read the shift visually. Named tooling (`pg_walinspect`, `lldb`, ENA metric IDs) is part of the evidence contract. |
| Voice register | Confident-quantitative narrator-as-detective. "We thought X, then we measured Y." Past for investigation, present for current state, future for remaining work. Honest disclosure of partial victories between fixes. |
| Sub-variants | Narrow-detective (one query, one root cause); multi-bottleneck peeling (one symptom, chain of independent causes); root-cause-chase-to-depth (descending layers most readers won't visit). |
| Dominant hybrid partner | Migration (perf framing for an underlying architecture migration). |
| Silent failure modes | "We sped it up by 60%" headline without baseline disclosure; mean-only evidence chart; single-fix-no-failures register (real perf work produces failed attempts); P99 graph without x-axis baseline; missing distribution shift. |

**Exemplars:**

- GitHub Issues navigation (`raw/articles/017-…from-latency-to-instant.md`) — felt-experience opening; HPC threshold-bucketing into Instant/Fast/Slow.
- Datadog network-latency (`raw/articles/041-…network-latency.md`) — multi-bottleneck peeling reference; five fixes, each section closing "a slight improvement, but clearly still higher than normal."
- GitHub diff-lines (`raw/articles/054-…diff-lines.md`) — six-row before/after metrics table as performance climax; "this didn't end here" close.

## E — Developer tutorial

| Column | Contract |
|--------|----------|
| Opening move | Use case + capability promise + "we show you how". Declare both the *what* (the procedure) and the *expected result* before any code. |
| Section sequence | Prerequisites → Conceptual primer → Stepwise procedure → Working example → Caveats → Next steps. |
| Closing move | Next-steps pointer with a runnable artifact. GitHub repo or "what to try next" path. |
| Length norm | 1,500–3,000 words. |
| Byline norm | Single author (developer advocate or solutions architect). |
| Obligatory evidence | Runnable code (end-to-end, copy-paste safe) + explicit prerequisites. Tutorials that hide code behind prose have misidentified the archetype. |
| Voice register | Imperative procedural. Second person or "we show you." "Create a new Lambda project using Cargo Lambda" — direct address, present tense. |
| Dominant hybrid partner | Launch (tutorials closing with launch-style headline numbers). |
| Silent failure mode | No prerequisites stated. Wastes reader time and is the genre's most common craft failure. |

**Exemplar:** AWS Lambda multi-threaded Rust (`raw/articles/170-…multi-threaded-rust-on-lambda.md`) — explicit prerequisites; vCPU-by-memory table; closes with launch-style "4-6x performance improvements" (tutorial + launch hybrid).

## F — Research-to-product translation

| Column | Contract |
|--------|----------|
| Opening move | Discipline-framed problem + paper link in the first scroll. arXiv link inside the Quick Links block above the first body paragraph. |
| Section sequence | Motivation → Method overview → Evaluation → Analysis (ablation) → Open-source / availability → Acknowledgments + licensing footnote. |
| Closing move | Acknowledgments block + licensing/scope footnote. Inheritance from academic papers. |
| Length norm | 1,500–2,500 words. Deliberately the shortest archetype — it exists to point at a longer paper. |
| Byline norm | Multi-author, plus a separate acknowledgments section listing contributors not named on the byline — inherited from papers. |
| Obligatory evidence | Paper link (preferred). When no paper exists, a method diagram + evaluation table. |
| Voice register | Academic gloss + engineering imperative. Third-person constructions, inline citations, footnotes carry licensing or scope caveats. Capability declarative; limitations hedged conditional. |
| Dominant hybrid partner | Tutorial (research post ships an open-source artifact; close pulls in tutorial structure). |
| Silent failure mode | No paper link, no method diagram, no evaluation table. Reads as a corporate announcement disguised as research. |

**Exemplar:** Google Research MLE-STAR (`raw/articles/074-…mle-star.md`) — arXiv link in Quick Links; MLE-Bench-Lite benchmark; ablation: model usage, human intervention, leakage/usage checkers; licensing footnote.

## G — AI / agent post (2026 wave specialty surface)

| Column | Contract |
|--------|----------|
| Opening move | One-paragraph capability claim + paper/repo link in the first scroll. Hero image, capability claim, then paper or open-source repo link before the second screen. Signals participation in a research conversation. |
| Section sequence | Capability claim → Paper/repo link → Product context → System overview diagram (named agents/tools/MCP) → Evaluation table (named benchmarks) → Ablation / "In-depth analysis" sub-section → Guardrails enumeration (named checkers) → Lessons → "AI handles the long tail" close. |
| Closing move | Open-source repo link OR "AI handles the long tail" motif. The motif's absence is itself diagnostic of a different sub-genre. |
| Length norm | 2,000–5,000 words (provisional — cohort still consolidating). |
| Byline norm | Multi-author; research-translation variants co-authored with the paper's researchers. |
| Obligatory evidence | Named benchmark + ablation + named guardrails + cost/token-burn disclosure (newer). Ablation is the load-bearing credibility move — *not* an aside. Named checkers as concrete artifacts (MLE-STAR's debugging-agent + data-leakage-checker + usage-checker). |
| Voice register | Hybrid academic gloss + engineering imperative. Declarative present for capability, hedged conditional for limitations. Capability and limitation in same voice = over-claiming. |
| Sub-variants | Research-translation (MLE-STAR — strongest constraints, academic-closest voice); capability-launch (operational metric instead of benchmark, lessons instead of ablation); tooling-platform (meta-infrastructure as subject; closest to migration shape); strategic-essay (contrast case — sibling genre, *not* a cohort post). |
| Silent failure modes | Capability claims without benchmarks; demo-only evidence; missing ablation; "we used AI agents to" as a credential; bolt-on AI-future closing on unrelated posts; conflating LLM-calls with "agentic"; under-disclosed failure modes. |

**Exemplars:**

- Google MLE-STAR (`raw/articles/074-…`) — research-translation reference.
- Datadog Bits AI eval platform (`raw/articles/019-…eval-platform.md`) — tooling-platform variant.
- Slack security agents (`raw/articles/160-…security-agents.md`) — three-persona Director-Expert-Critic loop with Critic as "weakly adversarial" check.

## H — Security / reliability post (specialty surface)

| Column | Contract |
|--------|----------|
| Opening move | Threat model first, not the fix. Adversary capability + asset + impact horizon. |
| Section sequence | Threat-model opening → Background/system context → Layered-defense walkthrough → Disclosure-timing section (for CVE responses) → Mitigations close. |
| Closing move | Follow-up work + what we'd do differently + explicit disclaimer. Notably more modest than launch-post closes. |
| Length norm | 1,000–7,000 words. |
| Byline norm | Multi-author for response posts with named acknowledgments paragraph; single or paired for security launches. |
| Obligatory evidence | CVE numbers, upstream commit links, named adversary capabilities, behavioural-detection validation timestamps, named external researchers, explicit scope caveats. |
| Voice register | Careful-declarative. Probabilistic for uncertainty, exact for load-bearing terms. Mature-without-alarmist. "An attacker could save encrypted sessions now and, if a suitable quantum computer is built in the future, decrypt them later" — conditional, calibrated, never "the quantum apocalypse." |
| Sub-variants | Preparedness / non-incident response (Cloudflare Copy Fail — no impact, "outcome" reports the *absence* of impact); industry migration / maturity-model framework (Meta PQC); educational explainer / failure analysis (Docker horror stories). |
| Dominant hybrid partner | Migration (when the security work is a long-running migration). |
| Silent failure modes | FUD-style threat framing; over-disclosure of working PoC; under-disclosure paraphrased into uselessness; "we have you covered" closures; missing coordinated-disclosure timeline. |

**Exemplars:**

- Cloudflare Copy Fail (`raw/articles/232-…copy-fail.md`) — preparedness / non-incident-response reference; CVE-2026-31431; upstream commit `a664bf3d603d`; bpf-lsm surgical mitigation.
- GitHub PQ SSH (`raw/articles/053-…post-quantum-security-for-ssh.md`) — compact security launch (1,145 words); explicit FIPS scope caveat; reader-action one-liner (`ssh -Q kex`).
- Meta PQC migration (`raw/articles/113-…pqc-migration.md`) — industry-migration / maturity-model variant.

## Archetype-straddle rules (legitimate hybrids)

- **Rule 1 — One archetype must be load-bearing.** Pure single-archetype posts are rare; mature posts hybridise. The opening 200 words decide which archetype the reader picks up. Whichever shape the lede establishes is the *dominant* archetype; the secondary archetype is recognised by structural absorption (lineage section inside a launch; performance result inside a migration; tutorial code inside a research post).
- **Rule 2 — Canonical hybrids:**
  - Launch + migration (Datadog Rust storage; signal: chronological "Gen 1… Gen N" framing inside an otherwise launch-shaped post).
  - Migration + performance (GitHub Issues navigation; signal: performance is the framing, migration is the substance).
  - Research translation + tutorial (MLE-STAR closing with ADK link inviting hands-on use).
  - Postmortem + reliability essay (Datadog 034/038 pair — same event, different time horizon).
  - Migration + security (Meta PQC, Slack SSH→REST — why-now leans on security).
  - AI-agent + postmortem (Datadog hackerbot-claw — cohort fingerprint meets incident response).
- **Rule 3 — Archetype-bait headlines are the silent failure.** A headline that promises archetype A and a body that delivers archetype B trains readers to mistrust the publisher's headlines. Once that trust erodes, even well-written posts get skimmed.
- **Rule 4 — Strategic essays are a sibling genre, not a hybrid.** Stripe-style industry essays share vocabulary with engineering archetypes but participate in a different genre. Engineering readers discount disguised essays unless the publisher has earned the right to opine.

## Decision tree — pick the archetype

Answer in order. The first "yes" wins.

1. **Did a user-visible failure occur?** → Postmortem (incident or non-incident-response if preparedness avoided customer impact).
2. **Is the artifact an architectural transition between two states with cutover discipline?** → Migration. Sub-decide: rewrite / decomposition / stack replacement / pipeline replacement / component modernization / maturity-level framework.
3. **Is the artifact a new public capability with a quantified envelope?** → Launch deep-dive. Sub-decide: pure launch or launch + migration (lineage-shaped).
4. **Is the artifact a system that was fine, became slow, then fast, and the work was the hunt?** → Performance deep-dive. Sub-decide: narrow-detective / multi-bottleneck peeling / root-cause-chase-to-depth.
5. **Is the artifact "how to do X, with runnable code"?** → Developer tutorial.
6. **Is the artifact a research result becoming a product-shipped capability?** → Research-to-product translation. Sub-decide: pure or with tutorial close (open-source release).
7. **Is the load-bearing claim an autonomous/LLM system + its capability/correctness contract?** → AI/agent post. Sub-decide: research-translation / capability-launch / tooling-platform.
8. **Is the load-bearing claim about *risk* (vulnerability, adversary capability, defense)?** → Security/reliability post. Sub-decide: preparedness / industry-migration / educational-explainer.
9. **None of the above; the post is market/industry framing?** → Strategic essay (sibling genre — label honestly, do not disguise as deeper-than-it-is technical post).

## Per-archetype silent failure modes (slop catalogue)

Each archetype has a single most-likely failure mode. Catch it during draft review.

- **Launch** — bury the headline number below the first H2.
- **Postmortem** — name a specific engineer in the causality section.
- **Migration** — claim percentage-complete without a base, or omit the dated status snapshot.
- **Performance** — mean-only chart, or "we sped it up by 60%" without baseline + slice + distribution.
- **Tutorial** — no prerequisites stated.
- **Research** — no paper link, no method diagram, no evaluation table.
- **AI/agent** — capability claim without benchmark or ablation.
- **Security** — FUD framing, missing coordinated-disclosure timeline, or "we have you covered" close without naming residual gaps.
- **All archetypes** — archetype-bait headline (title promises archetype A, body delivers archetype B).
