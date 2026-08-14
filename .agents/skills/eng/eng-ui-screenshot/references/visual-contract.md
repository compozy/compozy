# Visual Contract Mode

A local HTML mock, image, or named OpenDesign artifact cited by a task/spec is
normative unless that source explicitly marks it directional. Visual similarity
is not inferred from code, tests, or an implementation screenshot; it is proved
with a rendered reference/implementation evidence bundle.

Normative binds the reference's **visual language**: layout and composition,
region topology, spacing rhythm, on-screen component anatomy, typography,
tokens, and visible states — what the in-scope surface reads like, never what
it is built from. A prototype is **lossy** on every other axis: demo data,
fixture copy, placeholder brand marks, simplified or omitted product content,
hand-rolled markup standing in for shipped components, and host chrome
redrawn around the named piece are artifacts of prototyping, not
instructions. Each lossy axis has a canonical owner: content and data →
runtime truth; labels and copy → `COPY.md`; marks → the real brand inventory
(`@compozy/ui`); component identity → the `@compozy/ui` inventory and the
owning system's existing composites; host-surface chrome → the live
production surface. Resolve divergences on owned axes toward the owner and
record each as an authorized difference — never invent, delete, or rebrand
product content, and never fork a shipped primitive or rebuild a live host
surface, to match the reference. This scope outranks stricter reproduction
doctrine in any other active skill.

## Contents

- Resolve the contract before implementation
- Capture matched pairs
- Generate the diagnostic artifacts
- Read, classify, and close every divergence

## 1. Resolve the contract before implementation

1. Resolve every cited artifact exactly as written, including repo-relative and
   absolute paths. Do not substitute a same-named copy from another worktree.
2. Record each source path and `git hash-object <path>` identity. Stop on a
   missing or unreadable artifact.
3. Draw the scope boundary: classify every reference region as **in-scope
   piece** (the surface the task names) or **placement context** (host chrome
   the prototype redraws to situate the piece — nav, menubar, session shell,
   task/loop surfaces that already exist in production). Placement context is
   owned by the live surface: integrate the piece into it, judge parity on
   the in-scope regions, and treat divergence between the redrawn host and
   the live host as authorized by default. A host surface is in-scope only
   when the task names it for change.
4. Build the component map: assign every in-scope region an implementation
   owner — an exported `@compozy/ui` primitive (`packages/ui/src/index.ts`),
   an existing domain composite (`web/src/systems/<domain>/`), or, only when
   neither can express the region's read, a new component per the reuse gate.
   Reference markup and CSS never enter the map: where a hand-rolled control
   corresponds to a shipped component, that component is the implementation,
   and its standard internals (focus rings, padding, DOM) are not
   divergences. When a mapped component cannot reproduce the reference's
   read, reach for its variants, props, and tokens first, then a domain
   composite that wraps it — never a parallel hand-rolled copy.
5. Build a contract matrix with one row per required state and viewport:

   | ID | Reference artifact + state | Implementation target + state | Viewport | Fidelity | Authorized differences + authority |
   | --- | --- | --- | --- | --- | --- |

6. Expand phrases such as “every state” into explicit rows. Include shell,
   loading, empty, error, populated, responsive, dialog, tab, and menu states
   when the task or artifact defines them.
7. Treat fidelity as `normative` by default; `normative` binds the
   visual-language axes only, expressed through the component map. An
   authorized difference needs a cited spec, ADR, or canonical-owner
   rule (runtime truth, `COPY.md`, brand inventory, component identity, host
   chrome); implementation convenience is never authority.

Done when every visible acceptance state maps to one reference, one renderable
implementation target, one exact viewport; every reference region has a scope
classification and, if in scope, a component-map owner; and any allowed
difference has a cited owner.

## 2. Capture matched pairs

1. Choose the durable evidence root: `.compozy/tasks/<workflow>/evidence/visual/<task-id>/`
   during task execution, or `<QA_OUTPUT_PATH>/qa/visual-contract/<task-id>/`
   during isolated QA. Keep final evidence out of `/tmp`; the temporary
   `WORKDIR` only owns helpers and logs.
