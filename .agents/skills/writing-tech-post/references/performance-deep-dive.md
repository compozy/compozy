# Performance Deep-Dive

The performance archetype's contract: detective-arc structure, distribution-shift evidence (not means), iterative bottleneck-peeling, partial-victory paragraph cadence, and the two honest-recap closing variants.

## Contents

- [Opening: felt experience + stakes (not a number)](#opening-felt-experience--stakes-not-a-number)
- [The detective-arc structure](#the-detective-arc-structure)
- [The partial-victory paragraph (load-bearing cadence)](#the-partial-victory-paragraph-load-bearing-cadence)
- [Distribution-shift contract (mandatory)](#distribution-shift-contract-mandatory)
- [Named tooling as evidence](#named-tooling-as-evidence)
- [Three sub-variants](#three-sub-variants)
- [Closing move (two honest-recap variants)](#closing-move-two-honest-recap-variants)
- [Silent failure modes (performance slop)](#silent-failure-modes-performance-slop)

## Opening: felt experience + stakes (not a number)

Performance posts open with the *pathology + stakes*, not the headline number. The post then "promises the arc" in the second paragraph by naming the multi-fix structure to come.

Exemplars:

- GitHub Issues: *"Latency isn't just a metric. It's a context switch. Even small delays add up, and they hit hardest at the exact moments developers are trying to stay in flow."*
- Datadog network-latency: *"Getting paged to investigate high-urgency issues is a normal aspect of being an engineer. But none of us expect (or want) to get paged about every single deployment."*

**Why not result-first?** Result-first is the launch archetype's lede. Performance is a *detective story* — the reader has to live through the investigation. Opening with the result spoils the arc.

**Promise the arc in paragraph 2.** Datadog network-latency names "a series of hidden bottlenecks" within the second paragraph. The reader now knows the post is multi-fix and stays through the H2 chain to collect.

## The detective-arc structure

Each performance section repeats a beat:

> Hypothesis → instrument → measurement → partial victory → re-arm

Datadog network-latency `041:50-90` runs this beat five times — each section closing with the partial-victory cadence ("a slight improvement, but clearly still higher than normal").

**Five-beat section skeleton:**

1. **Baseline metric definition** — what we measured, how, in what units.
2. **Bottleneck identification** — current hypothesis, what made us look here.
3. **Architectural moves** — what we changed.
4. **Distribution shift evidence** — chart aligned with prior charts on the same axes.
5. **Tradeoffs / next bottleneck** — partial-victory paragraph re-arming the investigation.

## The partial-victory paragraph (load-bearing cadence)

The signature beat. Honest disclosure of partial victories between fixes is what licenses the next H2.

Examples (Datadog network-latency, repeated five times):

- *"This was a slight improvement, but clearly still higher than normal."*
- *"Instead of plateauing at about one second, the p99 remote cache latency was now oscillating between 300 ms and one second."*

**Without it,** the post reads as a sequence of unrelated fixes. **With it,** the post reads as ordered investigation.

The skill must require at least one partial-victory paragraph between successive fixes in any detective-arc draft.

## Distribution-shift contract (mandatory)

Any performance claim attaches to four pieces of context, or it is not falsifiable. (Full text in `evidence-diagrams-code.md`.)

- **Percentile.** p50 / p90 / p99. Mean-only charts are rejected.
- **Sample size.** State explicitly.
- **Measurement window.** Time range, traffic level, rollout window. Per-fix charts on the same axes.
- **Environment.** Instance type, browser, OS, hardware.

GitHub Issues uses HPC distribution histograms as the spine of the post — distribution at the start, after cache rollout, after preheating, after Turbo navigations, plus a final percentile chart.

**Missing-distribution anti-pattern:** a post quoting *"p99 latency dropped from 1s to 100ms"* without graphing the distribution between leaves the reader unable to see whether the improvement was uniform or whether a long tail moved while the body stayed put.

## Named tooling as evidence

Vague tooling reads as marketing. Strong perf posts name the exact tool:

- `pg_walinspect`, `pg_test_fsync`, `lldb` (with explicit version requirements: *"Starting in Postgres 15"*).
- ENA metric IDs like `system.net.aws.ec2.bw_in_allowance_exceeded`.
- Network Performance Monitoring as the instrument that surfaced a bottleneck.

The contract: when an investigation depends on a specific tool, name it. When the tool is a publisher's own product, name it **as an instrument**, not as a CTA.

## Three sub-variants

1. **Narrow-detective** — one query, one tool, one root cause. Datadog Postgres upsert.
2. **Multi-bottleneck peeling** — one symptom, chain of independent causes. Datadog network-latency, five fixes. The signature shape of the archetype.
3. **Root-cause-chase-to-depth** — single symptom, descending layers most readers won't visit. Datadog Postgres segfault → LLVM Arm64.

## Closing move (two honest-recap variants)

Both refuse the wrapped-bow close.

- **Honest-recap close** — bulleted chain of fixes + operational change in monitoring/alerting. Datadog network-latency, Datadog scaling-down-to-speed-up. Names the operational gap that should make next time faster.
- **Lessons-from-the-trenches close** — one transferable general lesson + the tool that surfaced it. Datadog Postgres upsert, Postgres segfault.

GitHub diff-lines says explicitly *"this didn't end here"* and lists ongoing work.

A third variant: **distribution-chart close** — close on a graph that shows the distribution moving. The chart *is* the closing argument; prose around it is supplementary.

## Silent failure modes (performance slop)

- **Headline-number-only lede.** "We sped it up by 60%" without disclosure of how. Launch register on performance archetype.
- **Mean-only evidence chart.** Performance is about distributions, not averages.
- **Single-fix-no-failures register.** Real performance work produces failed attempts; their absence reads as luck or fabrication.
- **P99 graph without x-axis baseline.** No time range, traffic volume, or version annotation.
- **Missing distribution shift.** Quoting a percentile move without graphing the distribution between.
- **Missing partial-victory cadence.** Each section claims complete success before the next one starts.
- **Triumphal close ("we sped it up by Xx and everyone was happy").** Performance closers are honest-recap or lessons-from-the-trenches, not wrapped-bow.
