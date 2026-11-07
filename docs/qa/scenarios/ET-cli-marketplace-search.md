---
id: ET-cli-marketplace-search
area: ET
title: Search the marketplace through structured CLI output
persona: Ada
journey: J-agent-marketplace-parity
expected: `compozy marketplace search` returns the canonical catalog envelope with truthful installed state, revision and pagination; items carry category, category_label and featured. `--category <id>` lists one category and binds its cursor; an unknown id exits 2 with marketplace_category_unknown and the valid ids. `compozy marketplace categories` lists the taxonomy with counts. JSON matches the daemon response; JSONL, human and TOON retain page metadata. Retired --kind flags and two-argument info fail before transport.
entry_points: compozy marketplace search [query] [--category <id>] [--cursor <opaque>] -o json|jsonl|toon; compozy marketplace categories [query] -o json; compozy marketplace info <entry_id> [--source <name>]; compozy marketplace refresh
qa_status: fail
bug_ids: BUG-20260715-native-marketplace-extension-parity; BUG-20260715-marketplace-stale-report; BUG-20260729-marketplace-json-parity; BUG-20260729-marketplace-file-cursor-fence; BUG-20261107-marketplace-empty-version-500
fix_status: open
retest_status: pending
fix_commits: 8eeb8a38;351f3535
evidence: /tmp/mpc-shots/s8 (cli-marketplace-categories.txt, cli-search-category.txt, native-marketplace-*.json)
last_report: docs/qa/reports/2026-09-13-marketplace-catalog.md
overlaps: ET-007; ET-016
---

QA 2026-09-13: the current hard-cut contract passed the scoped live/API/browser walks and applicable unchanged owning integration checks. See the dated report for exact evidence and boundaries; historical notes below do not redefine the current catalog.


Current contract (2026-09-13): compare CLI canonical browse/detail with HTTP and UDS for the same source and installed-state scope. Continue without a kind selector, verify revision/cursor fencing and source identity, and verify old flags/arity cannot query or mutate. Final tasks09/10 own this walk. The historical grouped-kind evidence below does not validate the amended contract.

## Historical evidence

Historical QA note: opaque cursor continuation through structured CLI output remains pending.

Added by marketplace Task 02 after the hard cut to one discovery namespace. The next agent-surface QA cycle should compare CLI JSON byte semantics with HTTP and UDS for the same daemon state, including one isolated kind failure.

QA impact 2026-07-18: `--cursor` now continues a single `--kind` from `next_cursor`; grouped search
rejects it. Compare both pages with HTTP/UDS and prove the cursor remains bound to scope/workspace.

QA impact 2026-07-18: grouped JSON omits continuation metadata, while a single-kind continuation
rejects a cursor when its catalog projection changes. Confirm the structured error tells the
operator to restart from the first page.

QA impact 2026-07-18: single-kind human and TOON output append a Page block, and JSONL appends one
`type: "page"` record after the rows. Verify every format exposes `next_cursor` and available total,
stale, and diagnostic metadata without changing the JSON envelope.

QA result 2026-07-29: CLI JSON added a transport-only resolution field, and unchanged local
catalog refetches invalidated continuation cursors. Both canonical regressions, staged root fixes,
and the rebuilt CLI/HTTP/UDS replay are green; the scenario remains failed until the fixes have
governed commits.

QA impact 2026-08-02: the marketplace kind set is now exactly MCP, Extension, and Skill. Reset to
prove all structured modes and deterministic rejection of the retired kind.

## QA impact 2026-11-07 — marketplace plugin catalog (spec `marketplace-plugin-catalog`, S8)

New agent surfaces (US-014; `_dx.md` CLI):

1. `compozy marketplace categories` (human) lists Featured (when > 0) and non-empty categories
   with counts, omitting zero-count rows;
   `-o json` returns `{revision, categories[]}` including `featured` and zero counts. Counts match
   `GET /api/marketplace/sections` totals for the same scope. Capture both outputs.
2. `compozy marketplace search --category productivity --limit 3 -o json`: every item has
   `category: productivity`; continue with `--cursor`; reusing that cursor with another category
   fails with the stale-cursor error.
3. `compozy marketplace search --category apps`: exit status 2, message names
   `marketplace_category_unknown` and lists the valid ids; no transport mutation.
4. Native parity: `compozy__marketplace_categories` and `compozy__marketplace_search {category}`
   return the same counts and items.

### Walk result 2026-11-07 (S8 integrated lab, `feat/mpc-s8` with S1–S7 + S9)

Isolated lab (own home, ports 49760/49761, worktree web dev server), default sources plus a local `lab-plugins` folder source; cold refresh of the default sources took ~65.7 s (compozy-catalog 18, claude-plugins-official 316, compozy-plugins 333). Screenshots and CLI captures are lab-local in `/tmp/mpc-shots/s8/` (not committed).

- PASS — `compozy marketplace categories`: Featured 6 plus the twelve categories, zero rows omitted in
  human output; JSON keeps every id.
- PASS — `search --category apps` exits 2 with `marketplace_category_unknown` and the valid ids;
  `--category data -o json` items carry `category`, `category_label`, `featured`; a `more` cursor
  reused with `--category design` is rejected ("Catalog changed; restart from the first page").
- PASS — HTTP `per_section=13` → 400 `invalid_request`; `?category=apps` → 400
  `marketplace_category_unknown`.
- PASS — native `compozy__marketplace_categories` and `compozy__marketplace_search {category}` via
  `compozy tool invoke` match the CLI.
- **FAIL** — with the real catalog, `search --category more --limit 100` errors with
  `curated marketplace version is required for "data-engineering"` (BUG-20261107-marketplace-empty-version-500).