2. For HTML references, serve the artifact from its owning directory through an
   owned static server and record its PID under `WORKDIR`. Never rewrite the
   canonical artifact to make it capturable.
3. Capture the reference before judging the implementation. Open the PNG and
   confirm it is the cited state, not a default route or fallback page.
4. Capture the implementation at the same CSS width, height, device scale,
   content state, and scroll position. Fixture text may differ only when the
   matrix authorizes dynamic data variance.
5. Store every row as:

   ```text
   <evidence-dir>/<contract-id>/
   ├── reference.png
   ├── implementation.png
   ├── side-by-side.png
   ├── diff.png
   ├── comparison.json
   └── review.md
   ```

Done when every matrix row has a visually inspected, dimension-matched PNG
pair; an implementation-only capture is an incomplete row.

## 3. Generate the diagnostic artifacts

Run the materialized mutating evidence helper for every pair:

```bash
bun run "$WORKDIR/visual-diff.mjs" \
  --reference <reference.png> \
  --implementation <implementation.png> \
  --out <evidence-dir>/<contract-id>
```

The helper writes a side-by-side image, highlighted pixel diff, and JSON metric.
The pixel ratio is diagnostic: a high ratio exposes drift inside the in-scope
regions — read a ratio driven by placement context through the scope boundary
instead of chasing it to zero — while a low ratio never overrides a visible
semantic mismatch.

Done when the helper exits zero and all three diagnostic artifacts exist for
every row.

## 4. Read, classify, and close every divergence

Open `reference.png`, `implementation.png`, `side-by-side.png`, and `diff.png`
for every row. Compare, in order:

1. shell boundaries, content frame, and responsive breakpoint;
2. region topology, order, alignment, sizing, and whitespace rhythm;
3. component anatomy, hierarchy, typography, density, and emphasis;
4. control placement and anatomy, icon treatment, and signal colors;
5. borders, radii, surfaces, and motion-relevant visible states.

Within the in-scope regions, a missing/reordered region, wrong shell,
materially different geometry or hierarchy, missing visible state,
hand-rolled markup where the component map names a shipped component, or
uncited visual-language difference blocks parity. Whether a control, metric,
label, datum, or mark exists at all is the canonical owner's call, not the
reference's: a prototype placeholder or omission on those axes resolves toward
the owner as an authorized difference. Build detail resolves the same way: a
mapped component's standard internals differing from the reference's
hand-rolled approximation, and live host chrome differing from the
prototype's redrawn placement context, are non-blocking — record them only
when they change the in-scope read. Text fixtures, timestamps, and
anti-aliasing may be non-blocking only when structure and semantics match.

If a canonical owner (runtime truth, `COPY.md`, the brand inventory, an
existing product surface) conflicts with the reference, the owner wins, but the
row remains blocked until the contract is reconciled or the difference is
recorded with its cited owner. Never relabel an observed mismatch as an
intentional design decision after implementation.

Write `review.md` with the contract row, source identities, capture paths,
machine metric, every observed divergence and disposition, and this exact
frontmatter contract:

```markdown
---
schema_version: 1
contract_id: VC-01
verdict: PASS
reference_source: <exact path>
reference_source_id: <git hash-object output>
implementation_target: <URL or story>
viewport: 1440x900
state: <named state>
reviewer: <agent/runtime identity>
reviewed_at: <ISO-8601 timestamp>
blocking_divergences: 0
authorized_differences: 0
---
# Visual Contract Review
## Divergences
## Authorized Differences
```

Use `FAIL` and the real blocker count until every divergence is fixed or cites
higher authority. Then run the materialized read-only validator:

```bash
bun run "$WORKDIR/validate-visual-contract.mjs" \
  --bundle <evidence-dir>/<contract-id>
```

Done when the validator exits zero for every matrix row, every allowed
difference cites authority, and a reader can reproduce the verdict from the
durable bundle alone.
