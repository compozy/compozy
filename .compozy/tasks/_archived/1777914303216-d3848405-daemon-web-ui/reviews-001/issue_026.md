---
status: resolved
file: packages/ui/src/tokens.css
line: 170
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqYv,comment:PRRC_kwDORy7nkc651WI9
---

# Issue 026: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Set light-mode `color-scheme` explicitly.**

`.light` overrides tokens but does not override `color-scheme`, so UA-rendered controls/scrollbars can remain dark in light mode.

<details>
<summary>Proposed fix</summary>

```diff
 .light {
+  color-scheme: light;
+
   --background: `#ffffff`;
   --foreground: var(--stone-800);
   --card: `#ffffff`;
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@packages/ui/src/tokens.css` around lines 142 - 170, The .light CSS rule in
tokens.css should explicitly set the color-scheme to ensure UA controls and
scrollbars render light; update the .light selector to include "color-scheme:
light;" (and optionally "forced-color-adjust: none;" if needed for consistent
OS-level forced color handling) so that browser-native UI elements follow the
light theme along with the existing CSS variables.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:9a0b004f-9914-46c3-b9a6-ca8c778f1453 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The review is valid. `.light` overrides semantic tokens for the light theme but inherits `color-scheme: dark` from `:root`, which can leave browser-native controls and scrollbars rendered in dark mode.
  - Root cause: the light-theme override updates tokens only and never resets the document color-scheme hint.
  - Fix approach: set `color-scheme: light` in `.light` and extend token coverage to assert the light-theme override exists.

## Resolution

- Added `color-scheme: light;` to `.light` in `packages/ui/src/tokens.css`.
- Extended `packages/ui/tests/tokens.test.ts` to assert both the dark root hint and the light-theme override.
- Verification:
- `bunx vitest run --config vitest.config.ts tests/primitives.test.tsx tests/package-exports.test.ts tests/tokens.test.ts tests/storybook-stories.test.tsx` (from `packages/ui`)
- `make verify`
