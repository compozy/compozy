---
status: resolved
file: internal/cli/extension/doctor.go
line: 94
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVOq,comment:PRRC_kwDORy7nkc621VaW
---

# Issue 004: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Avoid the “no drift issues” footer when drift warnings already exist.**

`report.Infos` only comes from `overrideInfos()`. If `skillPackDriftWarnings()` adds a warning but there are no override records, Lines 92-94 still append `No extension override or drift issues detected.`, which contradicts the warning section.



<details>
<summary>Suggested fix</summary>

```diff
-	if len(report.Infos) == 0 {
-		report.Infos = append(report.Infos, "No extension override or drift issues detected.")
-	}
+	if len(report.Infos) == 0 {
+		report.Infos = append(report.Infos, "No extension override records detected.")
+	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	report.Infos = append(report.Infos, overrideInfos(result.Overrides)...)
	slices.Sort(report.Errors)
	slices.Sort(report.Warnings)
	slices.Sort(report.Infos)
	if len(report.Infos) == 0 {
		report.Infos = append(report.Infos, "No extension override records detected.")
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/extension/doctor.go` around lines 88 - 94, The footer "No
extension override or drift issues detected." is added whenever report.Infos is
empty even if report.Warnings contains drift warnings from
skillPackDriftWarnings(); update the conditional so the message is only appended
when both report.Infos and report.Warnings are empty. Specifically, modify the
block that currently checks len(report.Infos) == 0 (after appending
overrideInfos via overrideInfos(result.Overrides)) to check len(report.Infos) ==
0 && len(report.Warnings) == 0 so that warnings prevent the misleading footer.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:388c7721-ae35-4ddb-a0e7-78fbd9aa7a58 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - The fallback info message currently says `No extension override or drift issues detected.` even though drift warnings are emitted through `report.Warnings`, not `report.Infos`.
  - Root cause: the fallback string is broader than the data source it summarizes; `Infos` only reflect override records.
  - Implemented fix: narrowed the fallback info text so it only describes missing override records, and added regression coverage for the drift-warning case.
