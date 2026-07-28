# Output Contracts

Exact templates for every artifact. Placeholders in `<angle brackets>`; keep section order and marker strings byte-stable — fingerprints and upserts depend on them. walkthrough.md is orchestrator-authored; review.md and state.json are rendered from findings.json by `scripts/render_review.py`, which implements these templates and the verdict rule — this file is the contract it must keep.

## Contents

- walkthrough.md
- Finding block
- review.md
- review.html
- ReportFindings mapping
- Verdict rule

## walkthrough.md

```markdown
<!-- deep-review:walkthrough -->
## Walkthrough

<one dense paragraph: what the change does across the stack, in prose>

## Changes

| Cohort / File(s) | Summary |
| --- | --- |
| **<cohort name>** <br> `<path>`, `<path>`, `<dir>/{a,b}.go` | <what changed there, 1–2 sentences> |

## Sequence Diagram(s)

<only when the change alters a multi-actor flow (request path, event flow, lifecycle);
one mermaid sequenceDiagram per flow, actors = real components>

## Estimated code review effort

🎯 <1-5> (<label>) | ⏱️ ~<minutes> minutes

## Review details

- **Scope**: <base sha short> → <head sha short> (<incremental round N | full review>)
- **Files**: <n> selected · <n> ignored by filters · <n> skipped (trivial/similar)
- **Posture**: assertive · **Mode**: <workflow|agent-fallback|subagent:runtime>
- **Rubric**: <sources consulted, comma-separated paths>
- **Linters**: <lane: ran/unavailable, ...>
```

Effort scale (calibrate on *reviewable surface* — selected source, not raw file count):

| 🎯 | Label | ⏱️ | When |
| --- | --- | --- | --- |
| 1 | Trivial | ~5 min | mechanical or config-only |
| 2 | Simple | ~12 min | small localized change |
| 3 | Moderate | ~25 min | one subsystem, some cross-file reasoning |
| 4 | Complex | ~60 min | multiple subsystems or a contract change |
| 5 | Critical | ~120 min | core invariants, storage, security, or wide blast radius |

## Finding block (used in review.md and PR comments)

```markdown
_<category badge>_ | _<severity badge>_[ | _<effort badge>_]

**<Imperative one-line claim ending in a period.>**

<Evidence paragraph: the concrete failure mode or improvement, referencing real
symbols and line numbers. State what happens, under which input/state, and why.>

[Also applies to: <path>:<lines>, <path>:<lines>]
[As per coding guidelines [R<NN>] (`<source path>`): "<verbatim rule>"]
Certificate: <defect: Premise → Path → Verdict | advisory: Premise → Improvement → Fix>

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**: review before committing — generated against lines <X>–<Y>.

```suggestion
<exact replacement for lines X–Y, only when the fix is mechanical and complete>
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify this finding against the current code and fix it only if still valid.
In <path> around lines <X>-<Y>, <imperative remediation steps naming exact
symbols and the target behavior>. Reference symbols: <sym1>, <sym2>.
```
</details>
<!-- deep-review:fp:<fingerprint> -->
```

Bracketed lines appear only when they apply. Every result has the certificate for its class. The committable `suggestion` block appears only when the replacement is exact and self-contained; the AI-agents prompt appears only on **Critical and Major defects**.

## review.md

```markdown
# Deep Review — <target> (round <N>)

**Verdict: <SHIP | FIX_BEFORE_SHIP | REWORK>** — <one-line rationale>
**Defects: <n>** (🔴 <n> · 🟠 <n> · 🟡 <n>) · advisories: <n> · duplicates: <n> · resolved since last round: <n> · merged duplicate reports: <n>

<walkthrough.md content, inlined>

## Findings

### <path>

<finding blocks for this file, severity-descending>

## Outside diff range

<finding blocks with in_diff: false, grouped per file; "None.">

## Spec conformance

<only with --spec; one row per contract artifact from the context pack:>
| Artifact | Assessment |
| --- | --- |
| `<path>` | conforms — no divergence found \| <n> violation(s): <finding claims> |

## Duplicates (unresolved from round <N-1>)

<one-line entries: badge · claim · original round; "None.">

## Advisories

<full advisory blocks grouped per file; "None.">

## Review observability

<candidate, suppression, and complete defect/polish hunk coverage counts>
```

`review.md` orders files by max severity, then path.

## review.html

The human-facing dashboard, emitted by `scripts/render_html.py`, hydrates `assets/REVIEW_UI.html` with defects and advisories in separate sections plus suppression and coverage observability. The fixed template is self-contained (inline CSS/JS, no network) and `review.html` is never hand-edited. Before render_review.py writes the state entry, the verdict remains a neutral "round in progress" state.

## ReportFindings mapping

When the harness exposes the ReportFindings tool, call it once after review.md is written: one entry per defect and advisory, defects first by severity and then advisories by file. Use `file`/`line` from the anchor, `summary` from the claim, and the matching evidence certificate. Omit `verdict`; the certificate and refutation checks are the confidence signal.

## Verdict rule

Derive after Step 4's merge from open **defects only**; advisories never change the verdict:

- **SHIP** — no Critical or Major defect is open; Minor defects and every advisory ship as follow-ups. With `--spec`, the Spec conformance section must also be complete with zero open parity violations.
- **FIX_BEFORE_SHIP** — at least one Critical/Major defect is open, and remediation is local: the change's shape is right and each defect names a bounded fix.
- **REWORK** — defects show structural failure needing redesign: a parity violation the implementation approach cannot express, one root cause across ≥3 cohorts, or a Critical whose fix rewrites the change's core. REWORK always carries a named rationale; otherwise FIX_BEFORE_SHIP is the ceiling.

The verdict lands in review.md, state.json, and the final message. render_review.py derives SHIP / FIX_BEFORE_SHIP from defects with round status new or duplicate and accepts REWORK only through `--rework "<rationale>"` backed by structural defects.
