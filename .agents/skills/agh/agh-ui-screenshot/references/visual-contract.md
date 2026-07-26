# Visual Contract Mode

A local HTML mock, image, or named OpenDesign artifact cited by a task/spec is
normative unless that source explicitly marks it directional. Visual similarity
is not inferred from code, tests, or an implementation screenshot; it is proved
with a rendered reference/implementation evidence bundle.

Normative binds the reference's **visual language**: layout and composition,
region topology, spacing rhythm, component anatomy, typography, tokens, chrome
geometry, and visible states. A prototype is **lossy** on every other axis:
demo data, fixture copy, placeholder brand marks, and simplified or omitted
product content are artifacts of prototyping, not instructions. Content and
data are owned by runtime truth, labels and copy by `COPY.md`, marks by the
real brand inventory (`@agh/ui`), and existing product surfaces by their own
contracts. Resolve divergences on those axes toward the canonical owner and
record each as an authorized difference — never invent, delete, or rebrand
product content to match the reference.

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
3. Build a contract matrix with one row per required state and viewport:

   | ID | Reference artifact + state | Implementation target + state | Viewport | Fidelity | Authorized differences + authority |
   | --- | --- | --- | --- | --- | --- |

4. Expand phrases such as “every state” into explicit rows. Include shell,
   loading, empty, error, populated, responsive, dialog, tab, and menu states
   when the task or artifact defines them.
5. Treat fidelity as `normative` by default; `normative` binds the
   visual-language axes only. An authorized difference needs a cited PRD,
   TechSpec, ADR, or canonical-owner rule (runtime truth, `COPY.md`, brand
   inventory); implementation convenience is never authority.

Done when every visible acceptance state maps to one reference, one renderable
implementation target, one exact viewport, and any allowed difference has a
cited owner.

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
The pixel ratio is diagnostic: a high ratio exposes drift, while a low ratio
never overrides a visible semantic mismatch.

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

A missing/reordered region, wrong shell, materially different geometry or
hierarchy, substituted component anatomy, missing visible state, or uncited
visual-language difference blocks parity. Whether a control, metric, label,
datum, or mark exists at all is the canonical owner's call, not the
reference's: a prototype placeholder or omission on those axes resolves toward
the owner as an authorized difference. Text fixtures, timestamps, and
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
