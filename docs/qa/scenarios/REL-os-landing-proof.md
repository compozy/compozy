---
id: REL-os-landing-proof
area: REL
title: Judge the integrated OS claim from the public landing page
persona: Cora
journey: J-evaluate-compozy-beta
expected: The locked hero pair, static shell capture, six ordered sections, and sourced proof make the product claim understandable without a generic dashboard or unsupported competitor assertion.
entry_points: Local site render (canonical origin declaration: https://compozy.com)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-07-29-os-shell-bento/desktop.png; docs/qa/evidence/2026-07-29-os-shell-bento/mobile.png
last_report: docs/qa/reports/2026-07-29-os-shell-bento.md
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
capture (`/images/hero/os-shell-capture-v1.png`) rendered as a 3D pitched/yawed window that follows
the pointer (static pose under reduced motion and touch), bleeding past the right edge on desktop,
and the locked headline now sets in the display serif, matching the deck cover. Copy, CTAs, signal
tiles, and section order are untouched. Status stays untested pending the same fresh landing walk.
