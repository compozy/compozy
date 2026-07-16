# Audit Method

Use this method to resolve a target, explore module depth, apply the deletion test, honor durable context, and produce a deterministic ranking. Keep the unit of analysis at module-interface scale.

## 1. Resolve the target before scanning

1. Canonicalize the workspace root and the optional positional target without following it outside the workspace.
2. When no target was supplied, ask for exactly one of: a module path, a feature-area path, or the whole project. Perform no source scan before the answer.
3. When the target does not exist, print `target not found: <target>` and stop without creating or changing audit artifacts.
4. When the target is a file, resolve its enclosing module directory, state `Auditing enclosing module <path> for file target <file>`, and use the module directory for the area and slug.
5. Express the canonical area as a workspace-relative path with `/` separators. Use the workspace directory name for an explicitly chosen workspace-root target.

### Source scope

Discover the repository's languages from manifests, extensions, and existing source layout instead of using a TypeScript- or Go-only glob. Count source files after excluding generated output, vendored dependencies, VCS metadata, caches, build directories, and prior `.compozy/arch-reviews/` artifacts.

- Zero source files: print `nothing to audit: <area>` and stop without a report or depth-map change.
- Up to approximately 50 source files: inspect the whole target.
- More than approximately 50 source files: offer either a narrower module or a deterministic sampled pass before exploration.
- When the user insists on a very large target, bound the pass. Sort paths lexically, always include entry points and public interfaces, add direct callers/tests for suspected modules, select the remaining sample deterministically across the sorted path list, and cap the reported candidate set. State the sampled scope in both reports and the run summary.

Do not silently fall back from an invalid or empty target to the whole repository.

## 2. Derive a deterministic slug and detect collisions

Derive `<slug>` from the canonical target path:

1. Trim surrounding whitespace and trailing separators.
2. Lowercase the value.
3. Replace every run of non-alphanumeric characters with `-`.
4. Collapse repeated `-` and trim leading/trailing `-`.
5. If an explicit workspace-root target reduces to empty or `.`, use the normalized workspace directory name.

Show `Target: <canonical-area> -> slug: <slug>` before any artifact replacement. The same target must always produce `.compozy/arch-reviews/<slug>.{html,md}`.

Write the canonical target and slug into report metadata. When existing artifacts for the slug name the same canonical target, treat the run as a deterministic re-audit. When they name a different target, or cannot prove their target, warn that two targets normalize to the same slug and request overwrite confirmation before publishing. A declined or unanswered collision writes nothing.

## 3. Read prior architecture memory

Read the audited area's active `avoid` entries before generating candidates. Normalize comparison keys from the proposed module, deepening move, and affected seam; suppress a matching proposal or label it `Previously declined` only when new evidence must be shown. Preserve its original load-bearing reason.

Treat superseded `#` provenance as history rather than an active constraint.

## 4. Read settled decisions best-effort

When `.compozy/DECISIONS.md` exists:

1. Skip blank lines and comment lines.
2. Split data lines on `|`, trim fields, and accept only the companion's six-field proven-record shape.
3. Soft-warn once for malformed rows and continue with valid rows.
4. Resolve supersession chains and use only each active tail; never let an obsolete decision suppress a candidate.
5. Match candidate modules, seams, intent, and affected paths against active settled decisions.

Suppress architecture already covered and shipped. Surface a conflicting candidate only when current code provides concrete friction that warrants reopening the decision; add a warning naming the decision and the reopening evidence. Never alter the decision index.

An absent or documented-empty index means there is nothing to suppress. An unreadable index disables only settled-decision filtering.

## 5. Walk the target organically

Give a read-only Explore agent the canonical target, source boundaries, active avoidances, active settled decisions, and the bundled `cy-codebase-design` vocabulary. If no sub-agent facility exists, perform the same walk directly.

Trace concepts rather than file categories:

- Start at entry points and public module interfaces.
- Follow representative callers through the implementation and tests.
- Note concepts that require bouncing through many small interfaces.
- Note caller knowledge, invariants, ordering, error modes, configuration, and performance facts leaking through a seam.
- Find pure helpers extracted only for testability while production bugs remain in orchestration.
- Find duplicated caller choreography that a deeper module could absorb.
- Find seams with two real adapters and distinguish them from hypothetical single-adapter indirection.
- Inspect tests to determine whether they exercise a module interface or reach past it.

