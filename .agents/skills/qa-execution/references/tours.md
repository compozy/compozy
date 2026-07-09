# Tours

A **tour** is a thematic lens for exploration. It frames the session around a specific failure pattern users actually hit. Picking the right tour for a charter (charters are planned by the `qa-report` skill) is the single biggest predictor of whether the session finds real bugs.

This catalog is the canonical source — the only home of tour definitions across both QA skills.

## Contents

- How to use this file (one tour per charter)
- Tour catalog (Feature, Money, Garbage, Back-Button, Multi-Tab, Network, Locale, Paste, Autofill, Interrupt)
- Picking the tour for a charter (surface-to-tour matrix)
- Anti-patterns
- Sources

## How to use this file

Every charter binds to **exactly one tour**. Pick by matching the surface to the tour's *"when to use"* clause. During the session, the tour is the lens — every interaction is asked: *"would this matter for this tour's theme?"*

## Tour catalog

### Feature Tour

- **Theme:** does the headline functionality work end-to-end as advertised?
- **When to use:** new features, smoke after a deploy, sanity-checking a recent change.
- **Mission verbs:** "use", "complete", "achieve".
- **Off-script actions:** none — this is the closest tour to scripted testing. Walk the documented happy path; verify every promise made on the landing/marketing surface.
- **What to look for:** missing functionality, broken promises, mismatch between docs/UI and behavior.
- **Sample evidence:** screenshot of the goal-achieved state, with timestamp.

### Money Tour

- **Theme:** the paths that move money or revenue.
- **When to use:** checkout, subscription change, refund, invoice, payout, currency-handling.
- **Mission verbs:** "pay", "refund", "upgrade", "convert".
- **Off-script actions:** abandon mid-payment; switch currency; apply expired coupon; apply more coupons than allowed; pay with declining card; pay with 3DS-required card; receive partial refund; cancel after charge.
- **What to look for:** ghost charges, double charges, lost discount codes, currency rounding bugs, receipt/invoice mismatches, tax miscalculation, dunning email failures.
- **Sample evidence:** screenshot of payment success + receipt + bank statement view (if accessible); for failures, screenshot of error + the user-visible state.

### Garbage Tour

- **Theme:** can the product survive being mistreated?
- **When to use:** any input-heavy surface (forms, file upload, search, bulk-edit, comments).
- **Mission verbs:** "abuse", "stress", "corrupt".
- **Off-script actions:** paste 10,000-char strings; paste rich-text with hidden formatting; upload empty file / 0-byte / 1GB / wrong-mime / executable; rapidly double-click submit; submit with browser autofill garbage; type emoji / RTL text / null chars / SQL keywords (not as a security test — as a *realistic copy-paste from external app* test); spam undo/redo until state breaks.
- **What to look for:** silent data loss, partial writes, frozen UI, server 500s with no user feedback, autosave corruption.
- **Sample evidence:** screenshot of the corrupted/missing state, plus the input that triggered it.

### Back-Button Tour

