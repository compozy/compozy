# Issue File Template

Use this exact structure for every issue file. The file is parsed by
`reviews.ReadReviewEntries()` and `prompt.ParseReviewContext()`.

## Format

```
# Issue NNN: <concise title summarizing the problem>

## Status: pending

<review_context>
  <file>path/to/file.go</file>
  <line>42</line>
  <severity>critical|high|medium|low</severity>
  <author>claude-code</author>
  <provider_ref></provider_ref>
</review_context>

## Review Comment

<detailed description of the issue, why it is a problem,
and a suggested fix with a concise code snippet if helpful>

## Triage

- Decision: `UNREVIEWED`
- Notes:
```

## Field Definitions

- **NNN**: Three-digit zero-padded issue number (001, 002, ...).
- **title**: One-line summary of the problem. Maximum 72 characters.
- **file**: Relative path from repository root to the affected source file.
  Use `unknown` only when the issue is purely architectural and not tied to a
  specific file.
- **line**: Line number where the issue is most visible. Use `0` when no
  specific line applies.
- **severity**: Exactly one of `critical`, `high`, `medium`, `low`.
  Read `review-criteria.md` for definitions.
- **author**: Always `claude-code` for manual review rounds.
- **provider_ref**: Always empty for manual review rounds.

## Parser Compatibility

- The `<review_context>` XML block must be well-formed and parseable by
  Go `encoding/xml.Unmarshal`. Escape XML special characters in field values:
  `<` as `&lt;`, `>` as `&gt;`, `&` as `&amp;`.
- The `## Status:` heading is matched by regex and must appear on its own line.
- Issue file names must match the pattern `issue_NNN.md` where NNN is a
  zero-padded number, for `prompt.ExtractIssueNumber()` to recognize them.

## Rules

- One issue per file. Do not combine multiple unrelated problems.
- The Review Comment must be actionable: state the problem clearly and
  provide a concrete suggestion for how to fix it.
- Keep code snippets in Review Comment under 15 lines.
- Keep the title descriptive but short.
  Good: "Missing nil check before map access in resolveConfig".
  Bad: "Code issue".
