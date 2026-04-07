# Verification Report: Refactoring Analysis (2026-04-06)

> **Verified by**: Claude Opus 4.6
> **Date**: 2026-04-06
> **Method**: Automated spot-checks against actual codebase, math verification, cross-report consistency analysis

---

## Overall Assessment

**Report accuracy: HIGH**. The vast majority of claims are verified correct. Line counts, function locations, duplication claims, and architectural observations are substantiated by the actual code. One math error was found in the Provider & Public report's severity breakdown, which propagates to the summary aggregate table. A few minor inaccuracies exist in line counts and file counts, none of which undermine the analysis conclusions.

---

## 1. Math Verification

### Per-Report Finding Counts

| Report | Stated P0 | Stated P1 | Stated P2 | Stated P3 | Stated Total | Verified P0 | Verified P1 | Verified P2 | Verified P3 | Verified Total | Status |
|--------|-----------|-----------|-----------|-----------|--------------|-------------|-------------|-------------|-------------|----------------|--------|
| CLI & Entry | 0 | 5 | 7 | 5 | 17 | 0 | 5 | 7 | 5 | 17 | PASS |
| Core Foundation | 2 | 5 | 6 | 5 | 18 | 2 | 5 | 6 | 5 | 18 | PASS |
| Agent & Run | 4 | 7 | 8 | 6 | 25 | 4 | 7 | 8 | 6 | 25 | PASS |
| Plan & Domain | 3 | 8 | 9 | 4 | 24 | 3 | 8 | 9 | 4 | 24 | PASS |
| Provider & Public | 2 | **7** | **8** | 5 | 22 | 2 | **6** | **9** | 5 | 22 | **FAIL (P1/P2 swap)** |

**Provider & Public error detail**: The report header claims P1=7 and P2=8, but the body contains only 6 P1 findings (F03-F08) and 9 P2 findings (F09-F16, F22). F22 (P2, `isTransientRunLoadError` string matching) appears after the P3 section, suggesting it was added late and the header was not updated. Total of 22 is correct.

### Summary Aggregate Table

The summary propagates the Provider report's P1=7, P2=8 values. With the correction:

| Severity | Stated Total | Corrected Total | Delta |
|----------|-------------|-----------------|-------|
| P0 | 11 | 11 | 0 |
| P1 | 32 | **31** | -1 |
| P2 | 38 | **39** | +1 |
| P3 | 25 | 25 | 0 |
| **Grand Total** | **106** | **106** | **0** |

The grand total of 106 findings is correct. The P1/P2 split is off by one due to the Provider report error.

### Double-Counting Check

No double-counting detected. The summary's "Cross-Report Correlations" section (5 systemic issues) correctly identifies related findings across reports without counting them twice. Specific verification:

- G3-F4 (conversion code duplication in `run/`) and G5-F01 (type hierarchy duplication in `model/` vs `kinds/`) are distinct issues targeting different code
- G2-F4 (decode function repetition within `content.go`) and G5-F01 are distinct issues
- G4-F01 (prompt architectural inversion) and G4-F02/F03 (error wrapper duplication) are consequences of the same root cause but represent separate actionable findings

---

## 2. Spot-Check Results

### Check 1: `execution.go` line count
- **Claimed**: ~1,900 lines
- **Actual**: 1,900 lines (exact)
- **Result**: **PASS**

### Check 2: `exec_flow.go` line count
- **Claimed**: ~1,176 lines
- **Actual**: 1,176 lines (exact)
- **Result**: **PASS**

### Check 3: `root.go` line count
- **Claimed**: ~1,190 lines
- **Actual**: 1,190 lines (exact)
- **Result**: **PASS**

### Check 4: `tool_call_format.go` line count
- **Claimed**: ~733 lines
- **Actual**: 732 lines
- **Result**: **PASS** (1 line off, within "~" tolerance)

### Check 5: Duplicated `wrapTaskParseError`
- **Claimed**: Same function in `internal/core/plan/input.go` and `internal/core/tasks/store.go`, identical
- **Actual**: Confirmed at `plan/input.go:325-330` and `tasks/store.go:225-230`. Bodies are identical: both check `ErrLegacyTaskMetadata` and `ErrV1TaskMetadata`, return the same formatted error strings.
- **Result**: **PASS**

### Check 6: Duplicated `wrapReviewParseError`
- **Claimed**: Same function in `internal/core/plan/input.go` and `internal/core/reviews/store.go`, identical
- **Actual**: Confirmed at `plan/input.go:332-337` and `reviews/store.go:420-425`. Bodies are identical: both check `ErrLegacyReviewMetadata`.
- **Result**: **PASS**

### Check 7: `prompt.ParseTaskFile` callers from `tasks/store.go`
- **Claimed**: `tasks/store.go` imports and calls `prompt.ParseTaskFile`
- **Actual**: Confirmed at `tasks/store.go:92` and `tasks/store.go:213`
- **Result**: **PASS**

