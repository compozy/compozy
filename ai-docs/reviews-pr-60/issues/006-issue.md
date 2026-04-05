# Issue 6 - Review Thread Comment

**File:** `internal/core/agent/client.go:292`
**Date:** 2026-04-05 19:22:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** INVALID

**Rationale:** The concern is plausible, but the current ACP/session contract exposes no explicit replay boundary after `LoadSession`. Without protocol-level metadata, replacing the quiet window with a local handoff would still be unable to distinguish late replay updates from the first prompt-turn updates and would risk suppressing legitimate resumed-turn output.

## Body

_⚠️ Potential issue_ | _🟠 Major_

**A fixed 15ms quiet window is not a safe replay boundary.**

`newLoadedSession()` suppresses `LoadSession` replays until `resumeUpdates()` is called. Dropping that guard after `waitForIdle(..., 15*time.Millisecond)` means any runtime that replays a little later can leak old-turn updates into the resumed turn/output. This needs an explicit handoff rather than a timed sleep.

<!-- fingerprinting:phantom:medusa:grasshopper:5606214d-8719-43d5-98a2-064a6cd78ca1 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc547tHi`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc547tHi
```

---
*Generated from PR review - CodeRabbit AI*
