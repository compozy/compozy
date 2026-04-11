---
status: resolved
file: Makefile
line: 86
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVPK,comment:PRRC_kwDORy7nkc621VbB
---

# Issue 002: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🌐 Web query:

`For npm scoped packages, when is `npm publish --access public` required, and what are the defaults if omitted?`

💡 Result:

For npm scoped packages (e.g., `@scope/package`), npm publish --access public is required when you want to publish the package as publicly accessible. Scoped packages default to restricted (private) access if the --access flag is omitted. Key points: - Scoped packages default to --access restricted (private visibility). - Unscoped packages are always public. - To publish a scoped package publicly, use npm publish --access public on the initial publish. - After initial publish, access can be changed with npm access public <package>. - Publishing private scoped packages requires a paid npm account; public scoped packages do not. - The --access flag only affects the initial publish; subsequent publishes do not change it unless using npm access. - Note: publishConfig.access in package.json can override CLI flags.

Citations:

- 1: https://docs.npmjs.com/cli/v8/commands/npm-access
- 2: https://docs.npmjs.com/creating-and-publishing-scoped-public-packages/
- 3: https://docs.npmjs.com/cli/v7/commands/npm-publish/
- 4: https://courses.cs.washington.edu/courses/cse481v/21sp/projects/team1/node-v14.17.0-linux-x64/lib/node_modules/npm/docs/public/using-npm/scope/
- 5: https://npm.github.io/orgs-docs/publishing-an-org-scoped-package.html
- 6: https://docs.npmjs.com/creating-and-publishing-private-packages/
- 7: https://docs.npmjs.com/creating-and-publishing-an-organization-scoped-package/

---

🏁 Script executed:

```shell
# Find package.json files that might belong to these packages
find . -type f -name "package.json" | head -20
```

Repository: compozy/compozy

Length of output: 392

---

🏁 Script executed:

```shell
# Check the package.json files for publishConfig.access
echo "=== ./sdk/extension-sdk-ts/package.json ==="
cat ./sdk/extension-sdk-ts/package.json | grep -A 5 "publishConfig"

echo ""
echo "=== ./sdk/create-extension/package.json ==="
cat ./sdk/create-extension/package.json | grep -A 5 "publishConfig"
```

Repository: compozy/compozy

Length of output: 147

---

🏁 Script executed:

```shell
# Read the full package.json files to see their complete configuration
echo "=== ./sdk/extension-sdk-ts/package.json ==="
cat ./sdk/extension-sdk-ts/package.json

echo ""
echo "=== ./sdk/create-extension/package.json ==="
cat ./sdk/create-extension/package.json
```

Repository: compozy/compozy

Length of output: 1562

---



</details>

**Add `--access public` flag to npm publish commands for scoped packages.**

These scoped packages (`@compozy/extension-sdk` and `@compozy/create-extension`) lack `publishConfig.access` configuration in their package.json files. Scoped packages default to private access when published without an explicit `--access public` flag. Add this flag to ensure they publish with public access:

```
npm publish --workspace `@compozy/extension-sdk` --access public
npm publish --workspace `@compozy/create-extension` --access public
```

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@Makefile` around lines 85 - 86, The Makefile publish commands for the scoped
packages do not set public access; update the two npm publish invocations in the
Makefile that reference `@compozy/extension-sdk` and `@compozy/create-extension` to
include the --access public flag so scoped packages are published as public
(i.e., modify the npm publish --workspace `@compozy/extension-sdk` and npm publish
--workspace `@compozy/create-extension` lines to add --access public).
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:ba2aaa30-7f4d-46c1-9957-425c6870aaec -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - Both published packages are scoped (`@compozy/...`) and neither package manifest defines `publishConfig.access`, so the publish target does not currently make the intended public visibility explicit.
  - Root cause: the Makefile publish commands rely on npm defaults instead of declaring public access at the release step.
  - Implemented fix: added `--access public` to both `npm publish --workspace @compozy/...` invocations.