### Check 8: `newPreparation` duplication
- **Claimed**: Exists in both `internal/core/api.go` and `internal/core/kernel/handlers.go`, with `handlers.go` dropping the `groups` field
- **Actual**: Confirmed. `api.go:393` includes `groups: jb.Groups` in `newJob`. `handlers.go:279` omits it. The divergence claim is verified.
- **Result**: **PASS**

### Check 9: Triple config chain / `RuntimeFields`
- **Claimed**: `RuntimeFields` in `kernel/commands/runtime_fields.go` with ~30 fields
- **Actual**: Confirmed. File exists at 121 lines. `RuntimeFields` struct has 32 fields (lines 11-44).
- **Result**: **PASS**

### Check 10: `model.go` line count
- **Claimed**: ~370 lines
- **Actual**: 369 lines
- **Result**: **PASS** (1 line off, within "~" tolerance)

### Check 11: `internal/version` usage
- **Claimed**: Never imported by any `.go` file
- **Actual**: Confirmed. No Go source file contains an import of the `internal/version` package. It is only referenced in `Makefile`, `.goreleaser.yml`, and `aur-pkg/PKGBUILD-src` for `-ldflags -X` injection.
- **Result**: **PASS**
- **Note**: Report claims version.go is 14 lines; actual is 13 lines. Minor inaccuracy.

### Check 12: `kinds/session.go` content block duplication
- **Claimed**: Duplicates the block type hierarchy from `model/content.go`
- **Actual**: Confirmed. Both files define identical type hierarchies: `ContentBlockType` enum (6 variants), `SessionStatus`, `SessionUpdateKind`, `ToolCallState`, 6 block structs (`TextBlock`, `ToolUseBlock`, etc.), `ContentBlock` with marshal/unmarshal, `SessionUpdate`, `Usage` with methods, decode/ensure functions. The only substantive difference is JSON tag casing (camelCase in `model`, snake_case in `kinds`).
- **File sizes**: `content.go` = 458 lines, `session.go` = 449 lines. Summary says "~900 lines" for the combined total; this is 907 lines total but not all 907 are duplicated (each file has some unique content).
- **Result**: **PASS**

### Check 13: `commandState` field count
- **Claimed**: 30+ fields at `internal/cli/root.go:38-83`
- **Actual**: 44 fields (36 data fields + 8 function fields) at lines 38-83. Report says "30+ fields including 8 function fields" which is accurate but conservative.
- **Result**: **PASS**

### Check 14: Dead `uiViewFailures`
- **Claimed**: Defined but never used as a distinct view state
- **Actual**: Defined at `run/types.go:223`. Referenced in `ui_view.go:34` in a combined case `case uiViewSummary, uiViewFailures:` -- grouped with `uiViewSummary`, rendering the same view. No code path ever assigns `uiViewFailures` to a variable (grep for `= uiViewFailures` returns zero results). The constant exists but is effectively unused -- no view state is ever set to `uiViewFailures`.
- **Result**: **PASS** (characterization is accurate; it is defined and pattern-matched but never set)

### Check 15: `clampInt` vs `clamp` duplication
- **Claimed**: Both exist in `run/`
- **Actual**: Confirmed. `clamp` at `run/ui_layout.go:45` and `clampInt` at `run/validation_form.go:225`. Both are clamping functions for integers in the same package.
- **Result**: **PASS**

### Bonus Check 16: `run/` package file count
- **Claimed**: 20+ files
- **Actual**: 17 production Go files (non-test, in `run/` excluding `journal/` subdirectory)
- **Result**: **FAIL** (overstated by 3 files). The 8,753 total lines exceed the claimed "7,500+" though, so the lines claim is correct.

### Bonus Check 17: Smell Distribution percentages (summary)
- **Claimed**: DRY Violations 31 (29%), Bloaters 28 (26%), Change Preventers 18 (17%), Dispensables 12 (11%), Couplers 8 (8%), SOLID Violations 6 (6%), Conditional Complexity 3 (3%)
- **Verification**: Sum = 31+28+18+12+8+6+3 = 106. Matches total finding count. Percentages: 31/106=29.2%, 28/106=26.4%, 18/106=17.0%, 12/106=11.3%, 8/106=7.5%, 6/106=5.7%, 3/106=2.8%. All round correctly.
- **Note**: The per-report smell distributions do not cross-add cleanly to these totals because individual reports use overlapping category labels and some findings span multiple categories. This is acceptable as an approximation.
- **Result**: **PASS**

---

## 3. Spot-Check Summary

