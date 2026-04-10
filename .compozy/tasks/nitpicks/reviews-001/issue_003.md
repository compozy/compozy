---
status: resolved
file: internal/core/fetch.go
line: 195
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56KkMA,comment:PRRC_kwDORy7nkc62sRjS
---

# Issue 003: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Track `SourceReviewID` in the history state too.**

This filter only re-imports a resolved nitpick when `submitted_at` is *strictly* later. Provider-side dedup already treats equal timestamps with a higher `SourceReviewID` as newer, so a reposted nitpick in the same second can be accepted upstream and then dropped here once the older copy was resolved. Persist and compare `SourceReviewID` alongside the timestamp so both code paths use the same freshness rule.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/fetch.go` around lines 153 - 195, nitpickHistoryState currently
only stores SourceReviewSubmittedAt so filterFetchedNitpicks can drop reposts
with identical timestamps; extend nitpickHistoryState to include SourceReviewID
and update loadNitpickHistory/save logic to persist it, then in
filterFetchedNitpicks when deciding to re-import a resolved nitpick compare both
parsed timestamp (via parseReviewSubmittedAt(item.SourceReviewSubmittedAt)) and,
if timestamps are equal, compare item.SourceReviewID against
record.SourceReviewID (use the same ordering semantics as the provider: higher
SourceReviewID is newer) so reposts in the same second are treated consistently.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:3c6a12e0-accd-4d1e-a3db-5a1d78606f67 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `filterFetchedNitpicks` only compares `SourceReviewSubmittedAt` when deciding whether a resolved nitpick should be re-imported, but CodeRabbit deduplicates same-second reposts using review ID as the tiebreaker.
- Impact: A reposted nitpick with the same timestamp but a newer `SourceReviewID` can be accepted upstream and then dropped by fetch history, causing real nitpicks to disappear from later rounds.
- Fix approach: Persist `SourceReviewID` in `nitpickHistoryState`, use it as the equal-timestamp freshness tiebreaker when filtering, and update fetch regression coverage to prove same-second higher review IDs are re-imported.
- Resolution: Fetch nitpick history now keeps `SourceReviewID` in memory, uses it as the same-timestamp freshness tiebreaker during filtering, and retains the newest historical record with the same ordering rule.
- Verification: `go test ./internal/cli ./internal/core ./internal/core/provider/coderabbit` passed, and `env -u COMPOZY_NO_UPDATE_NOTIFIER make verify` passed cleanly.
