# Issue 1 - Review Thread Comment

**File:** `agents/embed.go:8`
**Date:** 2026-04-13 18:25:17 UTC
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
# Description: List all files in the agents/ directory that will be embedded

fd . agents/ --type f
```

Repository: compozy/compozy

Length of output: 74

---

🏁 Script executed:

```shell
find agents/ -type d
```

Repository: compozy/compozy

Length of output: 66

---

</details>

**The embed pattern `*` will bundle the Go source file itself into the binary, which is likely unintended.**

The `agents/` directory currently contains only `agents/embed.go`. Using `//go:embed *` means the Go source file gets embedded in the binary—uncommon and wasteful. Additionally, the wildcard pattern will auto-embed any future files added to this directory, which could be problematic for maintenance.

Consider using a more specific pattern (e.g., `*.md` or embedding from subdirectories if that's the intent) to avoid unintended files and make the intent explicit.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@agents/embed.go` around lines 7 - 8, The current embed declaration var FS
embed.FS with directive //go:embed * will unintentionally include this source
file and any future files; change the pattern to an explicit/limited glob that
matches only intended assets (for example use a specific extension like `*.md`,
or a subdirectory pattern such as `assets/*` or `templates/*`) and update the
//go:embed line above the FS variable accordingly so only the desired files are
bundled.
```

</details>

<!-- fingerprinting:phantom:poseidon:ocelot:446b1696-5718-4b30-95b3-d9273668c060 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: VALID
- Resolution: narrowed the embed scope to `README.md` and added `agents/README.md` so the empty embedded filesystem remains readable without embedding Go source files.

## Resolve

Thread ID: `PRRT_kwDORy7nkc56nVLo`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc56nVLo
```

---

_Generated from PR review - CodeRabbit AI_