Do not emit generic dead-code, style, long-method, or line-level refactoring findings. Do not flag a Go adapter, functional-options constructor, TypeScript re-export, framework entry point, generated facade, or similar idiomatic thin module solely because it has little implementation.

## 6. Apply the deletion test

For every suspected module, imagine deleting its interface and implementation, then trace where its required knowledge and behavior go.

Record one of these verdicts:

- `Concentrates complexity — keep and deepen`: deletion recreates or disperses invariants, caller knowledge, orchestration, or test setup across multiple callers. The module is a credible home for more hidden implementation, but its current interface is too costly.
- `Merely moves complexity — skip`: deletion relocates pass-through code without losing a useful concentration point. Do not report it as a deepening candidate.
- `Uncertain — investigate`: evidence cannot distinguish the outcomes. Retain only when the uncertainty itself is useful, badge it `Speculative`, and state what evidence is missing.

Every reported candidate must quote or paraphrase its verdict and identify the observed callers/tests that support it. A confident verdict is required for `Strong` or `Worth exploring`.

## 7. Form module candidates

Each retained record contains:

- stable candidate ID and module name;
- current interface burden and shallow-depth problem;
- concrete deepening move and intended smaller interface;
- affected files/modules and representative callers/tests;
- dependency category from bundled `cy-codebase-design`: `in-process`, `local-substitutable`, `ports & adapters`, or `mock`;
- deletion-test verdict and evidence;
- expected locality, leverage, and test-surface gains;
- maintainability-cost evidence such as repeated change sites, drift, bug exposure, or setup breadth;
- strength: `Strong`, `Worth exploring`, or `Speculative`;
- optional settled-decision or prior-avoidance callout;
- before/after structural model.

Name the module, not a line, method, class fragment, or desired implementation detail.

### Overlap reconciliation

When two findings affect the same files or caller choreography, test whether one deeper module would resolve both. Merge them when they share the same seam and deepening move. Otherwise keep distinct module records and cross-reference their candidate IDs, explaining the shared blast radius. Never count the same fix-value twice.

## 8. Rank by fix-value

Order candidates by this tuple:

1. confident deletion-test concentration before uncertainty;
2. greater observed maintainability cost or bug risk;
3. greater leverage across callers and tests;
4. greater locality gain and interface reduction;
5. stronger evidence and lower implementation uncertainty;
6. smaller blast radius when the preceding factors tie;
7. lexical module path as the final stable tie-break.

Use evidence, not invented numeric precision. Explain the top pick in terms of current cost, the depth problem, and the specific deepening move. When no credible candidate survives, report a healthy target with zero candidates.

## 9. Reconcile the depth-map section safely

Treat each `## <area> | ...` block as a keyed raw-byte span.

1. Read the latest complete map and retain its original bytes.
2. Locate the canonical area key. Preserve all non-target spans exactly.
3. Carry active `avoid` lines and superseded provenance from the target span into the regenerated span unless the current run explicitly supersedes one.
4. Preserve unrecognized hand-written comments inside the target span. If off-grammar content cannot be preserved while producing a valid map, surface a conflict and leave the map unchanged.
5. Insert a new area in lexical order. For an existing-area re-audit, splice the replacement at the same raw span so every other byte stays unchanged.
6. For a confirmed rename, remove the old key, transfer its avoid/provenance history, add `# moved <YYYY-MM-DD>: <old-area> -> <new-area>` to the new span, and insert the new key in lexical order. Without a reliable match or confirmation, retain the old section and warn instead of guessing.
7. Write a temporary sibling, validate the entire staged map against `architecture-map-format.md`, then compare the current source bytes with the bytes originally read. If they changed, re-read and reapply the keyed replacement once.
8. Atomically replace the map only after validation. Ignore stale temporary files on the next run and rebuild from the latest complete map.

Keep the mandatory `deep`, then `seam`, then `avoid` grouping. Within the `deep` group and within the `seam` group, order generated entries lexically by target for stable diffs; then append active `avoid` entries. Keep all report paths workspace-relative.
