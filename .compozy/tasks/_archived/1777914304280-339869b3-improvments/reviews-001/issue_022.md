---
status: resolved
file: internal/core/reviews/store.go
line: 250
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUld,comment:PRRC_kwDORy7nkc68K-QZ
---

# Issue 022: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Do not ignore issues that are missing round metadata.**

If one issue has front-matter round fields and another has none, this loop silently skips the metadata-less entry and snapshots the round from the remaining subset. That bypasses the legacy fallback path and can hide a partially migrated or corrupted review round.

<details>
<summary>Suggested fix</summary>

```diff
 	for _, entry := range entries {
 		next, ok, err := roundMetaFromReviewEntry(entry)
 		if err != nil {
 			return model.RoundMeta{}, err
 		}
 		if !ok {
-			continue
+			return model.RoundMeta{}, errReviewRoundMetadataUnavailable
 		}
 		if meta == nil {
 			meta = &next
 			continue
 		}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/reviews/store.go` around lines 233 - 250, The loop over entries
currently skips entries with missing round metadata (when
roundMetaFromReviewEntry returns ok==false) which allows a mix of meta-present
and meta-absent entries; update the logic in the loop that calls
roundMetaFromReviewEntry so that if meta is already set and an entry returns
ok==false you return an error (e.g., indicate inconsistent or missing round
metadata for entry.AbsPath) instead of continuing, while preserving the existing
behavior that if meta remains nil after the loop you fall back to the legacy
path; refer to the roundMetaFromReviewEntry call, the meta variable, and
roundMetaMatches to locate and change the conditional.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e59768e0-9289-4ad8-8c6f-2ed95ddf0cc4 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: Confirmed `roundMetaFromIssueFrontMatter` skipped metadata-less entries even after seeing frontmatter metadata from another entry. This allowed mixed migrated/legacy issue sets to snapshot from a partial subset. Fixed by preserving all-missing legacy fallback while rejecting mixed metadata-present and metadata-missing entries.
