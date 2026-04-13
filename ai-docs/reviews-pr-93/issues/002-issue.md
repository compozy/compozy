# Issue 2 - Review Thread Comment

**File:** `internal/cli/extension/install.go:78`
**Date:** 2026-04-13 19:08:30 UTC
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Don’t turn a successful install into a hard failure on temp-source cleanup.**

If `CleanupSource` fails after the copy/provenance/state-update path succeeded, this defer still overwrites the command result. The user gets a failed install even though the extension is already present, and a retry will likely hit the “already exists” path. Prefer surfacing cleanup failure as a warning on the success path and only joining it onto an existing install error.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/extension/install.go` around lines 67 - 78, The defer that runs
resolvedSource.CleanupSource currently turns a successful install into a failure
by assigning err when cleanup fails; change the logic inside the deferred func
so that if cleanupErr != nil and err != nil you join the cleanup error onto the
existing err (using errors.Join as now), but if cleanupErr != nil and err == nil
do not set err—emit a warning/log message instead (e.g. via the command logger
or fmt.Fprintf to stderr) so the install returns success while surfacing the
cleanup failure as a warning; update the deferred function around
resolvedSource.CleanupSource(), err, and the errors.Join usage accordingly.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:f0410b06-af06-4312-8bf8-f831ab4cc296 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: VALID
- Resolution: install-source cleanup failures now join onto an existing install error but only emit a warning on the success path, with regression coverage proving the install still succeeds.

## Resolve

Thread ID: `PRRT_kwDORy7nkc56n9hR`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc56n9hR
```

---

_Generated from PR review - CodeRabbit AI_
