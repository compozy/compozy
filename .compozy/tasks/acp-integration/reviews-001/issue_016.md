---
status: resolved
file: internal/core/run/ui_view.go
line: 669
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHCZ,comment:PRRC_kwDORy7nkc60y0ge
---

# Issue 016: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Potential index out of bounds when accessing `offsets[job.selectedEntry]`.**

At line 657, `offsets[job.selectedEntry]` is accessed without verifying that `job.selectedEntry` is within bounds of `offsets`. If `selectedEntry` is modified concurrently or set to an invalid value, this could panic.


<details>
<summary>🛡️ Add bounds check</summary>

```diff
 func (m *uiModel) restoreTranscriptViewport(job *uiJob, offsets []int) {
 	if len(offsets) == 0 {
 		m.transcriptViewport.GotoTop()
 		job.transcriptYOffset = 0
 		job.transcriptXOffset = 0
 		job.transcriptFollowTail = true
 		return
 	}
 	if job.transcriptFollowTail {
 		m.transcriptViewport.GotoBottom()
 	} else {
 		m.transcriptViewport.SetYOffset(job.transcriptYOffset)
 		m.transcriptViewport.SetXOffset(job.transcriptXOffset)
 	}

+	if job.selectedEntry < 0 || job.selectedEntry >= len(offsets) {
+		job.transcriptYOffset = m.transcriptViewport.YOffset()
+		job.transcriptXOffset = m.transcriptViewport.XOffset()
+		job.transcriptFollowTail = m.transcriptViewport.AtBottom()
+		return
+	}
+
 	selectedLine := offsets[job.selectedEntry]
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/ui_view.go` around lines 642 - 669, The access
offsets[job.selectedEntry] in restoreTranscriptViewport can panic if
job.selectedEntry is out of range; before using it, validate and clamp
job.selectedEntry (or fall back to 0) against len(offsets) (e.g. if
job.selectedEntry < 0 or >= len(offsets) set selectedIndex := 0 or selectedIndex
:= min(max(job.selectedEntry,0), len(offsets)-1)) and then use
offsets[selectedIndex] to compute selectedLine; update job.selectedEntry if you
choose to clamp so future calls remain safe. Ensure this check sits just before
computing selectedLine in the restoreTranscriptViewport method of uiModel and
references uiJob and offsets.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:5915fb52-e9fa-449e-96f7-3838f12d9bae -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Rationale: `selectedEntry` is clamped by `syncSelectedEntry` before timeline rendering, and the Bubble Tea model mutates UI state on a single update loop rather than from concurrent goroutines. `offsets` is built from the same snapshot used for the selection index, so the reported out-of-bounds path is not reproducible in the current code.
