# State and Learnings

The tracker that makes rounds incremental and the feedback loop that stops repeated mistakes.

## Fingerprint — a result's identity

```
fp = first 16 hex of sha256("<result-kind>|<file>|<category>|<normalized title>")
```

`normalized title` = lowercase, alphanumerics and single spaces only. Line numbers are deliberately excluded — anchors drift between pushes; identity must survive that. The single implementation is `fingerprint()` in `scripts/_common.py` (applied by merge_findings.py); compute one by hand only when recovering fingerprints from a PR thread:

```bash
printf '%s' "defect|internal/store/queue.go|potential-issue|dont hard fail preferredmodel when config options are unrelated" \
  | shasum -a 256 | cut -c1-16
```

## state.json (per target, in `<out>/`)

```json
{
  "target": "pr:312",
  "rounds": [{ "n": 2, "base": "<sha>", "head": "<sha>", "verdict": "FIX_BEFORE_SHIP", "reviewed_at": "<ISO-8601>" }],
  "ledger": {
    "<fp>": { "file": "...", "title": "...", "severity": "major", "result_kind": "defect", "status": "open",
              "round": 1, "comment_id": 123456, "resolved_in": null }
  }
}
```

`status`: `open` → `resolved` (fix observed) | `dismissed` (user rejected — capture a learning; never re-raised). `comment_id` only when published.

## Round reconciliation (Step 4)

Implemented by merge_findings.py (round status) and render_review.py (ledger update); this is the rule they follow. For each defect/advisory this round, compute `fp` and look it up:

- **absent** (or previously `resolved`) → `new`; add to ledger as `open`.
- **present, `open`** → `duplicate`; render once in the Duplicates section, keep ledger row.
- **present, `dismissed`** → keep suppressed from the active results and expose it in the dismissed/suppression audit trail.

Then sweep the ledger's `open` rows *not* re-found this round: if the row's file was re-reviewed (selected) or left the diff entirely, mark `resolved` (`resolved_in` = head; publish mode adds the ✅ edit); if the file sits in the manifest un-re-reviewed (carried/skipped), keep `open` and list it under Duplicates.

## learnings.md (at `.deep-review/learnings.md`, repo-committable)

Append-only entries, one per correction:

```markdown
## <fp-or-slug> — <one-line rule>
- Scope: <glob> (e.g. internal/store/**)
- Rule: <the distilled instruction a future reviewer must follow, imperative>
- Why: <the rationale the user gave — the invariant, the design intent>
- Origin: <pr:312 | session, date>
```

**Capture triggers:** the user (or a PR reply from the author) rebuts a finding with a reason; the user says a class of findings is unwanted; a dismissal reveals a repo convention the rubric missed. Distill the *rule*, not the anecdote — "ReserveQueuedRun is the authoritative one-open-run enforcer; do not flag missing pre-checks in enqueue paths" beats a story about one PR.

**Application:** learnings are rubric input (context-pack.md §2) for every later round, scoped by their glob. Path instructions outrank learnings on conflict; a learning that contradicts project instructions or a selected skill signals that doctrine needs editing — surface the conflict.

## Storage conventions

- `<out>` holds manifest.json, knowledge.json, rules.json, context-pack.md, plan.json, prompts/, jobs.json, agents/, runs/, walkthrough.md, findings.json, review-stats.json, review.md, review.html, state.json, and round.json.
- When build_manifest.py starts a new round it archives everything except state.json/round.json/rounds/ into `<out>/rounds/round-<n>/` — the per-round audit trail; only state.json carries memory forward.
- `.deep-review/learnings.md` is shared across targets and worth committing — it is team review doctrine.
- Recommend adding `.deep-review/` to `.gitignore` with `!.deep-review/learnings.md` — suggest it once when the directory is first created; the decision belongs to the user.
