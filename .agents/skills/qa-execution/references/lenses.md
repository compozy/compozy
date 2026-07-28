# Experiential Lenses

Six qualities a real user *feels* that no single feature owns: usability, accessibility, perceived performance, compatibility, error recoverability, production parity. A dogfooder holds these lenses **during** journey walks, then runs one dedicated lens pass over the two widest journeys (Step 5) — never as a separate suite of cases.

This pass is intentionally lightweight — full audits (WCAG conformance, Lighthouse budgets) belong to dedicated tooling, indexed from the automation backlog when needed.

## Contents

- 1. Usability (Nielsen short list)
- 2. Accessibility (quick check)
- 3. Perceived performance
- 4. Compatibility
- 5. Error recoverability
- 6. Production parity
- Running the lens pass (45-minute box)
- Anti-patterns
- Sources

## 1. Usability — Nielsen short list

Walk the surface and answer each, citing the journey step:

- [ ] **Visibility of system status** — feedback within 1 second of every action?
- [ ] **Match with the real world** — copy in the user's language, not the system's? (No "Entity created", no "Request 200 OK".)
- [ ] **User control and freedom** — undo, cancel, or back out of every committed action?
- [ ] **Consistency** — same noun for the same thing; same icon for the same action?
- [ ] **Error prevention** — confirmation on irreversible actions; inline validation before submit?
- [ ] **Recognition over recall** — nothing to remember from a previous screen?
- [ ] **Flexibility for power users** — shortcuts for repeated actions?
- [ ] **Aesthetic and minimalist** — every word and button earning its place?
- [ ] **Help users recover** — plain-language errors with a specific next step?
- [ ] **Help and documentation** — a path to help from the failing state?

Unmet heuristics are usually `Friction` or `Trust-Damage` findings.

## 2. Accessibility — quick check

Quick check, not a conformance audit.

**Keyboard:** every interactive element reachable with Tab; tab order matches visual order; visible focus indicator; Escape closes modals; no keyboard trap.

**Screen reader** (VoiceOver / NVDA): logical heading hierarchy (one `<h1>`); form fields have labels (not just placeholders); buttons have accessible names; images have alt text or are marked decorative; toasts/modals announced; status messages use `aria-live`.

**Visual:** color is never the only signal; text contrast ≥ 4.5:1 (3:1 large text); UI holds at 200% zoom and OS text scaling; reduce-motion respected.

Violations are `Trust-Damage` unless they block a core journey (then `Blocks-Completion`).

## 3. Perceived performance

Measure what the user feels, not what synthetic tools report:

| Observable | Target | When it fails |
|---|---|---|
| First meaningful paint | <2s wifi, <4s 3G | Blank screen; layout shifts after 2s |
| Time to interactive | <3s wifi, <6s 3G | Click during load is ignored |
| Spinner threshold | Appears within 100ms for actions >300ms | Action looks dead before the spinner shows |
| Button feedback | State change within 50ms of click | User double-clicks because nothing happened |
| Optimistic UI | Must reconcile correctly on failure | "Saved" followed by silent loss |
| Long-task UX | >2s actions show progress / cancel | User abandons, assumes it's stuck |

Failures are `Friction`, promoted to `Blocks-Completion` when the perceived stall causes abandonment.

## 4. Compatibility

Smoke the changed surface across the minimum matrix:

| Layer | Minimum |
|---|---|
| Browser | Latest Chrome + Safari + Firefox |
| Mobile | Safari on iPhone (latest), Chrome on Android (latest) |
| Viewport | 1280, 768, 375 |
| OS dark mode | Light AND dark |
| Reduced motion | On AND off |

Layout/CSS changes make viewport coverage mandatory; form changes make mobile Safari mandatory (autofill diverges). Severity by user impact, not by which browser.

## 5. Error recoverability

For every failure path met during execution, the recovery experience must:

- [ ] Explain in plain language (no stack traces, no bare error codes).
- [ ] Offer a specific next step (retry, go back, contact support).
- [ ] Preserve user input (no "fill the whole form again").
- [ ] Say whether the failure is transient or permanent.
- [ ] For data-loss situations, name what was lost.

A failure path without recoverable UX is `Trust-Damage` at minimum, often `Data-Loss`.

## 6. Production parity

The session itself must resemble reality, or its verdicts don't generalize:

- [ ] Build matches the production deploy artifact.
- [ ] Third-party cookies enabled (the real default).
- [ ] Normal browser profile, NOT incognito (cache, autofill, extensions differ).
- [ ] Realistic extension set (an ad blocker, a password manager) — extension injection causes real bugs.
- [ ] The real auth path (SSO, magic link), not a test bypass.
- [ ] Real backend services, not local mocks.
- [ ] Worst-case realistic network tested (Slow 3G), not just office wifi.

Any deviation is recorded in the report — parity gaps qualify every verdict in the run.

## Running the lens pass

After journey walks and tours (Steps 3-4):

1. Pick **2 journeys** that exercise the largest changed surface.
2. Re-walk them as a lens audit, not a journey verification.
3. At each step, ask the six lenses; mark `pass` / `friction` / `fail` per lens.
4. File one finding per failure via the registry; record the pass in the report's lens section.
5. **45 minutes total.** Anything unfinished becomes a follow-up charter — fatigue produces false positives.

## Anti-patterns

- **Full conformance audit in the QA window** — the deep audit is dedicated tooling; queue it, don't improvise it.
- **Skipping lenses because "the feature works"** — working and feeling right are different claims.
- **Lens pass before journey walks** — lenses are a re-read of real flows; without a flow they produce shallow findings.
- **Lens cases in the tracker** — lenses qualify journey scenarios; they don't get their own scenario rows per category.
- **Treating this as security testing** — security is its own discipline; the lens concern is user trust perception, not vulnerability scanning.

## Sources

- Nielsen Norman Group — *10 Usability Heuristics for User Interface Design*.
- W3C — *WCAG 2.1 Quick Reference* (AA quick-check items).
- Thoughtworks — *10 tips for an Agile QA mindset*, Tips 7-9 (CFRs, incognito/cache, production parity).
