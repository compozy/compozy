---
provider: manual
pr: 6
round: 4
round_created_at: 2026-07-14T02:01:21Z
status: resolved
file: extensions/cy-improve-architecture/evals/skill_e2e_test.go
line: 371
severity: medium
author: claude-code
provider_ref:
---

# Issue 002: Healthy-target E2E accepts fabricated candidates

## Review Comment

For the Go healthy fixture, `assertArtifacts` confirms only one Markdown
sentence and then returns. It never checks that the parsed area has zero
entries, that the map contains the required dated no-opportunities comment, or
that the Markdown and HTML contain no candidate anchors, candidate cards, or
top-pick CTA.

An audit can therefore claim `Healthy target` while publishing `deep` or
`seam` guidance and a fabricated candidate recommendation, and this opt-in
evaluation still passes. That leaves the zero-candidate behavior in E2E-010,
E2E-022, and E2E-025 unverified.

Add a dedicated healthy-report assertion before returning: require
`len(area.Entries) == 0`, the dated no-opportunities map comment, Markdown
`No candidates.` with no candidate anchors, and an HTML healthy outcome with
no `candidate-` articles or candidate-targeting top-pick link. Cover both a
valid healthy pair and each rejected fabricated-candidate variant.

## Triage

- Decision: `VALID`
- Root cause: `assertArtifacts` returns from its healthy-target branch after checking only the Markdown healthy sentence. The `archmap.Parse` result is already available but its empty `Entries` slice is not checked; the raw depth map is also discarded even though `archmap.Parse` intentionally ignores comment lines. Markdown candidate anchors, HTML candidate cards, and the candidate-targeting top-pick CTA therefore remain unchecked for the healthy fixture.
- Fix approach: add a dedicated healthy-report validator invoked before the healthy branch returns. It will require zero parsed map entries, the healthy section's dated `# no deepening opportunities as of <YYYY-MM-DD>` comment, Markdown `No candidates.` with no candidate anchor references, and an HTML healthy outcome with neither candidate cards nor a candidate top-pick CTA. Add table-driven tests for the valid healthy artifact set and each fabricated-candidate variant.
- Verification: `go test -count=1 ./extensions/cy-improve-architecture/evals` passed. `make verify` passed with 4,577 Go tests (5 opt-in/environmental skips), a successful Go build, and 5 frontend E2E tests.
