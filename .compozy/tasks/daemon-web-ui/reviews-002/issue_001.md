---
status: resolved
file: Makefile
line: 76
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58jarw,comment:PRRC_kwDORy7nkc655Cwb
---

# Issue 001: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# Find the Makefile and examine the check-bun-version target
git ls-files | grep -i makefile
```

Repository: compozy/compozy

Length of output: 105

---

🏁 Script executed:

```shell
# Search for BUN_VERSION and check-bun-version in the repository
rg "BUN_VERSION|check-bun-version" --type makefile -B 3 -A 10
```

Repository: compozy/compozy

Length of output: 91

---

🏁 Script executed:

```shell
# Look for .bun-version file
fd ".bun-version" -type f
```

Repository: compozy/compozy

Length of output: 229

---

🏁 Script executed:

```shell
# Also check for any bun configuration files
fd "bun" -type f | head -20
```

Repository: compozy/compozy

Length of output: 229

---

🏁 Script executed:

```shell
# Read the Makefile to examine check-bun-version target
cat Makefile | head -100
```

Repository: compozy/compozy

Length of output: 4099

---

🏁 Script executed:

```shell
# Search for BUN_VERSION without type filter
rg "BUN_VERSION|check-bun-version" -B 3 -A 10
```

Repository: compozy/compozy

Length of output: 3330

---

🏁 Script executed:

```shell
# Look for .bun-version or bun version files
find . -name "*bun*version*" -o -name ".bun-version"
```

Repository: compozy/compozy

Length of output: 73

---



</details>

**`check-bun-version` does not actually validate the pinned Bun version.**

The target reads `.bun-version` into `$(BUN_VERSION)` and mentions it in the error message, but never compares the detected Bun version against the pinned requirement. This allows any installed Bun to pass the check, deferring version incompatibilities to downstream lint/build/test failures instead of failing fast. The pattern should match `check-go-version`, which performs numeric version comparison.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@Makefile` around lines 63 - 76, The check-bun-version target currently only
detects the installed Bun but never compares it to the pinned version; update
the target (check-bun-version) to read the pinned version from .bun-version
(similar to check-go-version), parse the installed Bun version from $(BUNCMD)
--version into a comparable string/semver, and perform a numeric/semantic
comparison against the pinned value, failing with a clear error if they differ;
ensure you reference BUNCMD, the .bun-version file, and the check-go-version
pattern for how to perform the comparison and error handling so the Makefile
fails fast on mismatched Bun versions.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:2fe85954-b1fb-4290-8e9e-00be8dcc48f0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `check-bun-version` only verifies that `bun --version` returns a value; it never compares the installed version against the pinned `.bun-version` value exposed as `$(BUN_VERSION)`.
  - That means frontend bootstrap/lint/test/build can proceed on an incompatible Bun version and fail later with harder-to-diagnose errors instead of failing fast at the gate.
  - Implemented: `check-bun-version` now normalizes the required and detected versions into comparable numeric tuples and exits non-zero when they differ.
  - Verification: `make check-bun-version` passed and `make check-bun-version BUN_VERSION=9.9.99` failed with the expected mismatch message.
  - Repo gate note: `make verify` is currently blocked before scoped code by a pre-existing frozen-lockfile mismatch from `package.json` bumping `oxfmt` to `^0.46.0` without a matching `bun.lock` update.
