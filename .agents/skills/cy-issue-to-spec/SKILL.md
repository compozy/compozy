---
name: cy-issue-to-spec
description: >-
  Publishes a Compozy spec set to compozy-specs and mirrors it into a GitHub +
  Linear issue pair — or starts from a bare issue and authors the spec first.
disable-model-invocation: true
argument-hint: "[spec slug or issue number]"
---

# Issue ↔ Spec

One feature lives in three places: the working **spec set** under `.compozy/tasks/<slug>/`, its published folder in the `compozy/compozy-specs` repo, and its **mirror** — a GitHub issue and a Linear issue that read identically. This skill moves a feature between those places and leaves all three agreeing.

## Branches

| Starting point | Enter at |
| --- | --- |
| A spec set already exists under `.compozy/tasks/<slug>/` | Step 1 |
| A GitHub issue exists carrying no spec behind it | Step 0 |

## Step 0 — Author the spec

Only when starting from a bare issue.

Read the issue and restate its scope in one sentence. When it bundles separable features, split it first: retitle the original to the narrower scope, open a second issue for the remainder, and cross-link both — then spec one of them.

Activate `cy-spec-preflight`, then `cy-create-spec`. Offer `cy-spec-peer-review` after the user approves the spec. Run `cy-create-tasks` last.

*Done when:* `.compozy/tasks/<slug>/` holds `_spec.md` (or `_prd.md` + `_techspec.md`), its companions, `adrs/`, and `_tasks.md`, and the user has approved the spec.

## Step 1 — Stage the spec set

Run the staging helper (**mutating** — writes into the specs repo working tree):

```bash
python3 .agents/skills/cy-issue-to-spec/scripts/publish-spec.py \
  --spec-dir .compozy/tasks/<slug> \
  --specs-repo <specs-repo-checkout> \
  --repo-root "$(pwd)"
```

It publishes `_*.md`, `task_NN.md`, and `adrs/*.md`; it leaves `analysis/`, `qa/`, `memory/`, `evidence/`, `reviews-*/`, `state.yaml`, and handoff notes in the workspace. Pass `--include '<glob>'` for a document that carries spec content under a name outside those patterns, and `--archived` when the workstream is already closed.

The folder is named `YYYY-MM-DD-<slug>`, dated by when the work began: the source folder's own date prefix when it carries one, else `state.yaml`'s `created_at`, else the earliest document. Active specs sit at the repo root; closed ones under `_archived/`.

*Done when:* the helper exits 0, reporting the staged path, file count, and a clean scan.

## Step 2 — Resolve the report

The helper scrubs machine-local paths and repairs `../adrs/` links itself. Two findings need a decision:

- **A surviving local path or a possible secret** exits the helper non-zero. Edit the staged document, then re-run the scan.
- **A broken relative link** points at something the filter left behind. Either publish that material too, or accept the dangling reference — the repo README already tells readers that older documents point at files they will not find.

*Done when:* every reported path, secret, and link is fixed or consciously accepted.

## Step 3 — Index and push

Add one row to the specs repo `README.md`, in the **Active** or **Archived** table: the folder link, one sentence on what the spec covers, the tracking-issue link, and the `Documents` column naming its shape (`spec`, `prd + techspec`, `brief + techspec`, …). Newest first.

Commit as `docs: add <slug> spec set` and push.

*Done when:* every relative link in the README resolves and the commit is on `main`.

## Step 4 — Write the issue body

Read `.agents/skills/cy-issue-to-spec/references/issue-body.md` in full and build the body against it.

*Done when:* the body opens with the spec-set link block and every section of the skeleton is either written or deliberately absent.

## Step 5 — Open the mirror

Create or update the GitHub issue: title matching the spec's subject, labels `roadmap` + `planned` (removing `needs-plan`), the Step 4 body, and a closing `Roadmap item tracked in Linear: [CY-NNN](…)` line.

Create or update the Linear issue with the same title and the same body, swapping the footer for `Tracked on GitHub: <url>`. Linear renders GitHub callout syntax as plain text — convert a `> [!NOTE]` block to a bold sentence.

Verify with the audit helper (**read-only**):

```bash
python3 .agents/skills/cy-issue-to-spec/scripts/check-tracker-sync.py --linear-json <export>
```

Save the Linear MCP `list_issues` result to a file first; without `--linear-json` the helper audits only the GitHub half.

*Done when:* the helper exits 0 with no defects, and both issues carry a link to each other.

## Reference

- Publishing an already-archived spec skips Step 0 and Step 4's tracking link when no issue exists — the README row is then the only index entry.
- Renaming a published folder breaks the links already sitting in both trackers; update the issue bodies in the same change.
- `references/issue-body.md` — the issue-body skeleton and where each part comes from in the spec.
- `scripts/publish-spec.py` — **mutating**; stages, scrubs, scans, and link-checks a spec set.
- `scripts/check-tracker-sync.py` — **read-only**; audits titles, states, and links across the mirror.