- **Theme:** the browser back button is a user expectation, not a developer feature.
- **When to use:** multi-step flows, wizards, deep-link navigation, modal-heavy UIs.
- **Mission verbs:** "go back", "return", "undo navigation".
- **Off-script actions:** at every step, press back. Then forward. Then back again. Open the previous step in a new tab. Refresh during the back transition. Press back from a modal. Press back from a success screen. Bookmark a mid-flow URL and visit it tomorrow.
- **What to look for:** state loss, ghost state ("you already did this" when you didn't), modal stuck open, form fields cleared, payment re-triggered, partial commits, broken breadcrumbs.
- **Sample evidence:** screenshot of state-after-back, with the URL bar visible and the previous step URL also captured.

### Multi-Tab Tour

- **Theme:** real users keep many tabs open and switch between them constantly.
- **When to use:** any surface where state is shared across sessions or tabs.
- **Mission verbs:** "open another tab", "duplicate session", "race".
- **Off-script actions:** open the same page in two tabs; edit different fields in each; submit both; in tab A start an action that takes time, then in tab B do the inverse; log out in tab A while tab B has an open form; refresh tab B after the logout; open the same record in two tabs and delete it in one.
- **What to look for:** stale state, lost-update bugs, "the resource has changed since you opened it" errors that prevent recovery, session-leak between tabs, broken auth handoff.
- **Sample evidence:** screenshots of both tabs side by side at the moment of divergence.

### Network Tour

- **Theme:** real users don't have wifi-fast.
- **When to use:** anything that does I/O — which is anything user-facing.
- **Mission verbs:** "stall", "drop", "throttle".
- **Off-script actions:** throttle to Slow 3G in devtools; cut the network mid-submit; restore network — does the request retry, succeed, or silently fail; load the page on a flaky connection; refresh during a long load; submit twice when the first submit looks stuck.
- **What to look for:** infinite spinners with no timeout, optimistic UI showing success when the request failed, double-submits, lost data on network blip, no offline indicator, retries that double-charge.
- **Sample evidence:** Network panel screenshot showing the throttle profile + UI screenshot at failure.

### Locale Tour

- **Theme:** the world is not en-US.
- **When to use:** any surface that renders text, numbers, dates, or currency.
- **Mission verbs:** "translate", "format", "switch locale".
- **Off-script actions:** switch the browser to pt-BR / de-DE / ar-EG / ja-JP / zh-CN; visit with a non-default timezone (set device to Sydney or Anchorage); enter dates in DD/MM/YYYY when the form expects MM/DD/YYYY; enter prices with comma decimal; observe right-to-left rendering; observe long German compounds breaking layout; observe Japanese character widths.
- **What to look for:** untranslated strings, hardcoded en-US dates, currency symbols missing, layout breaks with long translations, RTL text mirroring failures, timezone-off-by-N bugs in scheduling/billing.
- **Sample evidence:** screenshot with locale and timezone visible.

### Paste Tour

- **Theme:** real users paste from somewhere else, usually with formatting they didn't see.
- **When to use:** any text input that accepts user prose or structured data.
- **Mission verbs:** "paste", "import", "transfer".
- **Off-script actions:** paste from Word (smart quotes, em-dashes, hidden styles); paste from Excel (tabs, line breaks); paste from a markdown editor (code fences, links); paste a URL into a non-URL field; paste an image into a text field; paste with browser autofill firing simultaneously.
- **What to look for:** smart-quotes rendering as `&rdquo;`, line-break loss, hidden formatting submitted to the server, validation-passing input that breaks downstream consumers (CSV export, email render).
- **Sample evidence:** screenshot of the visible pasted state + the raw stored value via a user-reachable read path (export, re-open, email render).

### Autofill Tour

- **Theme:** browsers auto-fill in ways developers don't see locally.
- **When to use:** any form, especially auth / checkout / settings.
- **Mission verbs:** "let the browser do it", "save credentials", "auto-suggest".
- **Off-script actions:** save a test password in the browser; visit the form fresh; let autofill populate; submit. Save a credit card; let it autofill; submit. Use Apple Pay / Google Pay autofill on mobile. Use a password manager (1Password, Bitwarden) and let it fill.
- **What to look for:** autofill into the wrong field; double-fill; readonly-bypass via autofill; submit-before-validation race; saved-card UI not matching autofilled data; CVC field cleared after autofill.
- **Sample evidence:** screenshot of the form post-autofill, before submit.

### Interrupt Tour

- **Theme:** real life interrupts users.
- **When to use:** long flows (forms, wizards, uploads), mobile especially.
- **Mission verbs:** "interrupt", "background", "abandon-and-resume".
- **Off-script actions:** start a flow; lock the phone for 5 minutes; unlock and continue. Start an upload; switch apps; return. Start a form; receive a phone call; finish the call; finish the form. Start checkout; close the tab; reopen the link 3 days later.
- **What to look for:** session expiry mid-flow with no recovery, lost form state on app switch, upload that silently failed when the app was backgrounded, "resume from here" features that resume from the wrong place.
- **Sample evidence:** before-interrupt screenshot + after-interrupt screenshot + the interrupt cause noted in the debrief.

## Picking the tour for a charter

| Surface | First-choice tour | Second-choice tour |
|---|---|---|
| Onboarding / signup | Feature Tour | Network Tour |
| Checkout / payment | Money Tour | Network Tour |
| Settings | Back-Button Tour | Multi-Tab Tour |
| Bulk operations | Garbage Tour | Multi-Tab Tour |
| Forms (any) | Autofill Tour | Paste Tour |
| Search / filter | Garbage Tour | Locale Tour |
| File upload | Network Tour | Garbage Tour |
| Wizard / multi-step | Back-Button Tour | Interrupt Tour |
| Mobile-only flow | Interrupt Tour | Network Tour |
| International rollout | Locale Tour | Paste Tour |
| Recovery / error page | Back-Button Tour | Network Tour |

## Anti-patterns

- **Sampling all tours in one charter** — dilutes everything. One tour per box.
- **Tour-without-persona** — a tour is the *what*, a persona is the *who*. Charters need both.
- **Inventing a tour mid-session** — note the new pattern in the debrief and propose it for the catalog; don't pivot mid-box.
- **Treating a tour as a checklist** — the bullets are prompts, not requirements. The mission is to *find bugs in the theme*, not to execute every bullet.

## Sources

- Testlio — *Exploratory Testing 101: Going Off-Script* (rapid double-clicks, back/refresh mid-workflow, long strings, special chars, large files).
- Sahipro — *Types of Exploratory Testing*: scenario-based, strategy-based, freestyle.
- Thoughtworks — *10 tips for an Agile QA mindset, Tip 3 (corner cases)*: concurrent users, multiple uploads, background processes — the seed material for Multi-Tab, Garbage, and Interrupt tours.
- James Whittaker — *Exploratory Software Testing*: the original "tour" framing (FedEx, money, garbage, back-alley, supermodel). This catalog modernizes that taxonomy for current web/mobile QA.
