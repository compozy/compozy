---
id: REL-os-landing-proof
area: REL
title: Judge the integrated OS claim from the public landing page
persona: Cora
journey: J-evaluate-compozy-beta
expected: The locked hero pair, the ordered sections of the approved landing draft, and sourced proof (live provider and release counts, product captures, the comparison table) make the product claim understandable without a generic dashboard or unsupported competitor assertion.
entry_points: Local site render (canonical origin declaration: https://compozy.com)
qa_status: pass
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: .compozy/tasks/site-redesign/qa/R4/ET-site-landing/walk.jsonl; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/default-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/default-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/outline-1440.json
last_report: docs/qa/reports/2026-11-10-site-redesign-r4.md
overlaps: ET-compozy-public-brand-navigation; REL-beta-install-paths
---

QA impact 2026-07-27: Task 11 replaced the twelve-section landing and Remotion preview with the
OS-first six-slot argument, drawn wordmark, and static windows + task board + loop-run proof. Planning
flag only; the next QA cycle owns desktop/mobile browser evidence.

QA impact 2026-07-27: the landing comparison and final CTA now frame Compozy as the operating
system beneath an agent software factory. The next QA cycle owns copy, source, and responsive-layout
verification.

QA impact 2026-07-29: the Runtime bento became the OS Shell proof, with the `True OS. Every window
managed.` claim and a responsive 4:3 managed-window illustration. Cora's local desktop and mobile
Feature Tour confirmed that the proof remains readable, truthfully visualizes managed windows, and
does not add unsupported controls or runtime claims.

QA impact 2026-08-11: the landing repositioned to the compression canon (COPY.md §2 rewrite). Hero
pair is now "The system around the agent, already built." + the one-complete-environment subhead;
signal tiles became Create / Automate / Supervise / built-in providers; the comparison dropped named
rivals for a DIY-stack-vs-built-in table; the OS Shell bento claim is "Batteries included. Every
window managed."; the final CTA and features header follow the same canon. Sections order and assets
are untouched; the comparison table also changed shape (named-rival columns became DIY-vs-built-in
rows with adjusted column widths). Status reset to untested pending a fresh landing walk.

QA impact 2026-08-11: the hero visual replaced the Remotion chat player with the real OS shell
capture (`/images/hero/os-shell-capture-v2.png`) rendered as a 3D pitched/yawed window that follows
the pointer (static pose under reduced motion and touch), bleeding past the right edge on desktop,
and the locked headline now sets in the display serif, matching the deck cover. Copy, CTAs, signal
tiles, and section order are untouched. Status stays untested pending the same fresh landing walk.

2026-09-27 hero refresh: the landing uses the isolated-lab 2580×1654 capture with one queued Task and the two built-in Loops. The screenshot receipt belongs to the current retirement QA report.

QA impact 2026-10-07 (memory removal): the landing's Memory & Dream section was deleted, the bento is two cards, and the positioning and comparison copy no longer name memory (the Hero Lock is unchanged). Count the ordered sections from the rendered page when walking; do not expect a memory or Dream proof. Stale pass verdict reset to untested; historical evidence preserved; no QA session ran.

QA impact 2026-11-10 (site redesign): the landing was rebuilt from the approved landing draft on live numbers. The OS shell capture (`/images/hero/os-shell-capture-v2.png`) and the old hero background were deleted with the old landing sections; the hero still equals the COPY.md Hero Lock. Walk the claim on ET-site-landing; earlier notes above are history.

QA result 2026-11-10 (R4 walk on ET-site-landing): partial. The Hero Lock pair, the ordered sections of the approved landing draft and the live proof (providers, releases, captures, quotes) read as one product claim without a generic dashboard. Not verified: the comparison table still carries "to verify" cells and a literal `<date>` that wait on the owner; whether it makes an unsupported competitor assertion cannot be judged until that content lands.

QA result 2026-11-10 (comparison table filled, dev server at 1440 and 390, dark): pass. Every competitor cell carries a verdict taken from that product's public documentation, checked on November 10, 2026: Synara (trysynara.com/docs), T3 Code (github.com/pingdotgg/t3code docs), Hermes Agent (hermes-agent.nousresearch.com/docs) and OpenClaw (docs.openclaw.ai); each verdict's reason is recorded beside `COMPARE_VERDICTS` in `packages/site/components/landing/content.ts`, and the source page of every cell is in `.compozy/tasks/site-redesign/qa/R6/comparison/sources.md`. "Partial" is defined under the table; the footnote names the date and links each product's docs. The DIY stack column is a stated judgment, not a documented product. No "to verify" cell or `<date>` remains; the six CompozyOS checks settle; no overflow and no console error at either width (the table scrolls sideways at 390, as in the draft). Evidence: `.compozy/tasks/site-redesign/qa/R6/comparison/compare-{1440,390}.jpg`.
