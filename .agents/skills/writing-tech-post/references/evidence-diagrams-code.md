# Evidence, Diagrams, and Code

The twelve-form evidence taxonomy, captioning conventions, the `claim → artifact → reading` cadence, code-curation rules, and the distribution-shift / named-benchmark contracts.

## Contents

- [The atomic unit: claim → artifact → reading](#the-atomic-unit-claim--artifact--reading)
- [The twelve evidence forms](#the-twelve-evidence-forms)
- [Emergent 2026 evidence forms](#emergent-2026-evidence-forms)
- [Captioning conventions (six rules)](#captioning-conventions-six-rules)
- [Prose↔evidence cadence (three shapes)](#proseevidence-cadence-three-shapes)
- [Code-curation rules](#code-curation-rules)
- [Distribution-shift contract (mandatory for performance)](#distribution-shift-contract-mandatory-for-performance)
- [Named-benchmark contract (mandatory for AI/agent)](#named-benchmark-contract-mandatory-for-aiagent)
- [Per-archetype mandatory evidence forms](#per-archetype-mandatory-evidence-forms)
- [The "no-decoration" rule](#the-no-decoration-rule)

## The atomic unit: claim → artifact → reading

Every evidence asset must obey this triple:

1. **Claim** — a prose sentence that sets up what the reader should look for.
2. **Artifact** — the chart, diagram, code listing, table, or screenshot.
3. **Reading** — a prose sentence that interprets what the reader has just seen.

Textbook execution from Datadog network-latency `041:62-63`:

> *"We noticed that when `counter` restarted, the Envoy sidecar would max out its CPU allocation and get throttled, as shown in the graph below."* **(claim)**
>
> *[CPU usage chart]* **(artifact)**
>
> *"which explained the spikes in TCP retransmits and remote cache latency."* **(reading)**

Any artifact missing either the preceding claim or the following reading is a draft warning. Any claim without an artifact on the same screen is a credibility leak.

## The twelve evidence forms

Each form names a specific claim. Pick the form by the claim, not the artifact you have.

| Form | Claim | Caption convention | Common slop variant |
|------|-------|-------------------|--------------------|
| **Architecture diagram** | "Here are the components and how data crosses between them." | Noun phrase identifying the system tier. The diagram contains nothing the post will not name in prose. | Boxes never reappear in prose; arrows unlabelled; unique iconography forces icon-to-meaning mapping mid-narrative. |
| **Sequence diagram** | "Here is the order of operations across components, including failure handoffs." | Verb-led ("Sequence diagram for the initial Courier design"). Use only when timing matters. | Steady-state topology depicted as a sequence; time axis unlabelled. |
| **Flowchart / decision tree** | "Here is the branching logic of a process." | Names the decision being branched. | Branches non-exhaustive; "Yes/No" labels missing; used for architecture (a static structure is not a flow). |
| **Data-flow / pipeline diagram** | "Here is what the bytes look like as they move." | Names the data stage. Diagram and prose use identical terminology — if the prose says "fragment," the diagram cannot say "shard." | Vocabulary drift between diagram and prose. |
| **Before/after migration diagram** | "Here is what changed." | Names both the dimension and the delta. Migration archetypes require **phase intermediaries**, not just two states. | Cropped y-axes; no annotation of what the delta represents. |
| **Code snippets** | "This is the actual code, not a paraphrase." | Language-tagged code block; provenance link when borrowed. | Length thresholds: 1–15 lines at a glance; 16–30 lines require slowdown; >30 lines without intermediate prose almost always fail. |
| **Shell sessions / SQL traces** | "We ran this and observed this output." | Name the host, the time, and the command if any matter. | Prompt text decorative; host IPs inconsistent across snippets; output trimmed without an ellipsis marking the cut. |
| **Assembly / disassembly** | "The bug is at this level of the stack." | Print the address span and the function symbol. Identify architecture (Arm64 vs x86_64). | No register dump; reader not told which line is the focus. |
| **Charts and time-series plots** | "Here is the numeric shape of a phenomenon over a range." | Both axes labelled with units, time range stated, baseline shown if any, specific metric named in title or caption. | Single number ("we sped it up 60%") without distribution. |
| **Distribution charts** | "The population shape changed" — a stronger claim than "the mean changed." | State bucket width; y-axis labelled as count or share; tail visible. | "Improvement" claimed only on the median while the tail is hidden. |
| **Tables** | "Here is the comparison in normalized rows." | Caption names measurement conditions. | Units inconsistent within a column; "improvement" column absent; sprawls past ten rows without sub-grouping. |
| **Screenshots** | "This is what the operator saw." | Redact sensitive content; name what to look at; crop tightly. | Used as a substitute for code (always show the code); zoom level unstated; chrome distracts from focus. |
| **Embedded quotes / citations** | "This is not our wording." | Blockquote + link to original. | Quoted out of context to seem to support a stronger claim than the source actually makes. |

## Emergent 2026 evidence forms

Sub-types of existing slots with distinct obligations, surfacing in the AI/agent + security cohorts:

- **Named-benchmark result tables / charts** — first-class evidence in the AI cohort. Must cite a public benchmark (MLE-Bench-Lite, BrowseComp-Plus, Finance-Agent, PlanCraft, SWE-Bench) or an internal benchmark with documented composition, plus baseline and evaluation slice.
- **Ablation matrices / box-plot comparisons** — the cohort's credibility move. Decompose the headline gain across components. Publish negative findings as load-bearing results, not buried caveats.
- **Agent-trace transcripts** — closer to a shell capture than a chart. Identify agent role, timestamp, and what was redacted.
- **Multi-persona / role-graph diagrams** — hybrid of flowchart and architecture diagram. Every named persona must recur in prose and at least one structured-output schema.
- **Knowledge-pyramid / cost-shape diagrams** — disclose operational cost without dollar amounts (Slack's Director/Expert/Critic pyramid).
- **Eval-harness evolution diagrams** — trace the evaluation platform's phases; each diagram anchored to a quantitative claim (Datadog's 95% validation-time reduction, 11% pass-rate regression, 30% root-cause quality increase).
- **Structured-output schemas / JSON rubrics** — code snippet whose claim is contractual ("the exact format the model is constrained to produce"), not illustrative.
- **Alert / Slack-message screenshots as deployment evidence** — proves the system is wired into a real on-call workflow, not just running in a notebook.

## Captioning conventions (six rules)

1. **Declarative, not imperative.** A caption states what the figure shows, not what the reader should do with it. *"High-level overview of real-time timeseries database (RTDB) node"* — declarative. *"Note the three subsystems on the left"* — wrong.
2. **Tense.** Static diagrams take present tense ("the request flows through Envoy"). Time-bounded charts take past tense ("p99 latency dropped from 1s to 100ms after we increased Envoy CPU").
3. **Subject of the caption is the artifact, not the system.** *"The diagram shows the reverse path filter diagnosing traffic coming in on ens6 as Martian packets"* — diagram-as-subject. *"Reverse path filtering drops Martian packets coming in on ens6"* — wrong; the system is the subject and the caption has become redundant prose.
4. **Alt text as prose, not as label.** Alt text should let a screen-reader user reconstruct the diagram's claim. GitHub Issues' flowchart alt text reads as a step list — the corpus's strongest example.
5. **Code blocks must be syntactically copyable.** No smart quotes, no zero-width spaces, no ellipses inside code, no line numbers the reader has to strip.
6. **Captions name the finding, not the artifact.** Strong: *"Increasing Envoy's CPU helped mitigate the high latency, which now oscillated between 300ms-1s"* — the chart's reading. Weak: *"Latency chart"* — the chart's label. Captions starting with "Figure showing…" or "Diagram of…" are rejected.

## Prose↔evidence cadence (three shapes)

- **Lead-with-architecture.** A high-level system diagram appears in the first or second screen. The diagram functions as a **scope contract**: here is the map, we will not relitigate what is outside the box. Used by Datadog's Husky and RTDB posts.
- **Chart-as-cold-open.** A distribution chart appears immediately and reframes the post around its shape. The chart is the thesis. GitHub Issues' navigation-mix distribution graph; scaling-agents' task-performance hero image.
- **Alternating block.** Six successive sections follow *prose claim → diagram → prose interpretation*. The reader's eye and mind refresh on every screen. Husky `040`; DNS post `022` (charts + shell captures); `041` runs five iterations of *hypothesis → measurement → distribution graph → partial-victory paragraph*.

## Code-curation rules

1. **Minimum readable fragment over completeness.** Show the smallest excerpt that supports the claim; link the rest. The DNS post `022` uses 1–3 line kernel excerpts (`if (no_addr) goto last_resort;`).
2. **Elision marker conventions.** When cutting code, mark the cut explicitly with `// ...` or `# ...` ellipses; never silently truncate.
3. **Provenance.** Borrowed code (Linux kernel, Postgres, open-source library) must link back to its source line with a named PR or file/line link.
4. **Tutorial vs deep-dive split.** Tutorial code is runnable, end-to-end, and copy-paste-safe. Deep-dive code is the minimal disclosing slice — explanations are allowed fewer code examples than tutorials, but each must be irrefutable rather than illustrative.
5. **Language tag + syntax highlighting are mandatory.** No screenshots of code instead of code blocks (a screenshot of an IDE displaying code is unsearchable, uncopyable, and inaccessible). Exception: when the DOM is the evidence (diff-lines `054`), not the source.
6. **Code-without-context anti-pattern.** A 30+ line block with no surrounding prose claim almost always fails. Fix: split with explanatory prose, elide irrelevant lines with `// ...`, or link the full file and quote only the load-bearing region.

## Distribution-shift contract (mandatory for performance)

Any performance claim attaches to four pieces of context, or it is not falsifiable:

- **Percentile.** p50 / p90 / p99 / pXX. Mean-only charts are rejected.
- **Sample size.** Number of pull requests, investigations, competitions, nodes. State explicitly.
- **Measurement window.** Time range, traffic level, rollout window. Charts on the same axes so the reader can read the shift visually.
- **Environment.** Instance type, browser, OS, hardware. `054:107` reports *"m1 MacBook pro with 4x slowdown"* — without this, the INP numbers are not falsifiable.

The corpus's standing rebuke is the **missing-distribution anti-pattern**: a post quoting "p99 latency dropped from 1s to 100ms" without graphing the distribution between leaves the reader unable to see whether the improvement was uniform or whether a long tail moved while the body stayed put.

## Named-benchmark contract (mandatory for AI/agent)

For 2025–26 AI/agent capability claims, the contract substitutes capability-and-correctness for latency-and-throughput. Discipline is identical:

- **Cited benchmark.** Public (MLE-Bench-Lite, BrowseComp-Plus, Finance-Agent, PlanCraft, Workbench, SWE-Bench) or internal with documented composition.
- **Baseline.** MLE-STAR vs AIDE (25.8% → 63.6%); scaling-agents single-agent vs centralised/independent/decentralised/hybrid.
- **Methodology.** What the eval harness does — Datadog's evaluation-platform regression (publishing an 11% pass-rate drop and 35% label-count drop as deliberate short-term degradation) is the standing example.
- **Ablation.** Decompose the headline gain. MLE-STAR's "In-depth analysis" breaks the medal-rate gain into model-usage shift, human intervention, and per-checker contribution. Posts without ablation read as proof-of-concept, not production.

## Per-archetype mandatory evidence forms

Each archetype has a non-negotiable evidence form set:

- **Performance deep-dive** → distribution-shift evidence per fix + named tooling (`pg_walinspect`, `lldb`, ENA metric IDs) + partial-victory disclosure between fixes.
- **Postmortem** → UTC timestamps with defined granularity + named services and versions + quantitative impact + specific root-cause artifact (commit SHA, PR number, CVE).
- **Architecture migration** → paired before/after **with phase intermediaries** + dated phase-completion milestones + named cutover safety mechanisms + quantified scope.
- **AI/agent cohort post** → named benchmark + ablation + guardrails enumeration (named checkers / personas) + tools-and-MCP diagram + open-source repo or product preview at the close.
- **Incident-postmortem timeline** → both prose and tabular or graphical timeline.
- **Security post** → CVE number + upstream commit link + named adversary capability + behavioural-detection validation timestamp + named external researchers + explicit scope caveats.

## The "no-decoration" rule

Marketing-styled dashboards with sparklines, summary cards, and product logos are not engineering evidence. Before/after charts with cropped y-axes imply improvement without quantifying it. Undated traces cannot be correlated. Assembly listings without architecture labels do not localise the bug.

**The single test:** *cover the figure with a hand and re-read the surrounding prose — can the reader still extract the claim? If yes, the figure is doing additional work and is evidence. If no, the prose has not earned the figure and the figure is filler.*
