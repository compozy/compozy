# BUG-<num>: <short-title>

**Severity:** Critical | High | Medium | Low
**Priority:** P0 | P1 | P2 | P3
**Type:** Functional | UI | Performance | Security | Data | Crash
**Status:** Open

## Environment

- **Build:** <version or commit>
- **OS:** <operating system if relevant>
- **Browser:** <browser and version if Web UI>
- **URL:** <page or endpoint where bug occurs>

## Summary

<Describe the observable failure in one short paragraph.>

## Reproduction

```bash
<exact command or sequence>
```

Observed before the fix:

- <observable result>

## Expected

<Describe the correct behavior.>

## Root cause

<Describe the actual source of the failure, not the symptom.>

## Fix

<Describe the production change that fixed the root cause.>

## Verification

- <narrow reproduction rerun>
- <broader regression or full gate rerun>

## Impact

- **Users Affected:** <all / subset / specific role>
- **Frequency:** <always / sometimes / rarely>
- **Workaround:** <describe or "none">

## Automation Follow-up

- **Required:** Yes | No
- **Status:** Added | Pending | Blocked | N/A
- **Spec / Command:** <path, suite, or command>
- **Notes:** <rationale or blocker>

## Related

- Test Case: <TC-ID if applicable>
- Figma Design: <URL if UI bug>
