---
name: ubs
description: >-
  Run Ultimate Bug Scanner (UBS) for code review. Use when reviewing code,
  checking for bugs, scanning for security issues, validating AI-generated
  code, or pre-commit quality checks.
---

<!-- TOC: Core Insight | THE EXACT PROMPT | Quick Reference | When to Use | Critical Rules | Suppression | Triage | Troubleshooting | AI Validation | References -->

# Using UBS for Code Review

> **Core Insight:** UBS catches what compiles but crashes — null derefs, missing await, resource leaks, security holes. It has many false positives; triage is essential, not optional.

## The Golden Rule

```
ubs <changed-files> before every commit.
Exit 0 = safe to proceed.
Exit 1 = triage findings.
Exit 2 = run `ubs doctor --fix`.
```

---

## THE EXACT PROMPT — Fix-Verify Loop

```
1. Scan: Run UBS on changed files
   ubs --staged                    # Staged files (<1s)
   ubs --diff                      # Unstaged changes vs HEAD
   ubs file.ts file2.py            # Specific files

2. Triage each finding:
   Real bug?        → Fix root cause (not symptom)
   False positive?  → // ubs:ignore — [why it's safe]

3. Re-run until exit 0
   ubs --staged

4. Commit when clean
```

### Why This Workflow Works

- **`--staged` is fast** — Scans only what you're committing
- **Fix root cause** — Masking symptoms creates debt
- **Exit 0 gate** — Clean scan = confidence to commit
- **Justification required** — Every `ubs:ignore` must explain why

---

## Quick Reference

```bash
# Core workflow
ubs --staged                       # Staged files only (<1s)
ubs --diff                         # Working tree changes vs HEAD
ubs .                              # Full project scan

# Language-specific (--only=js excludes TS!)
ubs --only=go,rust src/            # Go and Rust only
ubs --only=ts,tsx frontend/        # TypeScript (js≠ts)

# Noise reduction
ubs --skip=11,12 .                 # Skip TODO/debug categories
ubs --profile=loose .              # Skip minor nits

# Output formats (json=summary, jsonl=per-finding details)
ubs . --format=jsonl                          # Per-finding details
ubs . --format=sarif > results.sarif          # IDE/GitHub integration

# PR review (new issues only)
ubs . --comparison=baseline.json --fail-on-warning

# Troubleshooting
ubs doctor                         # Check environment
ubs doctor --fix                   # Auto-fix issues
```

---

## When to Use What

| You Want | Command | Why |
|----------|---------|-----|
| Quick pre-commit | `ubs --staged` | Fast, only staged files |
| Strict gate | `--fail-on-warning` | Blocks on all findings |
| Skip noise | `--skip=11,12` | TODO/debug categories |
| Language focus | `--only=go,py` | Target specific languages |
| PR review | `--comparison=baseline.json` | Shows NEW issues only |
| Security audit | `--category=security` | Focused security scan |
| Full report | `--html-report=out.html` | Shareable dashboard |
| Per-finding data | `--format=jsonl` | Detailed parsing (json=summary only) |
| Environment fix | `ubs doctor --fix` | First-line troubleshooting |

---

## Critical Rules

| Rule | Why | Consequence |
|------|-----|-------------|
| **Exit 2 → doctor** | Scanner error | Run `ubs doctor --fix` immediately |
| **Every ignore needs why** | Audit trail | `// ubs:ignore — caller validates` |
| **Fix root cause** | Prevents debt | Don't mask symptoms |
| **Don't skip triage** | Real bugs hide | Review every finding |
| **JS/TS needs AST engine** | Semantic analysis | `ubs doctor --fix` if degraded |

---

## Suppression

```javascript
// GOOD — explains why it's safe
eval(trustedConfig);  // ubs:ignore — internal config, not user input

// BAD — no justification
eval(config);  // ubs:ignore
```

### Per-Language Comment Styles

| Language | Suppression Format |
|----------|-------------------|
| JS/TS/Go/Rust/Java | `// ubs:ignore — reason` |
| Python/Ruby/Shell | `# ubs:ignore — reason` |
| SQL | `-- ubs:ignore — reason` |

**Rule:** Every `ubs:ignore` MUST explain why the code is actually safe.

---

## Is This Finding Real?

```
Finding → Code path executes? → No → FALSE POSITIVE (dead code)
                             → Yes ↓
         Guard clause exists? → Yes → FALSE POSITIVE (ubs:ignore)
                             → No ↓
         Validated elsewhere? → Yes → FALSE POSITIVE (cross-file)
                             → No → REAL BUG, fix it
```

---

## Triage by Severity

| Blocks Commit | Blocks PR | Discuss in PR |
|---------------|-----------|---------------|
| Null safety (1) | Error swallowing (8) | Debug code (11) |
| Security (2) | Division by zero (6) | TODO markers (12) |
| Missing await (3) | Promise no catch (9) | TypeScript `any` (13) |
| Resource leaks (4) | Array mutation (10) | Deep nesting (14) |

**Category numbers** map to `--skip=N` and `--category=N` flags.

**Full breakdown:** [TRIAGE.md](references/TRIAGE.md)

---

## Troubleshooting

| Problem | Cause | Fix |
|---------|-------|-----|
| Exit code 2 | Missing optional scanners | `ubs doctor --fix` |
| JS/TS degraded | AST engine missing | `ubs doctor --fix` |
| Too many findings | Legacy code | Use `--comparison` for baseline |
| Too slow | Full scan | Use `--staged` or `--only=` |
| False positive storm | Test fixtures | Add to `.ubsignore` |

---

## AI Code Validation

AI-generated code is prone to:

| Pattern | Bug | Category |
|---------|-----|----------|
| `user.profile.name` | No null check | 1 (Null safety) |
| `fetch(url)` | Missing await | 3 (Async) |
| `open(file)` | Never closed | 4 (Resource) |
| `catch (e) {}` | Swallowed error | 8 (Error handling) |

```bash
# After AI writes code:
ubs [file] --fail-on-warning
```

---

## References

| Need | Document |
|------|----------|
| Prioritize findings | [TRIAGE.md](references/TRIAGE.md) |
| Identify false positives | [FALSE-POSITIVES.md](references/FALSE-POSITIVES.md) |
| CI/CD, hooks, workflows | [WORKFLOWS.md](references/WORKFLOWS.md) |
