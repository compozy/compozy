# Context Pack

How to assemble `<out>/context-pack.md` — the shared context every reviewer and sweep receives. **Keep it lean: target ≤ ~10 KB.** Every agent in the fan-out reads it in full, so each extra kilobyte is paid once per agent; reviewers dig into the code themselves (rg, git, file reads), so the pack carries only what they cannot cheaply rediscover — intent, review law, and what the linters already caught.

## 1. Repository knowledge — discover before extracting

Run the read-only discovery/bootstrap helper after the manifest:

```bash
python3 <skill-dir>/scripts/build_knowledge.py --out <out>
```

It discovers every repository-local root/nested `AGENTS.md` and `CLAUDE.md`, repo review config/learnings, project `SKILL.md` under conventional local skill roots, and direct markdown references of candidate skills. Nested instructions apply to selected paths in their directory subtree; all ancestors remain applicable and deeper sources have higher precedence.

`knowledge.json` records why every source is or is not a candidate. `rules.template.json` starts every candidate as `pending`. Read each pending source **in full**; for a selected skill, read each pending direct reference in full too. Copy the template to `rules.json` and change every pending row to:

- `applied` — the source was read and governs at least one selected path; or
- `not-applicable` — include a concrete reason why it does not govern this change.

`build_jobs.py` rejects missing sources, pending statuses, empty reasons, rule sources not marked applied, and rule scopes that match no selected path.

## 2. Rubric — extract the review law

Extract verdict-bearing rules in precedence order (higher wins on conflict):

1. Path instructions from repo review config.
2. Nested `AGENTS.md` / `CLAUDE.md`, deepest applicable directory first.
3. Root `AGENTS.md` / `CLAUDE.md`.
4. Explicitly dispatched or change-relevant project skills and their required references.
5. `.deep-review/learnings.md` entries whose scope matches selected files.

Extract only rules that can bind a review result (error handling, testing shape, layering, security, naming, documentation, design tokens, framework patterns). Operational commands can leave an applied source with zero rules when the accounting reason says it was read but contains no review law. Register each rule once and keep its text verbatim:

```json
{ "sources": [
    { "source": "AGENTS.md", "kind": "instruction", "status": "applied",
      "reason": "root rules govern every selected path" }
  ],
  "rules": [
    { "id": "R07", "scope": ["**/*_test.go"], "source": "AGENTS.md",
      "guideline": "MUST use t.Run(\"Should...\") pattern for ALL test cases" }
  ]
}
```

`scope` is the path-instruction glob, the instruction file's directory subtree, the selected skill's routed paths, or the learning's scope. To preserve the fan-out budget, the pack lists applied sources/rule counts plus one aggregate not-applicable count; complete per-source decisions stay in rules.json. `build_jobs.py` injects bound rules into defect cohorts, polish cohorts, and sweeps.

## 3. Linter lanes — run first, suppress overlaps

Detect what the repo already enforces and run it scoped to selected files; findings a lane reports are suppressed from the review (taxonomy rule 1).

| Signal in repo | Lane command (scope to changed files where supported) |
| --- | --- |
| `Makefile` with `lint`/`check` target | `make lint` (authoritative when present — prefer it over raw tools) |
| `golangci-lint` config / Go modules | `golangci-lint run <changed dirs>` |
| `package.json` scripts `lint`/`typecheck` | the repo's own script via its package manager |
| eslint/biome/oxlint config | corresponding tool on changed files |
| `tsconfig.json` | `tsc --noEmit` (project-wide; cheap signal) |
| `ruff.toml` / pyproject | `ruff check <files>` |
| `Cargo.toml` | `cargo clippy` |

Record per lane: `ran` (attach findings on selected files, trimmed) or `unavailable` (tool missing/failed — overlap suppression is off for that lane and review.md must say so). Never install tools to fill a lane.

## 4. PR intent

With `--pr`: title, description, linked issues (`gh pr view N --json title,body,closingIssuesReferences`), and base/head. Locally: `git log --oneline <base>..<head>` plus the user's stated intent. Reviewers judge the diff against *stated intent* — a change that does more than its description says is itself a finding.

## 5. Spec contract (`--spec`)

Resolve the conformance baseline: a file path is itself the artifact; a directory contributes its contract-bearing documents — `_prd.md`, `_techspec.md`, `_tests.md`, `_examples.md`, `_qa.md`, `_user_stories.md`, parity maps, requirement/UX docs, plus any document the spec's own files name as canonical. List every resolved artifact as `path → one-line role`. These are the baseline the `spec-parity` sweep judges against — do NOT extract rubric rules from them: §1 sources are review law, the spec is the contract under test.

## 6. context-pack.md layout

```markdown
# Context Pack — <target>

## Intent
<title/description/commits digest>

## Rubric
<applied source: path → rule count; N other sources classified not-applicable in rules.json; canonical forms: knowledge.json + rules.json>

## Linters
<lane → ran(findings digest) | unavailable(reason)>

## Spec contract
<only with --spec: one `- `path`` line per artifact — render_review.py parses these lines for the conformance table>
```
