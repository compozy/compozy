---
title: Trigger detail is a rule page
type: feature
---

A trigger now reads as the rule it is, not as a job inspector with the cron stripped out: a plain-language sentence of what the trigger does, a labeled Enable switch opposite it as the only accent on the page, one When / If / Then card, a single-open Recent-runs accordion, a four-card rail, and an Inspect sheet for runtime internals. (#411)

Migration notes: the shared automation detail panel is jobs-only again, and triggers render through their own component family.