| # | Claim | Result | Notes |
|---|-------|--------|-------|
| 1 | `execution.go` ~1,900 lines | **PASS** | Exact: 1,900 |
| 2 | `exec_flow.go` ~1,176 lines | **PASS** | Exact: 1,176 |
| 3 | `root.go` ~1,190 lines | **PASS** | Exact: 1,190 |
| 4 | `tool_call_format.go` ~733 lines | **PASS** | Actual: 732 |
| 5 | Duplicated `wrapTaskParseError` | **PASS** | Identical in both files |
| 6 | Duplicated `wrapReviewParseError` | **PASS** | Identical in both files |
| 7 | `prompt.ParseTaskFile` called from `tasks/store.go` | **PASS** | Lines 92 and 213 |
| 8 | `newPreparation` duplication with divergence | **PASS** | handlers.go drops `groups` field |
| 9 | `RuntimeFields` ~30 fields | **PASS** | Actual: 32 fields |
| 10 | `model.go` ~370 lines | **PASS** | Actual: 369 |
| 11 | `internal/version` never imported | **PASS** | Only used via ldflags |
| 12 | `kinds/session.go` duplicates `model/content.go` | **PASS** | Near-identical hierarchies confirmed |
| 13 | `commandState` 30+ fields | **PASS** | Actual: 44 fields |
| 14 | Dead `uiViewFailures` | **PASS** | Defined and matched but never set |
| 15 | `clampInt` vs `clamp` duplication | **PASS** | Both confirmed in `run/` |
| 16 | `run/` has 20+ files | **FAIL** | Actual: 17 production files |
| 17 | Smell distribution math | **PASS** | Sums and percentages are correct |

**Score: 16/17 spot-checks pass** (94% accuracy).

---

## 4. Contradictions and Overlaps Between Reports

### No Contradictions Found

All five reports describe code issues from consistent perspectives. No two reports flag the same code with conflicting assessments (e.g., one calling it P0 and another P1 for the same issue).

### Acknowledged Overlaps (correctly handled)

The summary's "Cross-Report Correlations" section identifies 5 systemic issues that span multiple reports. Each correlated finding is correctly listed as a distinct actionable item, not double-counted. Specifically:

1. **Content block type duplication**: G3-F4 (conversion code), G5-F01 (type hierarchy duplication), G2-F4 (decode function repetition) -- three different manifestations of the same architectural issue, correctly tracked as separate findings requiring separate fixes.

2. **Domain parsing in wrong package**: G4-F01 (prompt owns parsing), G4-F02/F03 (error wrappers duplicated as a consequence) -- root cause and symptoms, correctly separated.

3. **Config translation bloat**: G2-F2 (triple chain), G1-F3 (commandState 30+ fields), G1-F7 (buildConfig copy) -- related symptoms of the same structural issue, correctly separated because they require different fixes.

### Unacknowledged Minor Overlap

G3-F19 (provider report's `publicSessionUpdate`/`internalSessionUpdate` mirroring) and G3-F4 (content-block conversion duplication) are essentially the same issue described at different granularity levels. The report acknowledges this inline ("Related to F4 but at the session update level"), which is adequate. However, this could be seen as borderline double-counting within the same report. Both are counted toward the Agent & Run total of 25.

---

## 5. Corrections Needed

### Must Fix

1. **Provider & Public report header table**: Change P1 from 7 to 6, P2 from 8 to 9. The body contains 6 P1 findings and 9 P2 findings.

2. **Summary aggregate table**: Propagate the correction -- P1 total should be 31 (not 32), P2 total should be 39 (not 38). Grand total of 106 remains unchanged.

### Should Fix

3. **`run/` file count**: Change "20+ files" to "17 production files" across:
   - Summary line 1 ("God Package `run` (20+ files)")
   - Summary P0 table row G3-F1
   - Agent & Run report F1 title and body
   - Note: "7,500+ lines" is correct as a lower bound (actual is 8,753 lines); could be updated to "8,700+ lines" for precision.

4. **`version.go` line count**: Provider report F02 says "14-line"; actual is 13 lines. Minor.

### Optional

5. **Provider report F22 placement**: F22 (P2) appears after the P3 section (F17-F21), which breaks the severity-descending ordering convention. Moving it before the P3 block would improve readability.

6. **Summary G5-F01 description**: States "~900 lines" which is the combined total of both files (907), not the amount of duplicated code. More precise phrasing: "~450 lines duplicated across two files (~900 lines combined)".

---

## 6. Conclusion

The refactoring analysis is **reliable and actionable**. Out of 17 spot-checks against the actual codebase, 16 passed (94%). The one failure (file count in `run/`) is a modest overstatement (17 actual vs 20+ claimed) that does not change the God Package diagnosis. The P1/P2 severity miscount in the Provider report is the only math error, affecting 2 cells in the summary table without changing the grand total or the priority ordering of the top-10 recommendations.

All major architectural claims -- the God Package in `run/`, the misplaced parsing in `prompt/`, the content-block duplication between `model/` and `kinds/`, the triple config chain, and the `newPreparation` divergence -- are verified as accurate and correctly characterized.
