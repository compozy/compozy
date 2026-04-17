# Regression Testing Guide

Comprehensive guide to regression testing strategies and execution. When executing regression suites against a live repository, use the `qa-execution` companion skill which follows this same execution order and priority scheme.

---

## What is Regression Testing?

**Definition:** Re-testing existing functionality to ensure new changes haven't broken anything.

**When to run:**
- Before every release
- After bug fixes
- After new features
- After refactoring
- Weekly/nightly builds

---

## Regression Test Suite Structure

### 1. Smoke Test Suite (15-30 min)

**Purpose:** Quick sanity check

**When:** Daily, before detailed testing

**Coverage:**
- Critical user paths
- Core functionality
- System health checks
- Build stability
- Existing E2E smoke coverage when the repository supports it

**Example Smoke Suite:**
```
SMOKE-001: User can login
SMOKE-002: User can navigate to main features
SMOKE-003: Critical API endpoints respond
SMOKE-004: Database connectivity works
SMOKE-005: User can complete primary action
SMOKE-006: User can logout
```

### 2. Full Regression Suite (2-4 hours)

**Purpose:** Comprehensive validation

**When:** Before releases, weekly

**Coverage:**
- All functional test cases
- Integration scenarios
- UI validation
- Data integrity
- Security checks
- Automated E2E coverage for critical public flows when the repository supports it

### 3. Targeted Regression (30-60 min)

**Purpose:** Test impacted areas

**When:** After specific changes

**Coverage:**
- Modified feature area
- Related components
- Integration points
- Dependent functionality

---

## Building a Regression Suite

### Step 1: Identify Critical Paths

**Questions:**
- What can users absolutely NOT live without?
- What generates revenue?
- What handles sensitive data?
- What's used most frequently?

**Example Critical Paths:**
- User authentication
- Payment processing
- Data submission
- Report generation
- Core business logic

### Step 2: Prioritize Test Cases

**P0 (Must Run):**
- Business-critical functionality
- Security-related tests
- Data integrity checks
- Revenue-impacting features
- Public flows that should already be covered by E2E when the harness exists

**P1 (Should Run):**
- Major features
- Common user flows
- Integration points
- Performance checks
- Bug-prone public regressions that should become E2E candidates when the harness exists

**P2 (Nice to Run):**
- Minor features
- Edge cases
- UI polish
- Optional functionality

### Step 3: Group by Feature Area

```
Authentication & Authorization
├─ Login/Logout
├─ Password reset
├─ Session management
└─ Permissions

Payment Processing
├─ Checkout flow
├─ Payment methods
├─ Refunds
└─ Receipt generation

User Management
├─ Profile updates
├─ Preferences
├─ Account settings
└─ Data export
```

### Step 4: Tag Automation Candidates

When the repository already supports E2E, mark these as automation candidates:

- Changed P0 and P1 public flows
- Release-critical smoke checks
- Bug-driven regressions that were reproduced through browser, HTTP, CLI, or worker entrypoints

Keep cases manual-only when they are exploratory, usability-focused, or primarily visual judgment work.

---

## Regression Suite Examples

### E-commerce Regression Suite

**Smoke Tests (20 min):**
1. Homepage loads
2. User can login
3. Product search works
4. Add to cart functions
5. Checkout accessible
6. Payment gateway responds

**Full Regression (3 hours):**

**User Account (30 min):**
- Registration
- Login/Logout
- Password reset
- Profile updates
- Address management

**Product Catalog (45 min):**
- Browse categories
- Search functionality
- Filters and sorting
- Product details
- Image zoom
- Reviews display

**Shopping Cart (30 min):**
- Add items
- Update quantities
- Remove items
- Apply discounts
- Save for later
- Cart persistence

**Checkout & Payment (45 min):**
- Guest checkout
- Registered user checkout
- Multiple addresses
- Payment methods
- Order confirmation
- Email notifications

**Order Management (30 min):**
- Order history
- Order tracking
- Cancellations
- Returns/Refunds
- Reorders

---

## Execution Strategy

### Test Execution Order

**1. Smoke first**
- If smoke fails → stop, fix build
- If smoke passes → proceed to full regression
- If the harness exists, include existing E2E smoke coverage in this first pass

**2. P0 tests next**
- Critical functionality
- Must pass before proceeding

**3. P1 then P2**
- Complete remaining tests
- Track failures

**4. Exploratory**
- Unscripted testing
- Find unexpected issues

### Pass/Fail Criteria

**PASS:**
- All P0 tests pass
- 90%+ P1 tests pass
- No critical bugs open
- Performance acceptable

