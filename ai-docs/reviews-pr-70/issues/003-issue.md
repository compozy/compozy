# Issue 3 - Review Thread Comment

**File:** `internal/core/agent/registry_launch.go:87`
**Date:** 2026-04-07 11:44:37 America/Sao_Paulo
**Status:** - [x] ADDRESSED

## Technical Disposition

`VALID` - `EnsureAvailable` dereferenced `cfg.DryRun` before validating that `cfg` was non-nil. The fix now rejects a nil runtime config up front and covers that path in `internal/core/agent/registry_test.go`.

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Potential nil pointer dereference on `cfg.DryRun`.**

If `cfg` is `nil`, accessing `cfg.DryRun` at line 64 will panic before the nil-context check at line 67. While `ValidateRuntimeConfig` rejects nil configs, callers using `EnsureAvailable` directly (via the `RuntimeRegistry` interface) could bypass that validation.

Consider adding a nil check for `cfg` or documenting that callers must validate the config first.



<details>
<summary>🛡️ Proposed fix</summary>

```diff
 func EnsureAvailable(ctx context.Context, cfg *model.RuntimeConfig) error {
+	if cfg == nil {
+		return errors.New("runtime config is nil")
+	}
 	if cfg.DryRun {
 		return nil
 	}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry_launch.go` around lines 63 - 87, EnsureAvailable
may dereference a nil cfg (cfg.DryRun) before the ctx nil-check; add an explicit
nil check for cfg at the start of EnsureAvailable (e.g., if cfg == nil { return
fmt.Errorf("nil RuntimeConfig") }) and return a clear error, preserving the
existing ctx nil-default logic and subsequent calls to lookupAgentSpec and
resolveLaunchCommand; reference EnsureAvailable and cfg.DryRun when making the
change.
```

</details>

<!-- fingerprinting:phantom:poseidon:ocelot:f7ae0487-a12c-46f8-89d9-118afa50994b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55T4Lf`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55T4Lf
```

---
*Generated from PR review - CodeRabbit AI*
