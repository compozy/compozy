# Deep Analysis Output Template

Use this template for the Phase 2 diagnostic document and the final comprehensive analysis. Replace bracketed prompts with project-specific content.

```markdown
🔎 Deep Analysis Complete
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 Summary
├─ Findings: X total
├─ Critical: X
├─ High: X
├─ Medium: X
└─ Low: X

🧩 Finding #[num]: [Category]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📍 Location: path/to/file:line-range (or component)
⚠️ Severity: Critical/High/Medium/Low
📂 Category: Runtime/Logic/Concurrency/Memory/Resource/Performance/Architecture

Root Cause:
[Concise description]

Impact:
[User/system impact]

Evidence:

- [Code citation or trace]
- [Relevant logs/paths/flows]

Solution Strategy:
[One-line actionable recommendation]

Related Areas:

- [List of related files/components]

[Repeat for additional findings]

🔗 Dependency/Flow Map (if applicable)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[Summarize critical interactions and edges]

🌐 Broader Context Considerations (REQUIRED)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

- Reviewed Areas: [files/modules/callers/callees/interfaces/config/tests]
- Impacted Areas Matrix: [area → impact → risk → priority]
- Unknowns/Gaps: [what remains uncertain]
- Assumptions: [explicit assumptions made]

📐 Standards Compliance
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

- Rules satisfied: [@api-standards.mdc, @architecture.mdc, ...]
- Constraints considered: [context-first, logger.FromContext(ctx), DI, testing patterns]
- Deviations (if any): [explain and provide compliant alternative]

✅ Verified Sound Areas
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

- [Confirmed correct patterns/components]

🎯 Fix Priority Order
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. [Highest priority]
2. [Next]
3. [...]

Returning control to the main agent. No changes performed.
```

## Save Block Format

After printing the markdown analysis report, emit the structured save block with the same content and a timestamped filename:

```xml
<save>
  <destination>
    ./ai-docs/deep-analysis/{UTC_YYYYMMDD-HHMMSS}-{safe_name}.md
  </destination>
  <format>markdown</format>
  <content>
  [PASTE THE FULL REPORT MARKDOWN HERE]
  </content>
  <audience>main-agent</audience>
</save>
```

For the initial Phase 2 RepoPrompt/diagnostic output, use the same structure but append `_phase2` to the filename before the extension.
