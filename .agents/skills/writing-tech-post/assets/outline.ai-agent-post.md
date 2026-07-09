# [AI/AGENT TITLE — "[SYSTEM-NAME]: A [CAPABILITY] agent" or "How we built [SYSTEM]"]

<!--
archetype: ai-agent-post
depth-tuple: (R2 workload scale, R4 agent platform components + R5 specific skills, R1 in book-ended form, Yo-yo)
length-band: 2,000–5,000 (provisional — cohort still consolidating)
byline-norm: multi-author; research-translation variants co-authored with paper researchers
-->

## Quick Links

<!-- R2 — Paper / repo link in the first scroll. -->

- **[Paper]** — [arXiv link]
- **[Repository]** — [GitHub link]

## [Capability claim + task framing]

<!-- R2 — One-paragraph capability claim, then paper-link-first attribution. -->

[One-paragraph capability claim — what the agent does, the named task, why it matters.]

In our recent [paper](LINK), we introduce [SYSTEM NAME] — [LOAD-BEARING SUMMARY].

## Product context

<!-- R2 — Workload scale; why the engineering team built this. -->

[Workload context: scale, fleet size, engineer-hours saved or capacity unlocked.]

## System architecture

<!-- R4 — Named agents / tools / MCP. Diagram. -->

[Architecture diagram with named agent personas (e.g., Director / Expert / Critic), tools, MCP integrations.]

[Each persona's role; each tool's contract.]

## Evaluation setup

<!-- R3 — Named benchmark + baseline + methodology. -->

[Cited benchmark — public (MLE-Bench-Lite, BrowseComp-Plus, Finance-Agent, PlanCraft, Workbench, SWE-Bench) or internal with documented composition.]

| Method | [Benchmark 1] | [Benchmark 2] |
|--------|---------------|---------------|
| [Baseline]            | [X] | [Y] |
| **[SYSTEM NAME]**      | **[X']** | **[Y']** |

[Methodology disclosure — what the eval harness does, what counts as success, what was held back as ground truth.]

## In-depth analysis (ablation)

<!-- R4 + R5 — Decompose the headline gain. Publish negative findings as load-bearing. -->

### Component 1: [Named contribution]

[How much of the headline number this contributed. Include a chart or table.]

### Component 2: [Named contribution]

[...]

### Negative finding (if any)

[Honest disclosure of a regression or a domain where the system underperforms. *"+81% on parallelizable tasks (Finance-Agent), −70% on sequential tasks (PlanCraft)"* style.]

## Guardrails (named checkers)

<!-- R5 — Each guardrail named with role and detection contract. -->

- **[Checker 1]** — [Role; detection contract.]
- **[Checker 2]** — [Role.]
- **[Checker 3]** — [Role.]

## Failure modes

<!-- Hedged conditional voice. Limitations in past or hedged present. -->

[LLM-generated [OUTPUT] carries the risk of [FAILURE MODE].] [In our evaluation, [SYSTEM] would sometimes [INCORRECT BEHAVIOUR].]

## What's next

<!-- R1 in book-ended form — engineer time recovered. Specific applications. -->

[Engineers who previously [TIME-CONSUMING TASK] now [RECOVERED TIME — review AI-generated analyses in minutes].]

[Forward work — at least two specific applications, not generic "AI handles the long tail" unless the work is genuinely on the roadmap.]

## Open-source / availability

[Repo link + how to try the system.]
