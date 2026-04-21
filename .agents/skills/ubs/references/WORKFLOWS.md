# UBS Workflows

## Quick Reference

| Context | Command | When |
|---------|---------|------|
| Active coding | `ubs --staged` | Every 10-15 min |
| Pre-commit | `ubs --staged --fail-on-warning` | Before `git commit` |
| PR review | `ubs . --comparison baseline.json` | Before merge |
| CI pipeline | `ubs . --format=sarif` | On push |
| Security audit | `ubs --category=security .` | Periodic |
| Codebase health | `ubs . --html-report=report.html` | Weekly |

---

## 1. Active Development

```bash
# After each logical unit of work
ubs --staged

# Or specific files
ubs src/api/users.ts src/utils/auth.ts
```

**If findings:** Fix critical immediately, note others for later. Don't accumulate.

---

## 2. Pre-Commit Gate

### Manual

```bash
ubs --staged --fail-on-warning || echo "Fix before committing"
```

### Automated Hook

```bash
# .git/hooks/pre-commit
#!/bin/bash
set -e
echo "Running UBS..."
if ! ubs --staged --fail-on-warning; then
    echo "Fix issues or add ubs:ignore with justification"
    exit 1
fi
```

```bash
chmod +x .git/hooks/pre-commit
```

### Emergency Bypass

```bash
# ONLY for verified false positives you can't address now
git commit --no-verify -m "Emergency fix (UBS FP, will address)"
```

---

## 3. PR Review

### Basic

```bash
ubs . --fail-on-warning
ubs . --html-report=pr-review.html  # Shareable
```

### Regression Detection (Recommended)

Only fail on NEW issues:

```bash
# On main: capture baseline
ubs . --report-json=.ubs/baseline.json

# On PR branch: compare
ubs . --comparison=.ubs/baseline.json --fail-on-warning
```

### PR Review Checklist

```markdown
## Code Review Checklist

- [ ] UBS scan passes: `ubs . --comparison=baseline.json --fail-on-warning`
- [ ] No new CRITICAL findings
- [ ] Any ubs:ignore additions have justification comments
- [ ] No security category findings
```

---

## 4. CI/CD

### GitHub Actions

```yaml
# .github/workflows/quality.yml
name: Code Quality
on: [push, pull_request]

jobs:
  ubs-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install UBS
        run: |
          curl -fsSL "https://raw.githubusercontent.com/Dicklesworthstone/ultimate_bug_scanner/master/install.sh" | bash -s -- --easy-mode
          echo "$HOME/.local/bin" >> $GITHUB_PATH

      - name: Run UBS
        run: ubs . --fail-on-warning --format=sarif > results.sarif

      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif
```

### With Baseline Comparison

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0

- name: Get baseline
  run: git checkout origin/main -- .ubs/baseline.json 2>/dev/null || echo '{}' > .ubs/baseline.json

- name: Run with comparison
  run: ubs . --comparison=.ubs/baseline.json --fail-on-warning
```

### GitLab CI

```yaml
ubs-scan:
  stage: test
  script:
    - curl -fsSL "https://..." | bash -s -- --easy-mode
    - ubs . --fail-on-warning --format=json > ubs-results.json
  artifacts:
    reports:
      codequality: ubs-results.json
```

---

## 5. AI Agent Validation

### Claude Code Hook

```bash
# .claude/hooks/post-tool.sh
#!/bin/bash
FILE_PATH="$1"
if [[ "$FILE_PATH" =~ \.(js|ts|py|go|rs|java)$ ]]; then
    ubs "$FILE_PATH" --fail-on-warning 2>&1 | head -20
fi
```

### Common AI Bugs

| Pattern | Bug | Category |
|---------|-----|----------|
| `user.profile.name` | No null check | Null safety |
| `fetch(url)` | Missing await | Async |
| `open(file)` | Never closed | Resource |
| `catch (e) {}` | Swallowed | Error handling |

---

## 6. Security Audit

```bash
ubs --category=security .
ubs --category=security . --html-report=security-audit.html
```

### Checklist

- [ ] Review all innerHTML/dangerouslySetInnerHTML
- [ ] Review all eval/exec usage
- [ ] Review SQL query construction
- [ ] Check for hardcoded secrets
- [ ] Verify input validation at boundaries

---

## 7. Codebase Health Dashboard

Track quality trends over time.

### Generate Reports

```bash
# Full HTML dashboard
ubs . --html-report=reports/ubs-$(date +%Y%m%d).html

# JSON for tracking
ubs . --report-json=reports/ubs-$(date +%Y%m%d).json
```

### Track Trends

```bash
# Compare today vs last week
ubs . --comparison=reports/ubs-20250109.json --report-json=reports/ubs-20250116.json

# Summary of changes
jq '.summary' reports/ubs-20250116.json
```

### Scheduled Health Check

```yaml
# .github/workflows/health.yml
name: Weekly Health Check

on:
  schedule:
    - cron: '0 9 * * 1'  # Monday 9am

jobs:
  health:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run health scan
        run: ubs . --html-report=health-report.html
      - name: Upload report
        uses: actions/upload-artifact@v4
        with:
          name: health-report
          path: health-report.html
```

---

## 8. Legacy Codebase Cleanup

### Phase 1: Baseline

```bash
ubs . --report-json=.ubs/baseline.json
git add .ubs/baseline.json && git commit -m "Add UBS baseline"
```

### Phase 2: Prevent New Issues

```bash
# CI: only fail on NEW
ubs . --comparison=.ubs/baseline.json --fail-on-warning
```

### Phase 3: Incremental Cleanup

```bash
# See breakdown by category
ubs . --format=json | jq '.findings | group_by(.category) | map({category: .[0].category, count: length})'

# Fix one category at a time
ubs --category=resource-lifecycle .

# Update baseline after cleanup
ubs . --report-json=.ubs/baseline.json
```

---

## Troubleshooting

### "Too many findings"

```bash
# Priority order:
ubs . --format=json | jq '.findings[] | select(.severity == "critical")' | head -20
ubs . --format=json | jq '.findings[] | select(.category == "security")'
ubs . --format=json | jq '.findings[] | select(.category == "async")'
```

### "Scan too slow"

```bash
ubs --staged                    # Only changed
ubs src/api/                    # Only directory
ubs --only=js,python .          # Only languages
```

### "Too many false positives"

```bash
# .ubsignore
echo "test-fixtures/" >> .ubsignore
echo "generated/" >> .ubsignore

# Or skip categories
ubs --skip=11,12 .  # TODO/debug
```

### "Need emergency bypass"

```bash
git commit --no-verify -m "Emergency: <reason>"
# Create issue immediately to address
```
