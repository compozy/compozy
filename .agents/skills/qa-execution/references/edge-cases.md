# Edge Cases

This file catalogs the **non-technical** edge cases real users hit. These are not unit-test edge cases (null inputs, integer overflow, malformed JSON) — those are unit/integration concerns. These are *user-behavior* edge cases that scripted tests miss because they live in the seams between intent and software.

Use during Step 4 of `qa-execution` (tours & edge probes). Pair each with a persona and one tour from the tour catalog (routed together with this file at Step 4 of the SKILL).

## Contents

- Why this catalog exists
- Catalog
  - Navigation edges
  - Form & input edges
  - Session & auth edges
  - Network & timing edges
  - Device & viewport edges
  - Locale & accessibility edges
  - Interrupt edges
  - Trust & recovery edges
- How to use this catalog
- What is NOT in scope here
- Sources

## Why this catalog exists

> "Try doing unconventional actions like rapidly double-clicking buttons or links to see if the application can handle the extra events gracefully. Using the browser back and refresh buttons in the middle of a multi-step workflow… entering form fields out of order… real users might try [these]." — Testlio, *Exploratory Testing 101*

A scripted test always submits a form once, in order, with valid data, from a single tab, on a stable network. A real user does none of those things consistently.

## Catalog

### Navigation edges

| Edge case | What to try | What it surfaces |
|---|---|---|
| Refresh during submit | Click submit, refresh before the response arrives | Double-submit, lost confirmation, ghost record |
| Back button after success | Reach a success screen, press back | Cached success state, re-fire of the action, broken breadcrumb |
| Back button mid-form | Fill half a form, press back, return | Lost data, stale state, validation re-applied unfairly |
| Forward button after back | Back → Forward → back → forward | URL state desync, broken history, infinite loop |
| Deep-link mid-flow | Bookmark step 3, visit it tomorrow | Missing prerequisites, auth-redirect loops, partial state |
| Close-and-reopen tab | Close the tab during a long action, reopen immediately | Lost progress vs successful background completion |
| Open the same URL in two tabs | Edit in both, save in one, refresh the other | Stale read, lost update, optimistic-lock failures |
| Visit a 404 URL | Type a malformed or removed URL | 404 UX, recovery links, search fallback |

### Form & input edges

| Edge case | What to try | What it surfaces |
|---|---|---|
| Double-click submit | Click submit twice within 200ms | Double submission, double-charge, duplicate record |
| Tab through form out of order | Click in field 5, then 2, then 8 | Hidden tab-index bugs, validation triggered at the wrong moment |
| Submit with autofill | Let browser autofill, submit immediately | Race between fill and validate, wrong-field fill |
| Paste from Word | Copy text with smart quotes, em-dashes, formatting | Hidden chars submitted, rendering breaks, sort-order surprises |
| Paste from Excel | Copy a cell with `\t` or newlines | Field-overflow into next field, broken validation |
| Paste a URL into a non-URL field | Paste `https://...` into a name field | Auto-linkification, length-validation surprises |
| Empty submit | Submit a form with no fields filled | Validation copy quality, error proximity, focus management |
| Browser auto-translate | Right-click → translate page mid-flow | DOM mutation breaks bindings, lost click handlers |
| Drag-and-drop file the wrong place | Drag a file into a non-drop area | Browser navigates to file URL (data loss for unsaved work) |
| Voice input | Use platform voice-to-text into a field | Trailing spaces, capitalization, punctuation oddities |

### Session & auth edges

| Edge case | What to try | What it surfaces |
|---|---|---|
| Session expiry mid-form | Start a long form, wait until session times out, submit | Lost data on redirect to login, confused post-login state |
| Login in another tab | Log in to a second account in tab B, return to tab A | Cross-tab session contamination, wrong-user actions |
| Logout in another tab | Click logout in tab B while tab A has open work | Tab A acts on a dead session, silent failures |
| Sign in twice rapidly | Click sign-in twice during the OAuth roundtrip | Stuck state, double-redirect, double-account-creation |
| Password manager autofill mismatch | Let 1Password fill a stale password | Login fails, but the password manager doesn't know |
| Cookie blocked | Visit with third-party cookies blocked | Auth flows that silently break (especially in Safari/iOS) |
| Cookies cleared mid-session | Clear cookies in a second tab; act in the first | Token loss, session corruption |

### Network & timing edges

