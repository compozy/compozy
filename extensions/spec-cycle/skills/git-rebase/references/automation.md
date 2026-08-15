# Advanced: Automated Conflict Resolution

## When You KNOW the Resolution Strategy Upfront

For CI/CD pipelines or automated rebases:

```bash
# Accept ALL incoming changes (risky - verify first!)
git rebase -X theirs origin/main

# Accept ALL your changes (also risky!)
git rebase -X ours origin/main

# Three-way recursive merge (safest auto-strategy)
git rebase -Xrecursive origin/main
```

**⚠️ WARNING**: Only use automated strategies when:

1. You understand the consequences
2. You have comprehensive tests
3. Changes are non-critical
4. You've verified the strategy is correct for this scenario

**For production code**: Use manual resolution from the core workflow in SKILL.md.
