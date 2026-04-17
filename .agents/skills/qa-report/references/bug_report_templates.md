# Bug Report Templates

Unified templates shared with `qa-execution`. All bug reports use the `BUG-<num>` ID scheme and follow the structure in `assets/issue-template.md`.

---

## Standard Bug Report

The canonical format — used for all bug types. See `assets/issue-template.md` for the exact template.

```markdown
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
<One paragraph describing the observable failure.>

## Reproduction
```bash
<exact command or sequence>
```
Observed before the fix:
- <observable result>

## Expected
<Correct behavior.>

## Root cause
<Source of the failure, not the symptom.>

## Fix
<Production change that fixed the root cause.>

## Verification
- <narrow reproduction rerun>
- <broader regression or full gate rerun>

## Impact
- **Users Affected:** <all / subset / specific role>
- **Frequency:** <always / sometimes / rarely>
- **Workaround:** <describe or "none">

## Related
- Test Case: <TC-ID if applicable>
- Figma Design: <URL if UI bug>
```

---

## UI/Visual Bug Variant

Extend the standard template with a design-vs-implementation comparison table when reporting Figma discrepancies.

```markdown
# BUG-<num>: [Component] Visual Mismatch

**Severity:** Medium
**Priority:** P2
**Type:** UI
**Status:** Open

## Environment
- **Build:** v2.5.0
- **Browser:** Chrome 120
- **URL:** /components/button

## Summary
Primary button background color and font weight do not match Figma design.

## Design vs Implementation

| Property | Figma (Expected) | Implementation (Actual) | Match |
|----------|-------------------|--------------------------|-------|
| Background | #0066FF | #0052CC | No |
| Font Size | 16px | 16px | Yes |
| Font Weight | 600 | 400 | No |
| Padding | 12px 24px | 12px 24px | Yes |
| Border Radius | 8px | 8px | Yes |

## Reproduction
1. Navigate to /components/button
2. Inspect primary button with DevTools
3. Compare computed styles against Figma specs

## Expected
Button matches Figma design at [Figma URL].

## Root cause
[To be filled after investigation]

## Fix
[To be filled after fix is applied]

## Verification
- [ ] Visual comparison with Figma after fix
- [ ] Check at viewports: 375px, 768px, 1280px

## Impact
- **Users Affected:** All
- **Frequency:** Always
- **Workaround:** None — visual inconsistency

## Related
- Test Case: TC-UI-001
- Figma Design: [URL to specific component]
```

---

## Performance Bug Variant

Extend the standard template with a metrics table for performance issues.

```markdown
# BUG-<num>: [Feature] Performance Degradation

**Severity:** High
**Priority:** P1
**Type:** Performance
**Status:** Open

## Environment
- **Build:** v2.5.0
- **URL:** /dashboard

## Summary
Dashboard page load time exceeds 8 seconds with 1000+ records.

## Metrics

| Metric | Target | Actual | Variance |
|--------|--------|--------|----------|
| Page Load | < 2s | 8s | +300% |
| API Response | < 200ms | 1500ms | +650% |
| Memory Usage | < 100MB | 450MB | +350% |

## Reproduction
1. Load /dashboard with 1000+ records in database
2. Measure page load time in DevTools Network tab
3. Observe slow rendering

## Expected
Page loads in under 2 seconds.

## Root cause
[To be filled after investigation]

## Fix
[To be filled after fix is applied]

## Verification
- [ ] Page load time under target threshold
- [ ] Memory usage within acceptable range

## Impact
- **Users Affected:** All users with large datasets
- **Frequency:** Always with 1000+ records
- **Workaround:** Pagination reduces visible records
```

---

## Severity Definitions

| Level | Criteria | Response Time | Examples |
|-------|----------|---------------|----------|
| **Critical** | System crash, data loss, security breach | Immediate | Payment fails, login broken, data exposed |
| **High** | Major feature broken, no workaround | < 24 hours | Search not working, checkout fails |
| **Medium** | Feature partial, workaround exists | < 1 week | Filter missing option, slow load |
| **Low** | Cosmetic, rare edge case | Next release | Typo, minor alignment, rare crash |

## Priority vs Severity Matrix

|  | Low Impact | Medium | High | Critical |
|--|-----------|--------|------|----------|
| **Rare** | P3 | P3 | P2 | P1 |
| **Sometimes** | P3 | P2 | P1 | P0 |
| **Often** | P2 | P1 | P0 | P0 |
| **Always** | P2 | P1 | P0 | P0 |

## Bug Title Best Practices

**Good titles:**
- "[Login] Password reset email not sent for valid email addresses"
- "[Checkout] Cart total shows $0 when discount code applied twice"
- "[Dashboard] Page crashes when loading more than 1000 records"

**Bad titles:**
- "Bug in login" (too vague)
- "It doesn't work" (no context)
- "Please fix ASAP!!!" (emotional, no information)
