---
id: ET-web-ui-resilience
area: ET
title: Preserve truthful and accessible async UI across Web systems
persona: Bruno
journey:
expected: Known-geometry first loads render stable Skeleton geometry for the selected rows or cards view; action-pending feedback remains local while conflicting mutations are serialized; controls backed by unavailable data stay disabled; custom filter menus expose each supported comparison exactly once; schedule previews refresh on their second or minute boundary, including after invalid input becomes valid; focus remains visible on revealed actions and disclosures; operational errors do not invalidate unrelated form fields; errors are announced; and motion is interruptible, composited, and reduced-motion safe.
entry_points: Web Agents, Bridges, Jobs, Triggers, Loops, Marketplace, Network, Notifications, Onboarding, Sandbox, Scheduler, Settings, Tasks, Vault, and Session surfaces; shared Filters and Stepper; Web and UI Storybook
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /tmp/agh-ui-screenshot.EdZjtp/evidence; /tmp/agh-ui-screenshot.batch-d.X6X6Rq/out
last_report:
overlaps: TA-039; NB-045; ET-web-loop-editor-topbar; ET-web-marketplace-landing-browse; ET-window-manager-layout-gestures; ET-web-settings-hooks; ET-web-vault-opendesign-listing; RT-021
---

Added by the 2026-07-21 full frontend systems audit. Flag only: the next QA cycle owns live
keyboard, filters, loading, mutation-race, error-announcement, reduced-motion, and responsive retesting.
