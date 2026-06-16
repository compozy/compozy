---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/run/ui/view_test.go
line: 1497
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhqx,comment:PRRC_kwDORy7nkc7LlP4C
---

# Issue 020: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Broaden the forced-background SGR check.**

`assertNoForcedBackground` only detects truecolor/256-color backgrounds (`48;2;`, `48;5;`), so it can miss 16-color background codes like `40`–`47`/`100`–`107` or reverse video `7` while still claiming foreground-only rendering.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/run/ui/view_test.go` around lines 1487 - 1497, The
assertNoForcedBackground function only checks for truecolor (48;2;) and
256-color (48;5;) background SGR codes, but misses 16-color background codes
(40–47 for standard colors and 100–107 for bright colors) and reverse video (7).
Extend the conditional check to include these additional SGR patterns alongside
the existing checks for 48;2; and 48;5;, ensuring the function detects all
possible ways a forced background could be applied to the content string.
```

</details>

<!-- cr-comment:v1:824378735a01fc9af918f680 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `assertNoForcedBackground` only checks truecolor and 256-color background SGR codes, missing standard/bright 16-color backgrounds and reverse video.
- Fix approach: parse SGR escape parameters and fail on background color codes `40`-`47`, `100`-`107`, `48`, or reverse video `7`, while preserving the empty-content guard. The broader assertion exposed the composer textarea's virtual cursor reverse-video escape, so the fix also requires minimal production touches in `internal/core/run/ui/model.go` and `internal/core/run/ui/view.go` outside the initial code-file list to use the real Bubble Tea cursor while keeping rendered text foreground-only.

## Resolution

- Resolved by broadening forced-background SGR parsing and keeping composer rendering foreground-only.
- Verification: `rtk make verify` exited 0 after the code changes.
