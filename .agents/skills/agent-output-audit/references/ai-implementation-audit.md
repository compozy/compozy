# AI Implementation Audit

This reference catalogs the **observable signals** `agent-output-audit` uses when auditing test code produced by an AI agent. It defines what to scan for, how to classify findings, and how to record them. It does **not** explain *why* each pattern is wrong or *how* to fix it — that belongs to the `testing-boss` skill's pattern and anti-pattern catalogs.

## Contents

- When to apply
- Red Flag Scanners (RF-1..RF-6)
- Requirement → Test Mapping
- Verdict Matrix
- Recording findings
- Sources

## When to apply

- Any task with `declared_status: completed` whose implementation includes new or modified test files.
- Any commit that touches both production code and its sibling test in the same change set.
- Any `cy-codex-loop` Compozy slug under `.compozy/tasks/<slug>/` where the implementing agent self-reported success.

## Red Flag Scanners

For each red flag, run the listed scan against the test diff since the task baseline. Use `git log --follow <test_file>` to recover the baseline. Emit the listed verdict when the flag fires.

### RF-1 Skipped or disabled tests added

```bash
git diff <baseline_sha>..HEAD -- '*test*' '*spec*' \
  | rg -nP '^\+.*(\.skip\(|\.only\(|xit\(|xdescribe\(|t\.Skip\(|@pytest\.mark\.skip|@Ignore|fdescribe|fit\()'
```

**Verdict:** `FAIL`. File `BUG-<num>.md` with Type `Functional` and Status `pending`.

### RF-2 Weakened assertions

Detect replacements from strict equality (`toBe`, `toEqual`, `toStrictEqual`) to permissive matchers in the same commit that flipped `status: completed`.

```bash
git diff <baseline_sha>..HEAD -- '*test*' '*spec*' \
  | rg -nP '^-.*(toBe|toEqual|toStrictEqual)\(' \
  | rg -nP '^\+.*(toBeDefined|toBeTruthy|toBeFalsy|toBeNull|toBeUndefined|toMatch\b|expect\.anything|expect\.any\b|assert\.NotNil|require\.NotEmpty)'
```

Also flag changes from numeric/string equality to `.toContain(...)` or `.toMatchObject(...)` with only one expected field.

**Verdict:** `FAIL` when the weakened assertion covers a P0/P1 Success Criterion. `PARTIAL` when it covers only edge case checks. Always require a `BUG-<num>.md` naming the original strict assertion in Root cause.

### RF-3 Mocks inserted in tests classified as Integration or E2E

The TC declares `Automation Target: Integration` or `E2E` and lists an `External Dependencies` set — but the test file mocks one of those dependencies.

```bash
rg -nP '(jest\.mock\(|vi\.mock\(|nock\(|gomock\.|httpmock|patch\.object\(|patch\(.*Mock|sinon\.stub\(|MockBean\b)' <test_file>
```

Cross-reference each match against the TC `External Dependencies` list. Any mock targeting a dependency that the TC declared real is a violation.

**Verdict:** `FAIL`. Tag the BUG with `mock-hides-integration`. The fix must either remove the mock (preferred) or downgrade the TC to `Automation Target: Manual-only` with a documented reason.

### RF-4 Snapshot or gold-file drift

```bash
git diff --name-only <baseline_sha>..HEAD \
  | rg -nP '(__snapshots__/.*\.snap$|testdata/golden/|/__fixtures__/|\.golden$|/fixtures/.*\.(json|yaml|yml)$)'
```

When any path matches, open each file and verify the change is justified by an explicit requirement. A snapshot updated without a corresponding requirement change is drift.

**Verdict:** `FAIL` when the snapshot covers a P0/P1 Success Criterion. `PARTIAL` elsewhere.

### RF-5 Happy-path-only coverage

The TC is P0/P1 and the implementation has only positive-path assertions: no failure row in `it.each`/`test.each`, no `expect(...).toThrow`, no 4xx/5xx assertion, no empty/null/undefined input, no permission-denied case.

