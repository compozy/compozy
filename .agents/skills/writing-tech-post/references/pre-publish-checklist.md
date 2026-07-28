# Pre-Publish Checklist

The archetype-conditional checklist that gates a draft before publication. Each row is a hard gate; warnings are not optional. The lint script `scripts/lint-post.py` automates a subset of these checks; the rest are manual review gates.

## Contents

- [How to use this checklist](#how-to-use-this-checklist)
- [Universal gates (every post)](#universal-gates-every-post)
- [Postmortem gates](#postmortem-gates)
- [Migration gates](#migration-gates)
- [Performance gates](#performance-gates)
- [AI/agent gates](#aiagent-gates)
- [Security gates](#security-gates)
- [Launch gates](#launch-gates)
- [Tutorial gates](#tutorial-gates)
- [Research-translation gates](#research-translation-gates)
- [Publishable / hold-for-review / rework rubric](#publishable--hold-for-review--rework-rubric)
- [Disclosure blockers](#disclosure-blockers)
- [Lint script integration](#lint-script-integration)

## How to use this checklist

1. Identify the post's primary archetype (and absorbed archetype if hybrid).
2. Walk the **Universal gates** + the **archetype-specific gates** row by row.
3. Mark each as ✅ pass / ⚠️ hold / ❌ rework.
4. Apply the publishable / hold-for-review / rework rubric.
5. Run `scripts/lint-post.py <draft>` to verify the automated subset.

A single ❌ blocks publication. Any ⚠️ requires explicit acknowledgement from the author plus a one-line note in the draft frontmatter.

## Universal gates (every post)

- [ ] **Archetype committed.** Primary archetype named in frontmatter; absorbed archetype named if hybrid.
- [ ] **Depth four-tuple committed.** `(opening rung, body residency band, closing rung, traversal)` recorded in frontmatter or outline header.
- [ ] **Lede matches archetype.** See lede-taxonomy / archetype-pairing in `narrative-and-pacing.md`. Hedged ledes ("we want to share some thoughts on…") are rejected.
- [ ] **H2-as-question-resolution.** Every H2 resolves a question the previous section opened. Noun-phrase TOC-style H2s are rejected unless the archetype is reference-shaped (AWS-style how-to).
- [ ] **Claim → artifact → reading cadence.** Every artifact (chart, diagram, code, table) is preceded by a prose claim and followed by a prose reading. Captions state the *finding*, not the artifact name.
- [ ] **Evidence-form set per archetype.** The archetype's mandatory evidence forms are present (see archetype-specific gates).
- [ ] **First-200 + last-200 callback coupling.** Extract first 200 words and last 200 words; they thread back to each other.
- [ ] **Closer matches one of five forward-pointing shapes.** Call-to-build / call-to-adopt / open-question / shipping-status-roadmap / prevention-list. Summarising closers are rejected.
- [ ] **Headline does not promise an archetype the body does not deliver.** No archetype-bait.
- [ ] **Vendor names live at R5 (implementation), not R1/R2 (framing).** Apply the "remove the vendor's name" test.
- [ ] **Triumphal-language lint.** *"Successfully," "smoothly," "without issue"* appear at most twice per long post (≤5,000 words) or once per short post.
- [ ] **No hedged-lede slop.** *"In this post we'll explore…"* / *"We wanted to share some thoughts on…"* / *"This article is written for…"*
- [ ] **No "we're excited to announce" template.**
- [ ] **Anti-patterns sweep.** Run `anti-patterns.md` line-by-line against the draft; no patterns hit.

## Postmortem gates

- [ ] **Date + scope + impact in the first two sentences.** 3-W summary inside first 200 words.
- [ ] **UTC timeline with defined granularity.** Minute-resolution for incidents under an hour; hour-resolution for longer.
- [ ] **Specific root-cause artifact named.** Commit SHA, PR number, CVE, kernel function, package version. Vagueness is fatal.
- [ ] **Blameless register.** System-subject sentences in causality; no engineer's name attached to causation.
- [ ] **Passive/active polarity correct.** Passive isolates fault; active claims learning.
- [ ] **First-person plural for ownership.** Third-person only in summary.
- [ ] **At least one failed-mitigation paragraph.** *"We attempted X. Unfortunately, it didn't mitigate."*
- [ ] **Quantitative impact.** Numbers, not "severe impact."
- [ ] **Verbatim partner quote** if a third party shares responsibility.
- [ ] **Closing move: prevention commitments per category** (pure postmortem) OR **design principles** (reliability essay variant). No "we have you covered" closure.
- [ ] **No 24-hour postmortem with placeholder action items.** Wait until action items are specific.

## Migration gates

- [ ] **Empathetic legacy framing.** Charity toward predecessor system; no contempt.
- [ ] **Why-now justification.** Concrete trigger named.
- [ ] **Phased plan (numbered).** Three to seven phases, each named.
- [ ] **Per-phase tracking metrics.** Datadog: `postgresql.table.count` aggregated by schema; analogous in other migrations.
- [ ] **Named cutover safety mechanisms.** PG Proxy / shadow tables / dual-stack A/B / per-region cutover.
- [ ] **Quantified scope.** 700+ jobs, 50+ use cases, 30 schemas — explicit numbers.
- [ ] **Paired before/after WITH phase intermediaries.** Not just two states.
- [ ] **Dated status snapshot in close.** *"Phase 1 finished Q1 2025; Phase 2 will continue through 2026."*
- [ ] **"What we'd do differently" paragraph.** Honest disclosure of difficulties.
- [ ] **Forward work names specific applications.** No bolt-on AI-future paragraphs.
- [ ] **Multi-author byline** OR named acknowledgements paragraph.
- [ ] **No framework-without-instance.** Maturity model must be populated with the publisher's own trajectory.

## Performance gates

- [ ] **Felt-experience + stakes opening** (not a number). Promise the arc in paragraph 2.
- [ ] **Five-beat section skeleton per fix.** Hypothesis → instrument → measurement → partial victory → re-arm.
- [ ] **Partial-victory paragraph between fixes.** *"A slight improvement, but clearly still higher than normal."*
- [ ] **Distribution-shift evidence per fix.** Percentile + sample size + measurement window + environment. Mean-only charts rejected.
- [ ] **Per-fix charts on the same axes.** Reader can read the shift visually across interventions.
- [ ] **Named tooling.** `pg_walinspect` / `lldb` / ENA metric IDs / specific instrument used.
- [ ] **Two-author byline** (standard for the genre).
- [ ] **Closing move: honest-recap OR lessons-from-the-trenches.** No wrapped-bow close.

## AI/agent gates

- [ ] **Capability claim + paper/repo link in first scroll.** Paper-link-first attribution.
- [ ] **Cited benchmark** — public (MLE-Bench-Lite, BrowseComp-Plus, etc.) or internal with documented composition.
- [ ] **Baseline reported.** MLE-STAR vs AIDE; single-agent vs multi-architecture; etc.
- [ ] **Methodology disclosed.** What the eval harness does.
- [ ] **Ablation / "In-depth analysis" section.** Decomposes the headline gain across components.
- [ ] **Negative findings published as load-bearing**, not buried caveats.
- [ ] **Named guardrails / checkers.** Each named, with role and detection contract.
- [ ] **Capability vs limitation register switch.** Declarative present for capabilities; hedged future-conditional for limitations.
- [ ] **Closing move: open-source repo link OR "AI handles the long tail" motif** (only if genuinely on roadmap with at least two specific applications).
- [ ] **No "we used AI agents to" as a credential.** No AI-as-buzzword.

## Security gates

- [ ] **Threat-model opens before the fix.** Adversary capability + asset + impact horizon.
- [ ] **CVE number explicit** (response posts).
- [ ] **Upstream commit linked** (response posts).
- [ ] **UTC timeline with parallel workstreams** (response posts).
- [ ] **Upstream attribution** for exploit mechanics. Defer to upstream researcher write-up; do not re-publish working exploit.
- [ ] **Scope caveats explicit.** What the post does not cover.
- [ ] **Probabilistic register for adversary capabilities.** *"Could," "might," "if X then Y."* No FUD framing.
- [ ] **Closing move: follow-up + what we'd do differently + disclaimer.** No "we have you covered" closure.
- [ ] **Named external researchers credited.**
- [ ] **Multi-engineer byline + acknowledgements paragraph** for CVE response.

## Launch gates

- [ ] **Scale-then-headline-number opening.** Headline result above the first H2.
- [ ] **Confident technical "we" register.** Declarative present.
- [ ] **Architecture overview in first or second screen.**
- [ ] **Component walkthroughs in body.**
- [ ] **Results section with quantified envelope.**
- [ ] **Closing move: forward roadmap with named next steps.** Declarative, scoped, not aspirational.
- [ ] **Multi-author byline.**
- [ ] **Headline number not buried below the first H2.**

## Tutorial gates

- [ ] **Use case + capability promise + "we show you how" lede.**
- [ ] **Prerequisites stated.** Explicit. The genre's most common craft failure is missing prerequisites.
- [ ] **Conceptual primer before stepwise.**
- [ ] **Runnable code, end-to-end, copy-paste safe.** No elision in tutorial code; full snippets.
- [ ] **Imperative procedural voice.** Second person or "we show you."
- [ ] **Caveats section.**
- [ ] **Closing move: next-steps pointer with a runnable artifact** (GitHub repo / "what to try next" path).

## Research-translation gates

- [ ] **Discipline-framed problem + paper link in first scroll.** arXiv link inside Quick Links block above first body paragraph.
- [ ] **Method overview section** with diagram.
- [ ] **Evaluation table** with named benchmarks.
- [ ] **Analysis / ablation section.**
- [ ] **Open-source / availability section.**
- [ ] **Acknowledgments block + licensing/scope footnote.** Inheritance from academic papers.
- [ ] **Capability declarative; limitations hedged conditional.**
- [ ] **No corporate-announcement disguised as research.**

## Publishable / hold-for-review / rework rubric

After walking the gates:

- **✅ Publishable** — all universal gates and all archetype-specific gates pass. Lint script exits 0.
- **⚠️ Hold for review** — universal gates pass; ≥1 archetype-specific gate is ambiguous; lint script passes. Author + editor jointly decide.
- **❌ Rework** — ≥1 universal gate fails OR ≥2 archetype-specific gates fail OR lint script fails. Do not publish; return to the relevant phase (1–5) for repair.

## Disclosure blockers

Specific blockers that prevent publication regardless of other gates:

- **Security disclosure constraints** — CVE coordination embargo, legal review pending, customer notification not yet complete. Hold until resolved.
- **Postmortem before remediation closure** — remediations not shipped; publish only if "what we're doing next" section names remediations as in-flight with owners and ETAs.
- **AI/agent post with no benchmark** — must label internal evaluation explicitly as "internal ablation, no external benchmark" with documented composition. Do not name-drop a benchmark that was not run.
- **Migration with no dated status snapshot** — reads as if completion is being claimed before earned. Add the snapshot or downgrade the closing to "in-flight migration update."
- **Performance claim with no measurable data** — downgrade to qualitative language ("noticeably faster on cold start") or cut the claim. Do not inflate with placeholder numbers.

## Lint script integration

Run `python3 <writing-tech-post-dir>/scripts/lint-post.py <draft-path>` (read-only). The script automates the following gates from the checklist:

- Triumphal-vocabulary density (configurable threshold).
- Hedged-lede patterns (regex against the first 200 words).
- Uncaptioned figures (Markdown image syntax without surrounding caption).
- Evidence-free percent claims (numeric percentages not followed by a metric reference within N lines).
- Blame-by-implication patterns ("While our", "Although the team").
- "We're excited to announce" template.
- Code blocks over 30 lines without an elision marker (`// ...` or `# ...`).
- Headline-vs-body callback (extracts first 200 + last 200 words; reports if they share no nouns).

Lint failures are blockers, not warnings. The pre-publish gate does not pass with open lint findings.

The script exits 0 on a clean draft and non-zero with a structured findings report on any failure. Run it before walking the manual gates so the automated subset is already resolved.
