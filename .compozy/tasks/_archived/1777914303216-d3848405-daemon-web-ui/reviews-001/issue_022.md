---
status: resolved
file: packages/ui/.storybook/preview.css
line: 3
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqYk,comment:PRRC_kwDORy7nkc651WIw
---

# Issue 022: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail

echo "== Tailwind versions found in package.json files =="
fd -HI '^package\.json$' | while read -r pkg; do
  v=$(jq -r '.dependencies.tailwindcss // .devDependencies.tailwindcss // empty' "$pkg")
  if [ -n "$v" ]; then
    echo "$pkg -> $v"
  fi
done

echo
echo "== Stylelint config entries related to at-rule handling =="
fd -HI '(^\.stylelintrc(\..+)?$|^stylelint\.config\.(js|cjs|mjs|ts)$|^package\.json$)' | while read -r cfg; do
  if [[ "$cfg" == *package.json ]]; then
    jq -r 'if .stylelint then "\(input_filename): has stylelint config" else empty end' "$cfg"
    jq -r '(.stylelint.rules // {}) | to_entries[]? | select(.key|test("at-rule-no-unknown|scss/at-rule-no-unknown")) | "\(input_filename): \(.key) => \(.value|tostring)"' "$cfg" 2>/dev/null || true
  else
    echo "-- $cfg"
    rg -n -C2 'at-rule-no-unknown|scss/at-rule-no-unknown|ignoreAtRules|source|tailwind' "$cfg" || true
  fi
done
```

Repository: compozy/compozy

Length of output: 253

---

🏁 Script executed:

```shell
cat -n .stylelintrc.json
```

Repository: compozy/compozy

Length of output: 1685

---

🏁 Script executed:

```shell
cat -n packages/ui/.storybook/preview.css
```

Repository: compozy/compozy

Length of output: 135

---

🌐 Web query:

`Tailwind v4 `@source` directive Stylelint at-rule-no-unknown`

💡 Result:

The `@source` directive in Tailwind CSS v4 is a custom at-rule used to explicitly specify additional source files or paths for Tailwind to scan for class names beyond its automatic content detection. It is documented in the official Tailwind docs as one of the core directives. Stylelint's "at-rule-no-unknown" rule flags unknown CSS at-rules like `@source` by default, as it only recognizes standard CSS at-rules. To fix this: 1. Use a Tailwind v4-specific Stylelint config: Install `@dreamsicle.io/stylelint-config-tailwindcss` (designed for Tailwind v4+ and Stylelint 16.17+), which properly extends syntax to recognize all Tailwind v4 directives including `@source` via languageOptions. npm install --save-dev `@dreamsicle.io/stylelint-config-tailwindcss` stylelint stylelint-config-standard Then in .stylelintrc.js or eslint.config.js: module.exports = { extends: [ 'stylelint-config-standard', '@dreamsicle.io/stylelint-config-tailwindcss' ] }; 2. Manual configuration: Override the rule in your Stylelint config: module.exports = { rules: { 'at-rule-no-unknown': [true, { ignoreAtRules: ['source', 'theme', 'utility', 'variant', 'import', 'config', 'plugin'] }] } }; Note: This uses ignoreAtRules, but for full Tailwind v4 support (functions, properties), prefer the dedicated config which uses languageOptions.syntax.atRules. Other Tailwind v4 directives (`@theme`, `@utility`, etc.) require similar handling. Legacy configs like stylelint-config-tailwindcss are for v3.

Citations:

- 1: https://tailwindcss.com/docs/functions-and-directives
- 2: https://stylelint.io/user-guide/rules/at-rule-no-unknown/
- 3: https://github.com/dreamsicle-io/stylelint-config-tailwindcss
- 4: https://tailwindcss.com/blog/tailwindcss-v4

---

🌐 Web query:

`stylelint-config-standard-scss at-rule-no-unknown default configuration`

💡 Result:

The default configuration for the "at-rule-no-unknown" rule in stylelint-config-standard-scss is the SCSS-specific version from the stylelint-scss plugin enabled as true, i.e., "scss/at-rule-no-unknown": true. This is achieved because the config extends stylelint-config-standard (which has "at-rule-no-unknown": true) and stylelint-config-recommended-scss. The recommended-scss config bundles the stylelint-scss plugin and disables the core "at-rule-no-unknown" rule while enabling "scss/at-rule-no-unknown": true to recognize SCSS at-rules like `@use`, `@extend`, `@mixin`, etc., without errors. The core rule is overridden by the SCSS-aware rule from the plugin, which wraps the core rule but adds SCSS-specific at-rules to the known list.

Citations:

- 1: https://github.com/stylelint-scss/stylelint-config-standard-scss
- 2: https://github.com/stylelint-scss/stylelint-scss/blob/master/src/rules/at-rule-no-unknown/README.md
- 3: https://github.com/stylelint-scss/stylelint-config-standard-scss/blob/main/README.md
- 4: https://github.com/stylelint-scss/stylelint-config-recommended-scss
- 5: https://github.com/stylelint-scss/stylelint-config-standard-scss/tree/v17.0.0

---



</details>

**Fix the Stylelint rule violation for the Tailwind v4 `@source` directive.**

The `@source` directive is valid in Tailwind v4, but the current Stylelint config (which extends `stylelint-config-standard-scss`) flags it as unknown via the `scss/at-rule-no-unknown` rule, causing CI failures. Add a scoped suppression:

<details>
<summary>Recommended fix</summary>

```diff
 `@import` "tailwindcss";
 
+/* stylelint-disable-next-line scss/at-rule-no-unknown */
 `@source` "../src/**/*.{ts,tsx}";
```
</details>

Alternatively, update `.stylelintrc.json` to configure `at-rule-no-unknown` with `ignoreAtRules` for Tailwind directives, or adopt a Tailwind v4-specific Stylelint config for broader directive support.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
`@import` "tailwindcss";

/* stylelint-disable-next-line scss/at-rule-no-unknown */
`@source` "../src/**/*.{ts,tsx}";
```

</details>

<!-- suggestion_end -->

<details>
<summary>🧰 Tools</summary>

<details>
<summary>🪛 Stylelint (17.7.0)</summary>

[error] 3-3: Unexpected unknown at-rule "@source" (scss/at-rule-no-unknown)

(scss/at-rule-no-unknown)

</details>

</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@packages/ui/.storybook/preview.css` at line 3, The `@source` Tailwind v4
directive is being flagged by the scss/at-rule-no-unknown Stylelint rule; fix by
adding a scoped suppression immediately before the directive in
preview.css—insert a comment like /* stylelint-disable-next-line
scss/at-rule-no-unknown */ directly above the `@source` "../src/**/*.{ts,tsx}";
line so only that rule is disabled for this line; alternatively, update
.stylelintrc.json to add "source" to ignoreAtRules under at-rule-no-unknown if
you prefer a config change.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:38b0a0df-a444-4870-8217-60b9591bdb24 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Notes:
  - The review does not match the current repository tooling. The current tree has no `.stylelintrc*` file and no stylelint step in package scripts, `Makefile`, or CI; frontend verification uses `oxfmt`, `oxlint`, TypeScript, Vitest, builds, and Playwright instead.
  - Root cause of the comment: it assumes a Stylelint configuration that is not present in this branch.
  - Resolution path: no code change is warranted here. If Stylelint is introduced later, Tailwind at-rule support should be configured centrally rather than with inline suppressions.

## Resolution

- Closed as invalid. No code change was made because the repository does not currently include Stylelint configuration or execution in its verification path.
- Verification:
- `make verify`