**FAIL (Block Release):**
- Any P0 test fails
- Critical bug discovered
- Security vulnerability
- Data loss scenario

**CONDITIONAL PASS:**
- P1 failures with workarounds
- Known issues documented
- Fix plan in place

---

## Regression Test Management

### Test Suite Maintenance

**Monthly Review:**
- Remove obsolete tests
- Update changed functionality
- Add new critical paths
- Optimize slow tests

**After Each Release:**
- Update test data
- Fix broken tests
- Add regression for bugs found
- Add or update E2E coverage for public regressions when the harness exists
- Document changes

### Automation Considerations

**Good Candidates for Automation:**
- Stable, repetitive tests
- Smoke tests
- API tests
- Data validation
- Cross-browser checks
- Changed P0 and P1 public flows when the harness already exists
- Bug reproductions that should never regress again

**Keep Manual:**
- Exploratory testing
- Usability evaluation
- Visual design validation
- Complex user scenarios

---

## Regression Test Execution Report

This report format aligns with the verification report generated by `qa-execution`. When both skills are used, the `qa-execution` verification report includes TEST CASE COVERAGE and AUTOMATED COVERAGE sections that cross-reference TC-IDs and automation follow-up from this regression suite.

```markdown
# Regression Test Report: Release 2.5.0

**Date:** 2024-01-15
**Build:** v2.5.0-rc1
**Tester:** QA Team
**Environment:** Staging

## Summary

| Suite | Total | Pass | Fail | Blocked | Pass Rate |
|-------|-------|------|------|---------|-----------|
| Smoke | 10 | 10 | 0 | 0 | 100% |
| P0 Critical | 25 | 23 | 2 | 0 | 92% |
| P1 High | 50 | 47 | 2 | 1 | 94% |
| P2 Medium | 40 | 38 | 1 | 1 | 95% |
| **TOTAL** | **125** | **118** | **5** | **2** | **94%** |

## Automated Coverage

- Support detected: Yes (`npm run test:e2e`)
- Specs added or updated: `e2e/checkout.spec.ts`
- Blocked flows: None

## Critical Failures (P0)

### BUG-001: Payment processing fails for Visa
- **Test:** TC-FUNC-001
- **Severity:** Critical
- **Priority:** P0
- **Impact:** Blocks 40% of transactions
- **Status:** Open

### BUG-002: User session expires prematurely
- **Test:** TC-FUNC-045
- **Severity:** High
- **Priority:** P1
- **Impact:** Users logged out unexpectedly
- **Status:** Open

## Recommendation

**Verdict:** CONDITIONAL PASS
- Fix BUG-001 (payment) before release
- BUG-002 acceptable with documented workaround
- Retest after fixes
- Final regression run before production deployment

## Risks

- Payment issue could impact revenue
- Session bug may frustrate users
- Limited time before release deadline

## Next Steps

1. Fix BUG-001 by EOD
2. Retest payment flow
3. Document session workaround
4. Final smoke test before release
```

---

## Common Pitfalls

**❌ Don't:**
- Run same tests without updating
- Skip regression "to save time"
- Ignore failures in low-priority tests
- Test only happy paths
- Forget to update test data
- Run regression once and forget

**✅ Do:**
- Maintain suite regularly
- Run regression consistently
- Investigate all failures
- Include edge cases
- Keep test data fresh
- Automate repetitive tests
- Add or update E2E coverage for critical public regressions when the harness exists

---

## Regression Checklist

**Before Execution:**
- [ ] Test environment ready
- [ ] Build deployed
- [ ] Test data prepared
- [ ] Previous bugs verified fixed
- [ ] Test suite reviewed/updated

**During Execution:**
- [ ] Follow test execution order
- [ ] Document all failures
- [ ] Screenshot/record issues
- [ ] Note unexpected behavior
- [ ] Track blockers

**After Execution:**
- [ ] Compile results
- [ ] File new bugs
- [ ] Update test cases
- [ ] Update automation status for P0, P1, and bug-driven public regressions
- [ ] Report to stakeholders
- [ ] Archive artifacts

---

## Quick Reference

| Suite Type | Duration | Frequency | Coverage |
|------------|----------|-----------|----------|
| Smoke | 15-30 min | Daily | Critical paths |
| Targeted | 30-60 min | Per change | Affected areas |
| Full | 2-4 hours | Weekly/Release | Comprehensive |
| Sanity | 10-15 min | After hotfix | Quick validation |

**Remember:** Regression testing is insurance against breaking existing functionality.
