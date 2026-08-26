---
id: SITE-terminal-docs-truth
area: SITE
title: Follow the published terminal documentation and have it hold up
persona: Lea
journey: J-learn-terminal-from-docs
expected: The Terminal section's six pages and the generated command-line reference describe the terminal the daemon actually runs — the tutorial works verbatim, the safety, journal, profile, and platform pages match observed behaviour, and no page promises a capability the runtime refuses.
entry_points: compozy.com /docs/terminal; /docs/terminal/tutorial; /docs/terminal/agents-and-safety; /docs/terminal/journal-and-recordings; /docs/terminal/profile-segmentation; /docs/terminal/platform-support; /docs/cli/terminal
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-compozy-official-skill-discovery; ET-site-docs-search-context
---

Planned by integrated-terminal task 09 for the public documentation pages shipped by task 06 and
amended by task 08, which no scenario owned. The official CompozyOS skill's terminal reference is
tracked on the older `ET-compozy-official-skill-discovery` rather than duplicated here. Task 10 owns
the walk, evidence, and verdict.

Walk:

1. Open the Terminal section and confirm its navigation lists exactly the published pages, each
   reachable and titled as the section promises.
2. Run the tutorial verbatim against a real runtime, substituting only the workspace it tells you to
   substitute; confirm every command runs as printed and every documented result shape matches.
3. Check the agents-and-safety page against behaviour: the approval tiers, the per-terminal typing
   grant, and the instruction that terminal output is data and not authority.
4. Check the journal-and-recordings page against behaviour: what is always recorded, what is only kept
   while the terminal lives, what recording opt-in retains, and for how long.
5. Check the profile-segmentation page against behaviour: ownership at creation, what a profile switch
   hides, and what archiving closes versus keeps readable.
6. Check the platform-support page against behaviour on the workspace you are using, and confirm it
   states the execute-only path before a reader could hit a failure.
7. Open the generated command-line reference and confirm it carries every verb the tool accepts and no
   verb it does not.
8. Confirm no page contradicts another and that none claims a capability the runtime refuses.
