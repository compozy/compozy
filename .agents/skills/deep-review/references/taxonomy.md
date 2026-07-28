# Review Taxonomy

The grammar for defects, advisories, evidence, and objective suppressions. Every review uses one assertive posture: report every specific, actionable survivor regardless of size.

## Result classes

| Class | Categories | Verdict impact | Decision rule |
| --- | --- | --- | --- |
| **Defect** | `⚠️ Potential issue` | Critical/Major block SHIP | The change can produce a wrong result, crash, leak, vulnerability, broken contract, or failing-capable test gap under a concrete input/state. |
| **Advisory** | `🛠️ Refactor suggestion`, `🧹 Nitpick` | Never | The code can remain functional, but a bounded change measurably improves maintainability, simplicity, clarity, naming, documentation, idiom, or conformance with a project rule/skill. |

There is no advisory quota. A small advisory is valid when the premise is observed, the benefit is specific, and the fix is bounded. Formatter-owned style and vague preferences are suppressions, not advisories.

## Severity

| Badge | Class | Bar |
| --- | --- | --- |
| `🔴 Critical` | defect | Plausible production incident, data loss, or security compromise. |
| `🟠 Major` | defect | Wrong behavior, user-visible degradation, unsafe rollout, or a bug awaiting a realistic trigger. |
| `🟡 Minor` | defect or advisory | Narrow real defect, safety erosion, or a meaningful non-local maintainability improvement. |
| `🔵 Trivial` | advisory | Cheap clarity, naming, documentation, small deduplication, or idiom improvement. |

Severity measures impact if unfixed, not confidence or effort. Choose the lower level when between two levels.

## Effort modifier

`⚡ Quick win` means the fix is local and mechanical (roughly ≤15 lines at one site). A larger advisory remains reportable; effort never suppresses it.

## Source attribution

Every result names what produced it through evidence and optional rule ids: repository rule/skill, learning, linter interpretation, verification command, or direct code review. Quote bound rules verbatim through their registry ids and source paths.

## Evidence certificates

Defects start with a causal certificate:

```text
Premise: <observed fact at file:line> → Path: <named caller/input/control flow> → Verdict: <resulting failure>
```

Advisories start with an improvement certificate:

```text
Premise: <observed fact at file:line> → Improvement: <specific measurable benefit> → Fix: <bounded change>
```

Later evidence entries record one `command or file:line → what it showed` check each. Advisories do not invent a runtime failure to clear the defect evidence bar.

## Outside-diff results

A result on untouched lines is allowed only when the diff breaks that code or when a sibling path must mirror the changed invariant. Mark it `in_diff: false` and `hunk: null`; it belongs in the summary rather than an inline comment.

## Objective suppressions

When an investigated candidate is rejected, record it in `suppressions` with one of these reasons and a concrete note:

1. `linter-overlap` — a linter/typechecker lane already reports it.
2. `intentional` — an adjacent justified disable, ADR, comment, or behavior-locking test proves intent.
3. `generated-vendored` — the manifest excludes ownership of generated/vendor code.
4. `formatting` — a configured formatter owns the proposed change.
5. `speculative` — a defect claim has no concrete failure path and no valid advisory premise.
6. `pre-existing` — untouched debt satisfies neither outside-diff clause.
7. `phantom-knowledge` — the claim depends on uninspected code or an irrelevant framework generality.
8. `duplicate-within-job` — the candidate is represented by another result and its anchor appears under `also_applies`.

Profile, volume, low severity, and personal taste are not suppression reasons.

## Volume discipline

There is no numeric cap. Find broadly, refute actively, report every survivor, and account for every investigated rejection. One root cause becomes one result; search every occurrence and list the rest under `also_applies`.
