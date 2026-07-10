# Evidence-Block Snippets

Reusable captioned-evidence templates. Each snippet honours the `claim → artifact → reading` triple and the captioning conventions.

## Figure with finding caption

```markdown
[Claim sentence that sets up what the reader should look for.]

![Alt text written as prose reconstructing the diagram's claim — e.g., "A line graph shows the count of instance conntrack entries over time for different node types, with the spike at 14:32 UTC corresponding to the deployment window."](path/to/figure.png)

*Figure N: [Finding stated declaratively — "Latency dropped from p99 1s to p99 100ms after the cache rollout." Never "Figure showing latency." Never "Diagram of the cache."]*

[Reading sentence interpreting what the reader has just seen.]
```

## Distribution-shift chart (performance)

```markdown
[Claim — what the change was supposed to do.]

![Alt text describing the bucket boundaries and the visible shift across versions.](path/to/distribution.png)

*Figure N: p99 latency distribution before (top) and after (bottom) the cache rollout, measured on m1 MacBook Pro with 4x slowdown across 12,500 navigations during the 2026-04-08 to 2026-04-15 rollout window.*

[Reading — what the shift confirms or refutes.]
```

## Before/after metrics table (performance closer)

```markdown
| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| Total lines of code        | 2,800       | 2,000        | -27%  |
| Unique component types     | 19          | 10           | -47%  |
| Components rendered        | ~183,504    | ~50,004      | -73%  |
| DOM nodes                  | ~200,000    | ~180,000     | -10%  |
| Memory                     | 150-250 MB  | 80-120 MB    | -50%  |
| INP (large PR)             | 450 ms      | 100 ms       | -78%  |

*Table caption: Evaluated on a pull request using a split-diff setting with 10,000 line changes on m1 MacBook Pro with 4x slowdown.*
```

## Architecture diagram with scope contract

```markdown
*The diagram below outlines the high-level architecture of [SYSTEM]. Anything outside the dashed box is out of scope for this post.*

![Alt text walking the named components and their relationships in prose.](path/to/architecture.png)

*Figure N: High-level overview of [SYSTEM] node. [Component A] receives [DATA] from [SOURCE]; [Component B] forwards [PROCESSED DATA] to [DESTINATION]; [Component C] persists [STATE] in [STORE].*
```

## Sequence diagram (timing-sensitive)

```markdown
[Claim — why the order matters.]

![Alt text describing the message flow chronologically as a step list.](path/to/sequence.png)

*Figure N: Sequence diagram for the [INITIAL / FAILURE / RECOVERY] design. Top-to-bottom denotes time. Failure handoff occurs at step 4 when [COMPONENT] times out.*

[Reading — how this ordering surfaces the bug or the fix.]
```

## UTC-timeline table (postmortem)

```markdown
## Timeline

| Time (UTC)   | Event |
|--------------|-------|
| 2024-11-12 09:08 | Automated upgrade was triggered. |
| 2024-11-12 09:11 | First customer-facing 5xx surfaced in [REGION]. |
| 2024-11-12 09:14 | On-call paged; investigation began. |
| 2024-11-12 09:23 | First mitigation attempted (task count scaling). It did not mitigate the issue. |
| 2024-11-12 09:41 | Second mitigation deployed (CDN block + traffic shift). |
| 2024-11-12 10:00 | canva.com fully recovered. |
```

## Code listing with elision marker

```markdown
[Claim — what the snippet illustrates.]

```rust
// Excerpt from src/router.rs lines 145–162 (see GitHub PR #482)
pub fn route_request(req: Request) -> Response {
    let target = lookup_target(&req.host)?;
    // ... validation and authorization checks elided ...
    let response = forward(target, req).await?;
    record_metric(&response);
    response
}
```

[Reading — what to notice in the code; what the elided section does.]
```

## Named-benchmark result table (AI/agent)

```markdown
| Method | MLE-Bench-Lite (Kaggle) | Finance-Agent | PlanCraft |
|--------|--------------------------|---------------|-----------|
| AIDE (baseline)         | 25.8% | 41.2% | 28.7% |
| **MLE-STAR**             | **63.6%** | **74.3%** | 8.4% (regression) |

*Table caption: Medal rate on MLE-Bench-Lite across 100 Kaggle competitions; baseline AIDE evaluated on the same competition set. Note: MLE-STAR regresses on PlanCraft (sequential tasks); ablation in §[In-depth analysis] decomposes the parallelisable vs sequential gain.*
```

## Failed-mitigation paragraph (postmortem)

```markdown
We attempted to work around this issue by significantly increasing the desired task count manually. Unfortunately, it didn't mitigate the issue, and additional tasks ended up immediately failing health checks once they came up.

[Next mitigation paragraph naming what we tried next and why.]
```

## "What we'd do differently" paragraph (migration)

```markdown
In retrospect, we'd underestimated the complexity of [SPECIFIC SUB-PROBLEM]. The dual-stack approach surfaced thousands of duplicate symbol errors that required modifying hundreds of thousands of lines across thousands of files. If we were starting over, we would [SPECIFIC CHANGE WE'D MAKE].
```

## Verbatim partner quote (postmortem with vendor responsibility)

```markdown
Cloudflare provided the following statement regarding the contributing factor on their side:

> [Verbatim quote from the partner, blockquoted, with attribution to a named role or team.]
>
> — [Named team / role at partner company]

We've been working closely with [PARTNER] to gain an in-depth understanding of the contributing factor. On our side, we underestimated [PUBLISHER-SIDE FACTOR].
```