**Verdict:** `PARTIAL`. File `BUG-<num>.md` requesting the missing negative paths. Do not REOPEN unless `External Dependencies` make negative paths trivial.

### RF-6 Test-implementation symbiosis

```bash
git log --oneline --name-only <baseline_sha>..HEAD \
  | awk '/^[a-f0-9]/ {commit=$0; next} {print commit, $0}' \
  | rg -nP '(\.test\.|\.spec\.|_test\.go|/test_)' \
  | sort -u
```

Group by commit and flag commits where both implementation and test sibling appear together without a third commit message that names the requirement. Then apply the Requirement → Test Mapping below.

**Verdict:** `PARTIAL` until the mapping below resolves to `covers` for every criterion. Otherwise `FAIL`.

## Requirement → Test Mapping

For every Success Criterion in `task_NN.md` (frontmatter or body) and every linked bullet in `_techspec.md`, build the table:

| criterion | matched test | assertion verdict |
|---|---|---|
| `<verbatim criterion text>` | `<test file:line or "none">` | `covers` / `weak` / `missing` |

**Verdict definitions:**

- `covers` — A specific assertion in the matched test references the literal value, behavior, or contract the criterion describes. The assertion is strict (equality, status code, error type, exact text).
- `weak` — A test exists in the criterion area but uses a permissive matcher, checks the wrong layer (internal state instead of public outcome), or only checks the happy path of a multi-path criterion.
- `missing` — No test references the criterion area, or every candidate test is `.skip`/`.only`-fenced, mocked away, or asserts unrelated state.

A `weak` row blocks `PASS` on a P0/P1 task. A `missing` row blocks `PASS` on any task. Record the table in the Task Implementation Matrix column `ai_audit_findings` and in `audit-report.md` under the per-task block.

## Verdict Matrix

| Red flag fired | Task verdict | Required action |
|---|---|---|
| RF-1 Skip/disable | `FAIL` | REOPEN frontmatter + BUG (`Type: Functional`) |
| RF-2 Weakened on P0/P1 criterion | `FAIL` | REOPEN + BUG, name the original assertion in Root cause |
| RF-2 Weakened on edge case only | `PARTIAL` | BUG, do not REOPEN unless P0 |
| RF-3 Mock hiding integration | `FAIL` | REOPEN + BUG (tag `mock-hides-integration`) |
| RF-4 Snapshot drift on P0/P1 | `FAIL` | REOPEN + BUG, require requirement-change justification |
| RF-4 Snapshot drift elsewhere | `PARTIAL` | BUG, defer to maintainer |
| RF-5 Happy-path-only on P0/P1 | `PARTIAL` | BUG requesting negative paths |
| RF-6 Symbiosis + `weak`/`missing` row | `FAIL` | REOPEN + BUG |
| RF-6 Symbiosis + all `covers` rows | `PASS` | Note in audit log; no action |

When multiple flags fire on the same task, take the strictest verdict.

## Recording findings

Record findings in three places:

1. `audit-report.md` → `TASK IMPLEMENTATION AUDIT` block → per-task `AI audit findings:` field (list red flag IDs that fired with their verdicts).
2. `audit-report.md` → `SUITE HEALTH SNAPSHOT` → `AI audit findings:` count.
3. Compozy mode only: `.compozy/tasks/<slug>/memory/qa-execution.md` → `Errors / Corrections` section, **before** any frontmatter status flip (memory-precedes-status invariant).

## Sources

- Anthropic — [Demystifying Evals for AI Agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents): independent evaluator principle.
- Florian Bruniaux — [Claude Code Ultimate Guide: TDD with Claude](https://github.com/FlorianBruniaux/claude-code-ultimate-guide/blob/main/guide/workflows/tdd-with-claude.md): Verification Gap.
- Autonoma — [Vibe Coding Best Practices: The Testing Checklist](https://getautonoma.com/blog/vibe-coding-best-practices): Stanford/UIUC finding on vulnerability rates with AI-assistant trust.
