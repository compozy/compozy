# Anti-Patterns

The full banned-move catalogue. Each pattern: name, why it fails, banned example, fixed example, and the archetype(s) it most often appears in.

## Contents

- [Structural anti-patterns (across archetypes)](#structural-anti-patterns-across-archetypes)
- [Lede anti-patterns](#lede-anti-patterns)
- [Voice and disclosure anti-patterns](#voice-and-disclosure-anti-patterns)
- [Evidence anti-patterns](#evidence-anti-patterns)
- [Depth and abstraction anti-patterns](#depth-and-abstraction-anti-patterns)
- [Narrative and closer anti-patterns](#narrative-and-closer-anti-patterns)
- [Per-archetype-specific anti-patterns](#per-archetype-specific-anti-patterns)

## Structural anti-patterns (across archetypes)

- **Archetype-bait headlines.** Title promises archetype A; body delivers archetype B. Erodes publisher trust across a series of posts. Example: a "60x" title with no 60x evidence. Fix: align headline commitment with body evidence; reject mismatches at the pre-publish gate.
- **Framework-without-instance.** Migration post names a maturity model or phase taxonomy but never populates the publisher's own trajectory. Example: introducing "PQC Migration Levels" without showing where Meta itself sits. Fix: every framework must be populated with the publisher's own concrete journey.
- **Triumphal language at any density.** *"Successfully," "smoothly," "without issue."* Triumphal language signals luck or untruth; the genre rejects it. Fix: replace with concrete evidence (named cutover safety mechanism, dated phase completion, concrete job count).
- **One-size-fits-all "engineering blog template."** A skill that emits a single generic template — *Problem / Architecture / Results / Conclusion* — produces archetype-confused posts. Fix: archetype selection gates everything else.

## Lede anti-patterns

- **Buried lede.** Headline number is in section three. *"If the 60x is in section three, the reader has already left."* Fix: launch deep-dives land the headline number above the first H2.
- **Hedged-lede.** *"We wanted to share some thoughts on…"* *"In this post we'll explore…"* *"This article is written for…"* Burns the first paragraph on apologia. Fix: use the five-lede taxonomy (result-first / mystery / stakes-first / 3-W summary / paper-link-first / shipping-status).
- **Exciting-announcement template.** *"We are excited to announce that we are excited to announce."* Banned at Datadog by editorial reflex; reads as marketing on any engineering post. Fix: replace with feature framing + code block (Vercel changelog voice) or stakes-and-problem two-step (GitHub).
- **Abstract-then-concrete inversion.** Opens with a generic industry trend and waits until section two to introduce the specific system. Strategic essays do this on purpose; engineering deep-dives that imitate forfeit pull. Fix: open with the specific system or pathology, defer industry framing to the body if needed.

## Voice and disclosure anti-patterns

- **Blame-by-implication in postmortems.** Naming a team, a person, or *"the on-call engineer."* Fix: system-subject sentences. *"On start-up of v248, systemd-networkd flushes all IP rules"* — not *"the systemd maintainers chose to flush."*
- **Defensive "While/Although" register in postmortems.** *"While our autoscaling capability was outpaced…"* Signals concern for reputation over learning. Fix: *"Our autoscaling was outpaced."*
- **"We have you covered" closures** without naming residual gaps. The promise of completeness invites the reader to discover the gaps later. Fix: name specific defenses, scopes, and explicit residual gaps.
- **Ghost-written-byline.** Engineer's name on a post written by a marketer ventriloquising the engineer. The prose loses the texture of decisions-made-under-uncertainty. Fix: multi-engineer bylines on incident and security posts; refuse to author posts requiring author voice the publisher does not actually have.
- **Mismatched closing CTA.** A 4,000-word postmortem ending with *"Sign up for a free trial"* alienates both audiences. Fix: postmortems close with engineering reflection or prevention; security responses close with follow-up work + hedge; migrations close with dated remaining work; AI posts close with a callable artifact.
- **Over-redacted postmortem.** *"A critical microservice," "an upstream provider."* Readers notice. Fix: disclose why redaction is necessary; do not silently genericise.
- **FUD-style threat framing.** *"The quantum apocalypse," "the AI threat is reshaping…"* Fix: probabilistic register with concrete impact horizons and literature-anchored terms.
- **Paper-name-dropping without summary.** Citing a paper title without summarising its contribution. The academic equivalent of vendor name-dropping. Fix: cite the paper, then summarise the load-bearing claim in one sentence.
- **AI-eval-as-anecdote.** *"It works great in our tests"* without a named benchmark or ablation. Fix: cite a public benchmark or label internal evaluation explicitly ("internal ablation, no external benchmark") with documented composition.
- **Triumphal migration post.** *"We migrated 700 jobs with zero issues"* reads as luck or untruth. Fix: disclose specific problems and how each was resolved ("what we'd do differently" paragraph).
- **Capability and limitation in the same voice.** Reporting failure modes in the assertive present that capability claims use = over-claiming. Fix: declarative present for capabilities; hedged future-conditional for limitations.

## Evidence anti-patterns

- **Evidence-free percent claims.** *"30% faster"* with no chart, table, or methodology block. Fix: every percent claim points to a chart, table, or named benchmark.
- **Mean-only performance charts.** Performance is about distributions, not averages. Fix: percentile reporting with sample size, measurement window, and environment.
- **Code without context.** A 60-line listing with no surrounding prose claim and no elision marker. Fix: split with prose, elide with `// ...`, or replace with source-link plus a load-bearing slice.
- **False-precision metrics.** *"p99 = 47.3ms"* with no measurement window, sample size, or environment. *"99.3% accuracy"* without dataset, baseline, and class-imbalance disclosure. Fix: every metric attaches to its four-tuple of context.
- **Decoration in evidence's clothing.** Marketing-styled dashboards with sparklines, summary cards, and product logos. Before/after charts with cropped y-axes implying improvement without quantifying it. Fix: cover the figure with a hand and re-read the surrounding prose — if the claim still extracts, the figure is evidence; if not, it is filler.
- **Caption-as-label.** Captions starting with *"Figure showing…"* or *"Diagram of…"* Fix: caption states the *finding*, not the artifact name.
- **Missing alt text** or **alt text as label** (not as prose reconstructing the diagram's claim).
- **Screenshots of code instead of code blocks.** Unsearchable, uncopyable, inaccessible. Fix: use code blocks with language tag. Exception: when the DOM is the evidence (diff-lines `054`), not the source.
- **Missing-distribution.** Quoting *"p99 latency dropped from 1s to 100ms"* without graphing the distribution between. Fix: distribution-shift evidence is mandatory for any perf claim.

## Depth and abstraction anti-patterns

- **Rung whiplash.** Jumping >2 rungs without an anchor. Most often: R1 testimonial → R5 vendor product name with no R3 or R4 bridge (customer-story collapse). Fix: gradual descent; no jump exceeds one rung without a transition sentence or section break.
- **Missing-rung descent.** Post moves through rungs in order but skips one — most often R3. The reader is left holding a design they cannot evaluate. Fix: insert an R3 paragraph that states the measurable property in units, with a baseline.
- **Premature R1 ascent in close.** *"We rewrote the storage engine in Rust, which means our customers can sleep at night"* — skips R2-R4. Fix: graduated ascent — end R5, name R3 result, name R2 implication, only then close at R1.
- **Vendor-name padding.** Vendor product names at R1/R2 (framing) instead of R5 (implementation). Fix: name the abstract capability first, then disclose the specific product. Apply the "remove the vendor's name" test.
- **Depth without product framing.** A deep-dive that never returns to R1 — peer-impressive, user-illegible. Fix: R1 anchor in opener and/or closer (mandatory for posts owing user-perceived impact).
- **Customer-story collapse.** R1 testimonial → R5 vendor product name with no R3 bridge → R1 testimonial. **Whiplash when sold as engineering.** Fix: refuse to author customer stories in this skill; route to the customer-story template.

## Narrative and closer anti-patterns

- **Every section a noun phrase.** H2s read as a TOC, not as the spine of an argument. Diagnostic: when the H2s can be skimmed in isolation and re-ordered without changing meaning, the question-chain is broken. Fix: each H2 is a verb-phrase that names an action taken to resolve a question.
- **Callbacks that never connect.** A fact named in the lede is not referenced again. Diagnostic: extract the first 200 words and the closing 200 words — if they cannot be paired into a same-thread statement, the post has no spine.
- **Closers that summarise instead of forward-pointing.** *"In conclusion, we have shown that…"* Almost every read-at-depth closer in the corpus picks one of the five forward-pointing shapes (call-to-build / call-to-adopt / open-question / shipping-status-roadmap / prevention-list).
- **Bolted-on AI-future close** on a non-AI post. Generic *"AI handles the long tail"* paragraph that reads as performative. Fix: if used, the forward-section must name at least two specific applications.
- **Rhetorical-question budget exceeded.** More than three rhetorical-question H2s feels performative. Cap at three per long post.

## Per-archetype-specific anti-patterns

- **Launch — bury the headline number.** The marketing instinct to "set up" the number before revealing it is the genre's most common credibility leak.
- **Postmortem — naming a specific engineer** in the causality section.
- **Migration — aspirational language.** *"We are beginning to roll out…"* without a current state. Or **percentage-complete without a base** — *"we're 75% migrated"* with no denominator.
- **Performance — single-fix-no-failures register.** Real perf work produces failed attempts; their absence reads as fabrication. Or **headline-number-only lede** (launch register on perf archetype).
- **Tutorial — no prerequisites stated.** Wastes reader time and is the genre's most common craft failure.
- **Research — no paper link, no method diagram, no evaluation table.** Reads as a corporate announcement disguised as research.
- **AI/agent — capability claim without benchmark or ablation.** Proof-of-concept, not production. Or **"we used AI agents to" as a credential** — AI-as-buzzword.
- **Security — over-disclosure of working PoC** before upstream patch is widely deployed. Or **missing coordinated-disclosure timeline** for CVE response posts.
- **Strategic essay disguised as engineering deep-dive.** Sibling genre — label honestly rather than disguise. Engineering readers discount disguised essays.
