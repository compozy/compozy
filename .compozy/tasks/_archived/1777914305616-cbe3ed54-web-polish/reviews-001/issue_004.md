---
status: resolved
file: packages/ui/src/components/markdown.tsx
line: 21
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc59RNfm,comment:PRRC_kwDORy7nkc662oQS
---

# Issue 004: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🌐 Web query:

`In Tailwind CSS v4, what is the correct arbitrary variant syntax to style descendant anchor elements only when the anchor itself is hovered: `hover:[&_a]:brightness-110` or `[&_a:hover]:brightness-110`?`

💡 Result:

The correct arbitrary variant syntax in Tailwind CSS v4 to style descendant anchor elements (a) only when the anchor itself is hovered is [&_a:hover]:brightness-110. This generates the CSS selector element &_a:hover { filter: brightness(1.1); }, which targets a:hover descendants of the element with the class. The alternative hover:[&_a]:brightness-110 generates element:hover &_a { filter: brightness(1.1); }, which styles all descendant a elements when their ancestor is hovered, regardless of the anchor's hover state.

Citations:

- 1: https://tailwindcss.com/docs/hover-focus-and-other-states
- 2: https://www.stefanjudis.com/today-i-learned/how-to-style-element-descendants-with-tailwind-css/
- 3: https://tailwindcss.com/docs/pseudo-class-variants
- 4: https://tailwindcss.com/docs/adding-custom-styles

---

🏁 Script executed:

```shell
cd packages/ui/src/components && if [ -f markdown.tsx ]; then cat -n markdown.tsx | head -30; else echo "File not found"; fi
```

Repository: compozy/compozy

Length of output: 2291

---



</details>

**Fix the anchor hover selector to target the link itself, not the wrapper.**

`hover:[&_a]:brightness-110` brightens links when the markdown container is hovered. Use `[&_a:hover]:brightness-110` instead to brighten links on their own hover state.

<details>
<summary>Suggested change</summary>

```diff
-  "[&_a]:text-[color:var(--primary)] [&_a]:underline [&_a]:underline-offset-2 hover:[&_a]:brightness-110",
+  "[&_a]:text-[color:var(--primary)] [&_a]:underline [&_a]:underline-offset-2 [&_a:hover]:brightness-110",
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
  "[&_a]:text-[color:var(--primary)] [&_a]:underline [&_a]:underline-offset-2 [&_a:hover]:brightness-110",
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@packages/ui/src/components/markdown.tsx` at line 21, The hover rule currently
brightens links when the markdown container is hovered because the class uses
hover:[&_a]:brightness-110; update the selector to target the anchor's own hover
state by replacing hover:[&_a]:brightness-110 with [&_a:hover]:brightness-110 in
the class list inside the Markdown component (look for the string array in
packages/ui/src/components/markdown.tsx that contains
"[&_a]:text-[color:var(--primary)] [&_a]:underline [&_a]:underline-offset-2
hover:[&_a]:brightness-110").
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:fd160fe4-d2d3-4a0b-ad04-a1afdb520e03 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The current class uses `hover:[&_a]:brightness-110`, which reacts to hovering the markdown container instead of the anchor element itself.
  - That makes link emphasis fire on the wrong interaction target.
  - Fix: switch to `[&_a:hover]:brightness-110` and add a regression test that locks the rendered class contract.
