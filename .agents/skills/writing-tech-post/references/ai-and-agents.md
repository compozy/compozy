# AI and Agent Posts (2026 wave)

The AI/agent specialty surface's contract: paper-link-first attribution, named-benchmark + ablation evidence, named-checker idiom, "AI handles the long tail" close motif, and the four sub-variants (research-translation, capability-launch, tooling-platform, strategic-essay).

## Contents

- [Opening: capability claim + paper/repo link in first scroll](#opening-capability-claim--paperrepo-link-in-first-scroll)
- [Nine-stage canonical section sequence](#nine-stage-canonical-section-sequence)
- [Named-benchmark contract (mandatory evidence)](#named-benchmark-contract-mandatory-evidence)
- [Ablation as load-bearing credibility move](#ablation-as-load-bearing-credibility-move)
- [Named guardrails (checkers as concrete artifacts)](#named-guardrails-checkers-as-concrete-artifacts)
- [Closing move: open-source repo or "long tail" motif](#closing-move-open-source-repo-or-long-tail-motif)
- [Four sub-variants](#four-sub-variants)
- [Capability vs limitation register switch](#capability-vs-limitation-register-switch)
- [Silent failure modes (AI/agent slop)](#silent-failure-modes-aiagent-slop)

## Opening: capability claim + paper/repo link in first scroll

Hero image, one-paragraph capability claim, then a paper link or open-source repo link inside the first scroll. The convention signals the post participates in a research conversation even when the host blog is product-facing.

Exemplars:

- MLE-STAR (`074:40-54`) — capability claim followed by *"In our recent [paper], we introduce MLE-STAR, a novel ML engineering agent"* with arXiv link inside "Quick links" block above the first body paragraph.
- Datadog Bits AI eval platform (`019:46-51`) — capability claim before product context.

**Paper-link-first attribution** is the cohort's load-bearing voice move. Pair the citation with a one-sentence summary of the load-bearing claim. Slop variant: name-drop the paper title without summarising the result.

## Nine-stage canonical section sequence

1. **Capability claim** — one paragraph.
2. **Paper / repo link** — first scroll.
3. **Product context** — what task the system performs.
4. **System overview diagram** — named agents, tools, MCP.
5. **Evaluation table** — named benchmarks with baseline.
6. **Ablation / "In-depth analysis" sub-section** — decompose the headline gain.
7. **Guardrails enumeration** — named checkers as concrete artifacts.
8. **Lessons** — what we learned about the eval, the system, the failure modes.
9. **"AI handles the long tail" close** — or open-source repo link.

## Named-benchmark contract (mandatory evidence)

Any AI/agent capability claim attaches to:

- **Cited benchmark.** Public — MLE-Bench-Lite, BrowseComp-Plus, Finance-Agent, PlanCraft, Workbench, SWE-Bench. Or internal with documented composition — BewAIre's curated dataset of malicious + simulated + benign PRs, with weekly updates.
- **Baseline.** MLE-STAR vs AIDE (25.8% → 63.6%). Scaling-agents single-agent baseline against centralised / independent / decentralised / hybrid architectures.
- **Methodology disclosure.** Datadog's evaluation-platform regression — publishing an 11% pass-rate drop and a 35% label-count drop as deliberate short-term degradation in service of a more honest evaluation — is the corpus's standing example of how to disclose methodology shifts.
- **Evaluation slice.** Task-specific reporting: scaling-agents reports *"+81% on parallelizable tasks (Finance-Agent), −70% on sequential tasks (PlanCraft)"* — the negative finding is published as a load-bearing result, not a buried caveat.

Capability claims without a benchmark are **proof-of-concept**, not production. The skill rejects them at the pre-publish gate.

## Ablation as load-bearing credibility move

The cohort's load-bearing momentum move: decompose the headline gain across components.

MLE-STAR's "In-depth analysis" section breaks the medal-rate gain into:

1. Model-usage shift (EfficientNet/ViT vs ResNet).
2. Human intervention (RealMLP integration).
3. Per-checker contribution (debugging agent + data-leakage checker + usage checker).

Scaling-agents publishes a five-architecture box-plot comparison with per-architecture computational complexity, communication overhead, and coordination mechanisms — plus an error-amplification reliability chart showing 17.2× for independent multi-agent vs 4.4× for centralised.

**Distinct obligation:** publish negative findings as load-bearing results, not buried caveats.

**Requirement:** the skill refuses to mark an AI-cohort draft as complete until the H2 chain includes an "In-depth analysis" or "Ablation" section that names what fraction of the headline number each component contributed.

## Named guardrails (checkers as concrete artifacts)

Three exemplars:

- **MLE-STAR** — *"debugging agent, data leakage checker, data usage checker."* Three named artifacts; each has a distinct role.
- **Slack security-investigation** — three-persona Director/Expert/Critic loop with the Critic as a *"weakly adversarial"* check. Knowledge-pyramid diagram showing acquired knowledge flowing up between progressively more advanced models.
- **BewAIre** — prompt engineering + recursive chunking + pattern exclusion + balanced accuracy + manual validation as named checkers with quantitative claims.

**Slop variant:** vague "we added safety checks" without naming what each check does and what it catches. Strong posts name the checker, its role, and its detection contract.

## Closing move: open-source repo or "long tail" motif

Two variants:

- **Call-to-build** — open-source repo link. MLE-STAR closes with `google/adk-samples`; Google production-ready-agents closes with a fork link.
- **"AI handles the long tail" motif** — Meta capacity-efficiency: *"the end goal is a self-sustaining efficiency engine where AI handles the long tail."* The motif is so consistent that its absence is itself diagnostic of a different sub-genre.

**Slop:** bolted-on "AI handles the long tail" on non-AI posts. The trope has become so common it reads as performative when the AI work is not genuinely on the roadmap. If used, the forward-section must name at least two specific applications (build-health + conflict-resolution in Meta WebRTC).

## Four sub-variants

1. **Research-translation** (MLE-STAR, scaling-agent-systems) — strongest constraints, academic-closest voice. Co-authored with the paper's researchers.
2. **Capability-launch** (Meta capacity-efficiency, Datadog BewAIre) — operational metric instead of benchmark, lessons instead of ablation.
3. **Tooling-platform** (Datadog Bits AI eval platform, Datadog hackerbot-claw) — meta-infrastructure as subject; closest to a migration narrative in shape.
4. **Strategic-essay** (contrast case — Stripe checkout) — same vocabulary, different reader contract; **explicitly not a cohort post**. Sibling genre.

## Capability vs limitation register switch

Failure modes shift register: capabilities in declarative present, limitations in hedged conditional.

- **Capability:** *"MLE-STAR uses web search to retrieve relevant and potentially state-of-the-art approaches."*
- **Limitation:** *"LLM-generated Python scripts carry the risk of introducing data leakage."*
- **Limitation:** *"errors cascaded unchecked"* (scaling-agents).
- **Limitation:** *"it would quickly jump to a convenient or spurious conclusion"* (Slack).

These are observations of failure in past or hedged present, never in the assertive present that capability claims use.

**Audit test:** if a post reports limitations in the same voice as capabilities, the post is over-claiming.

## Silent failure modes (AI/agent slop)

- **Capability claims without benchmarks.** Proof-of-concept, not production.
- **Demo-only evidence** — a single transcript or screenshot.
- **Missing ablation.** Headline gain not decomposed.
- **"We used AI agents to…" as a credential.** AI-as-buzzword.
- **Bolt-on AI-future closing on unrelated posts.** "AI handles the long tail" on a non-AI post.
- **Conflating LLM-calls with "agentic."** Single-call systems are not agent systems.
- **Under-disclosed failure modes.** Limitations buried as caveats instead of published as load-bearing results.
- **Paper-name-dropping without summary.** Citing a paper title without summarising its contribution.
- **AI-eval-as-anecdote.** "It works great in our tests" without a named benchmark.
- **No regression disclosed.** Strong posts publish methodological regressions (Datadog's 11% pass-rate drop) as honesty signals. Absence is a possible over-claim flag.