| Edge case | What to try | What it surfaces |
|---|---|---|
| Slow 3G | Throttle the network in devtools | Optimistic UI showing success on failure, spinners with no timeout |
| Flaky network (drop & resume) | Toggle offline mid-request | Lost data, no retry, no offline indicator |
| Long-running request | Trigger an action that takes >30s | Timeout UX, "still working" indicator, cancellable? |
| Submit while offline | Disable network, submit | Queued vs lost vs error UX |
| Server clock skew | Visit when your device clock is wrong | Token-validity bugs, scheduling-off-by-N bugs |
| Daylight-saving transition | Schedule something across a DST boundary | Off-by-one-hour bugs in dates, alarms, billing windows |

### Device & viewport edges

| Edge case | What to try | What it surfaces |
|---|---|---|
| Rotate device | Rotate portrait → landscape mid-flow | Layout reflow loses focus/state, modal jumps |
| Tiny viewport (320px) | Resize the browser to 320×568 | Sticky elements covering content, untappable buttons |
| Very large viewport (4K) | Test at 3840×2160 | Whitespace explosion, layout drifting to corners |
| Reduced motion enabled | OS setting "reduce motion" | Critical animations skipped, lost feedback |
| High contrast / dark mode | OS dark mode | Hardcoded color bugs, invisible UI elements |
| OS zoom 200% | System-level zoom | Layout breaks, overlapping elements, scrollbar surprises |
| Browser zoom 50% / 200% | Ctrl+- / Ctrl++ | Layout broken at non-100% zoom |

### Locale & accessibility edges

| Edge case | What to try | What it surfaces |
|---|---|---|
| Long-translation overflow | Switch to de-DE | German compounds break button widths |
| RTL layout | Switch to ar-EG | Mirrored layout, icons pointing the wrong way |
| Non-Latin characters | Enter `日本語` / `العربية` / `русский` | Encoding bugs, font fallbacks, search/sort failures |
| Screen-reader navigation | Use VoiceOver / NVDA / TalkBack | Missing labels, focus traps, dynamic content not announced |
| Keyboard-only navigation | Unplug the mouse | Focus order, focus-visible, skip links, modal escape |
| Voice control | Use macOS / Windows voice control | Buttons without accessible names |

### Interrupt edges

| Edge case | What to try | What it surfaces |
|---|---|---|
| Phone lock mid-upload | Start an upload on mobile, lock the screen | Suspended request, silent failure |
| App-switch during checkout | Open another app during the 3DS step | Lost auth, broken redirect chain |
| Phone call mid-form | Receive an incoming call | Mobile browser repaint loses focus/state |
| Battery saver / low-power mode | Enable iOS low-power mode | Background timers slow, requests throttled |
| Push notification mid-action | Receive a push during a critical action | Tap-target shift causing wrong-button press |

### Trust & recovery edges

| Edge case | What to try | What it surfaces |
|---|---|---|
| Return after error | Hit a 500, refresh in 1 minute, then 10, then 1 day | Stale error page, "we're sorry" loops, fixed-but-cached |
| Bookmark of a moved URL | Visit an old marketing link with new IA | 404 vs redirect quality |
| Forward someone an internal URL | Send a private URL to a logged-out user | Leak of preview content, auth-redirect quality |
| Copy a magic link, open in different browser | Cross-browser auth link | Token-bound-to-browser bugs |

## How to use this catalog

1. After picking the persona and tour for a charter, pick 5-10 relevant edge cases from the catalog above. Match the edge case to the surface — onboarding doesn't need a "schedule across DST" test; scheduling does.
2. Record which edge cases were attempted in the charter debrief — attempted-and-clean is evidence too.
3. For each finding, file via the global bug registry (routed at Step 6 of the SKILL) with `Persona Affected:` and `Journey Step:` filled in.
4. Don't try every edge case in one box. The time-box governs.

## What is NOT in scope here

These belong elsewhere:

- **Unit-level boundary tests** (null, empty array, integer overflow) — unit tests.
- **SQL injection / XSS / CSRF** — security review / SAST.
- **Load and concurrent-user stress** — load testing tools.
- **Race conditions in code** — integration tests, fuzzing.
- **Build-time errors** — CI gate.

If you find one of those during a user-QA session, file the bug and recommend the right gate, but do not attempt to verify it from inside the QA session.

## Sources

- Testlio — *Exploratory Testing 101: Going Off-Script*.
- Sahipro — *Exploratory Testing Best Practices*.
- Thoughtworks — *10 tips for an Agile QA mindset, Tip 3 (corner cases)*: concurrent users, multiple file uploads, background processes.
- Thoughtworks — *10 tips for an Agile QA mindset, Tip 8*: don't rely on incognito or cleared cache; users don't.
