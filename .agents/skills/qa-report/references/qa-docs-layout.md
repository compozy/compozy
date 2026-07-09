# QA Docs Layout

The canonical living QA tree. One tree per project, committed to the repository, appended to by every QA round. This layout is the contract between `qa-report` (writes plans, registry, tracker) and `qa-execution` (writes results, reports, evidence).

## Contents

- The canonical tree
- What is durable vs per-run
- Bootstrap procedure (new project)
- Adoption procedure (project with scattered QA artifacts)
- Evidence policy (lean by design)
- Anti-patterns

## The canonical tree

```
<qa-docs-path>/                      # default: docs/qa/
├── README.md                        # project QA conventions, area codes, changelog
├── state.csv                        # living scenario tracker (see state-schema.md)
├── personas.md                      # project personas (instances, not methodology)
├── journeys/
│   └── J-<NN>-<slug>.md             # journey map (YAML) + Mermaid flowchart
├── charters/
│   └── CH-<NNN>.md                  # session charters; status updated in place
├── bugs/
│   └── BUG-<NNNN>.md                # global bug registry (see bug-registry.md)
├── reports/
│   └── <YYYY-MM-DD>-<scope>.md      # one per run; dated; never overwritten
├── evidence/
│   └── <YYYY-MM-DD>-<scope>/        # screenshots per run; lean (see policy below)
├── automation-backlog.md            # automation intent (see automation-backlog.md)
└── templates/
    ├── bug.md                       # seeded from qa-report/assets/bug-template.md
    ├── charter.md                   # seeded from qa-report/assets/charter-template.md
    └── report.md                    # seeded from qa-execution/assets/report-template.md
```

Both skills prefer the project-local `templates/` copies (projects may customize them); the bundled assets are the fallback and the seed source.

## What is durable vs per-run

| Durable (grows, never reset)                          | Per-run (dated, appended)                    |
| ----------------------------------------------------- | -------------------------------------------- |
| `state.csv` rows and their lifecycle columns          | `reports/<date>-<scope>.md`                  |
| `bugs/BUG-NNNN.md` (ids monotonic forever)            | `evidence/<date>-<scope>/`                   |
| `personas.md`, `journeys/`, `automation-backlog.md`   | charter `debrief` sections                   |
| `charters/` (missions persist; re-run across cycles)  |                                              |

The durable side is the memory that per-round QA trees never had: the next cycle reads it before planning, dedups against it before filing, and updates it instead of duplicating it.

## Bootstrap procedure (new project)

1. Create the directory tree above under `<qa-docs-path>`.
2. Seed `state.csv` from `qa-report/assets/state.csv` (header + delete the example row after reading it).
3. Copy the three templates into `templates/` (bug + charter from `qa-report/assets/`, report from the sibling skill's `qa-execution/assets/report-template.md` when installed).
4. Write `README.md` with: the project's area codes for scenario ids (e.g. `AUTH`, `CHK`, `SET` — 2-4 uppercase letters each, defined once), the product's entry points (URLs, CLI commands), how to start the dev server, and an empty `## Changelog` section.
5. Create empty `bugs/`, `journeys/`, `charters/`, `reports/`, `evidence/` directories (add a `.gitkeep` when the VCS needs it).

## Adoption procedure (project with scattered QA artifacts)

For projects that accumulated per-round `qa/` trees, orphan bug files, or external lab outputs before this layout existed:

1. **Inventory first.** List every location holding QA artifacts (old `qa/` dirs, archived task trees, external labs). Record the inventory in `README.md` under `## Adopted from`.
2. **Migrate durable knowledge only:** journey definitions, still-relevant charters, and open/unresolved bugs. Historical verification reports and evidence stay where they are — indexed by path from `README.md`, never copied.
3. **Re-mint every migrated bug id.** Old per-round `BUG-NN` ids collide across rounds (the same id names different bugs in different rounds). Mint fresh global `BUG-NNNN` ids and record the origin path in each migrated bug's `Origin:` field. Never import old ids as-is.
4. **Seed `state.csv` from the product, not from old test cases.** Derive rows from journey flows (Step 3-4 of the SKILL). Old TC-* files describe checks, not scenarios; mine them for forgotten edge knowledge, then leave them behind.
5. Old trees are then dead weight: recommend archival or deletion to the operator — do not maintain both systems.

## Evidence policy (lean by design)

Living docs stay reviewable. Evidence discipline:

- Screenshots: **checkpoints and failures only** — goal-reached states, divergences, bug reproductions. Not every step.
- Big artifacts (videos, HAR files, multi-MB logs, lab workspaces) never enter the tree. Store them wherever the project keeps build artifacts and **index them by path** from the report.
- If a run's evidence directory exceeds what a reviewer would reasonably diff (rule of thumb: tens of files / a few MB), prune to the failures and note the pruning in the report.
- Projects may gitignore `evidence/` entirely and keep only reports + tracker committed; the report must then reference evidence by absolute path. Record the choice in `README.md`.

## Anti-patterns

- **Parallel trees** — a second tracker, a `qa-v2/` dir, or a per-round `qa/` tree next to the canonical one. One tree; rounds append.
- **Resetting ids** — bug or scenario ids restart at 001 "for the new round". Ids are global and monotonic; the round is recorded in the report, not in the id.
- **Copying evidence into docs** — evidence is indexed, not hoarded. The tree's value is reviewability.
- **Temp-dir fallback** — writing QA output to `/tmp` because no path was given. The default is `docs/qa` in the repo; a missing tree is bootstrapped, not dodged.
- **Bootstrapping without reading existing state** — if the tree exists, plans build on `state.csv` and the bug registry. Planning from scratch on top of an existing tree recreates the duplication problem inside the tree itself.
