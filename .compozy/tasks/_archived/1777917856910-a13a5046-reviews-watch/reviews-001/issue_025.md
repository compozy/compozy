---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: packages/ui/src/components/surface-card.tsx
line: 26
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22Es,comment:PRRC_kwDORy7nkc68_V7N
---

# Issue 025: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Avoid forcing `shrink-0` when header/footer has only one child.**

With a single child, that element matches both `:first-child` and `:last-child`; forcing `shrink-0` can cause overflow instead of shrinking/wrapping.

<details>
<summary>Proposed fix</summary>

```diff
- "flex min-w-0 items-start justify-between gap-4 border-b border-border-subtle px-5 py-4 [&>*:first-child]:min-w-0 [&>*:last-child]:shrink-0",
+ "flex min-w-0 items-start justify-between gap-4 border-b border-border-subtle px-5 py-4 [&>*:first-child]:min-w-0 [&>*:last-child]:shrink-0 [&>*:only-child]:shrink",

- "flex min-w-0 items-center justify-between gap-3 border-t border-border-subtle px-5 py-4 [&>*:first-child]:min-w-0 [&>*:last-child]:shrink-0",
+ "flex min-w-0 items-center justify-between gap-3 border-t border-border-subtle px-5 py-4 [&>*:first-child]:min-w-0 [&>*:last-child]:shrink-0 [&>*:only-child]:shrink",
```
</details>

 


Also applies to: 69-69

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@packages/ui/src/components/surface-card.tsx` at line 26, The header/footer
class forces shrink-0 on the last child which, when there's only one child (it
is both first and last), prevents shrinking; update the selector so shrink-0 is
only applied when the last child is not the only child—replace the
[&>*:last-child]:shrink-0 usage with a non-only-child selector such as
[&>*:last-child:not(:only-child)]:shrink-0 (and apply the same change for the
other occurrence referenced in the file) while keeping the existing
[&>*:first-child]:min-w-0 rule.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:3b1ae74e-bb76-420f-b820-5ec86b24a552 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: The reviewer is correct that a single child matches both `:first-child` and `:last-child`, so the current `shrink-0` selector can block shrinking in the one-child case. I will narrow the selector so only a non-unique trailing child gets `shrink-0`.
- Resolution: Narrowed the selector to `:last-child:not(:only-child)` for both header and footer slots and added UI regression coverage; confirmed in `make verify`.
