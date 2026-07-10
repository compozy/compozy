# Incident Postmortems

The postmortem archetype's contract: canonical section sequence, blameless register, UTC-timeline + named-artifact root cause obligations, failed-mitigation discipline, and the postmortem/reliability-essay hybrid rule.

## Contents

- [Opening move and section sequence](#opening-move-and-section-sequence)
- [The blameless register (three rules)](#the-blameless-register-three-rules)
- [Evidence obligations](#evidence-obligations)
- [The failed-mitigation paragraph](#the-failed-mitigation-paragraph)
- [Closing move (prevention vs reliability essay)](#closing-move-prevention-vs-reliability-essay)
- [Postmortem / reliability-essay hybrid](#postmortem--reliability-essay-hybrid)
- [Tone calibration: between detached and contrite](#tone-calibration-between-detached-and-contrite)
- [Silent failure modes (postmortem slop)](#silent-failure-modes-postmortem-slop)

## Opening move and section sequence

**Opening:** date + scope + impact in the first two sentences. The 3 W's + impact (when, what, where, how-bad) inside the first 200 words. Delay reads as evasive.

Exemplars:

- Canva: *"On November 12, 2024, Canva experienced a critical outage that affected the availability of canva.com. From 9:08 AM UTC to approximately 10:00 AM UTC, canva.com was unavailable."*
- Datadog 2023-03-08: *"On March 8, 2023, Datadog experienced an outage that affected all services across multiple regions."*

**Canonical section sequence** (strict, near-canonical):

1. **Summary** — third-person ("Datadog experienced an outage").
2. **Background** — system context the reader needs.
3. **The incident** — narrative of what happened.
4. **Timeline** — UTC timestamps with defined granularity, narrated.
5. **Contributing factors** — usually multiple, not "one root cause."
6. **Mitigation** — including failed attempts (see §[failed-mitigation paragraph](#the-failed-mitigation-paragraph)).
7. **Action items** — per category, with owners and ETAs.
8. **Optional: Lessons** — for reliability essay hybrid (see below).
9. **Optional: Acknowledgements** — for cross-team or vendor-collaborative incidents.

The sequence is not aspirational — every section is required *unless* the post is explicitly labelled as a non-canonical variant (e.g., "preparedness post" / "non-incident response").

## The blameless register (three rules)

1. **System-subject sentences.** Subjects in the causality narrative are systems, not people. *"The affected asset was a JavaScript file responsible for displaying the editor's object panel"* (Canva `050:61`) — not *"the engineer who deployed that bundle."* *"On start-up of v248, systemd-networkd flushes all IP rules it does not know about"* (Datadog `034:48`) — not *"the systemd maintainers chose to flush."*

2. **Passive voice where it isolates fault, active voice where it claims learning.** *"An automated upgrade was triggered at 6:00 UTC"* (passive — isolates fault). *"We have built substantially more robust persistent disk storage"* (active — claims the learning). The polarity is the genre's signature; reversing it is the most common failure mode. Google's general active-voice preference relaxes specifically in postmortem causality sections.

3. **First-person plural for ownership.** Postmortems use "we" — not "Datadog" or "the SRE team" — when claiming responsibility *and* learning. Canva: *"We attempted to work around this issue… Unfortunately, it didn't mitigate."* Third-person ("Datadog experienced an outage") appears only in the summary.

**Test:** read the draft aloud and replace every "we" with the publisher's name. If the sentence still scans, the register is correct. If awkward, the "we" was hiding individual blame.

## Evidence obligations

- **UTC timestamps with defined granularity.** Minute-resolution for incidents under an hour; ten-minute or hour-resolution for longer incidents. Always UTC; never local time without UTC conversion alongside.
- **A specific named artifact for root cause.** Commit SHA (Datadog 034 links systemd PRs `#17477` and `#19287`), CVE number (Cloudflare Copy Fail cites `CVE-2026-31431`), kernel function, package version. **Vagueness here is fatal.**
- **Quantitative impact.** Sample: Canva's "1.5M req/s, 3× peak load, 1700% TTFB increase, 270,000+ pending requests." Numbers, not "severe impact."
- **Named services and versions.** Canva names Netty, Amazon ECS, Cloudflare tiered cache, AWS S3. Datadog 034 names systemd v248/v249, Ubuntu 22.04.
- **Verbatim partner quote** when a third party shares responsibility. Canva blockquotes Cloudflare's full statement about the stale traffic-management rule with attribution. The verbatim quote establishes accountability without Canva making claims on Cloudflare's behalf.

## The failed-mitigation paragraph

Strong postmortems disclose mitigations that **did not work**. The failed-mitigation paragraph is *load-bearing momentum*: it licenses the next mitigation paragraph to exist. Without it, escalation reads as panic; with it, it reads as ordered learning.

Canonical execution — Canva `050`: *"We attempted to work around this issue by significantly increasing the desired task count manually. Unfortunately, it didn't mitigate the issue."*

**Requirement:** at least one failed-mitigation paragraph in any blameless-arc draft before flagging the draft as complete.

## Closing move (prevention vs reliability essay)

Two variants depending on archetype scope:

- **Pure incident postmortem** → prevention commitments per category. Canva closes with grouped action items: *"Incident response process improvements / Increased resilience of the API Gateway / Fix the telemetry bug / Improvements to detecting page deployment failures / Collaboration with Cloudflare."*

- **Reliability essay derived from postmortem** → bulleted design principles. Datadog 038 (`038-…rethinking-reliability.md`) closes with *"Always start with what's important to the end user… Persist data early…"* The post is the postmortem zoomed out 6–18 months later, with the incident as motivation and design principles as the load.

Both variants forbid the **"we have you covered"** closure that promises completeness without naming residual gaps.

## Postmortem / reliability-essay hybrid

The reference example is the Datadog 034 / 038 diptych:

- `034` (March 2023, single author) — pure incident postmortem; date + scope opening; dormant-fault causality back to 2020 systemd PRs.
- `038` (October 2025, 3 authors) — reliability essay derived from the March 2023 incident; "square-wave failure pattern" coined-term lede; "never-fail architecture" / "failing better" vocabulary; published 18 months after.

The hybrid produces *paired diptychs* in the 3,000–4,000 + 4,000–7,000 word band, with the second post linked from the first's closing and vice versa.

## Tone calibration: between detached and contrite

Blameless does not require false neutrality.

- Datadog: *"This incident reminded us"* and *"we have to accept"* — acknowledges significance.
- Canva: *"We've been working closely with Cloudflare to gain an in-depth understanding"* — accepts shared responsibility without ducking.

Overly neutral postmortems read as detached. Overly contrite postmortems read as performative.

## Silent failure modes (postmortem slop)

- **Naming a specific engineer** in the causality section.
- **Defensive register** ("While our autoscaling…"). The "While"/"Although" construction signals more concern for reputation than learning.
- **"We have you covered" closures** without naming residual gaps.
- **Over-redaction** ("a critical microservice," "an upstream provider"). Disclose why redaction is necessary, do not silently genericise.
- **24-hour postmortems with placeholder action items.** Wait for the action items to be specific.
- **Missing timeline.** UTC timestamps are non-negotiable.
- **"Five whys without the answer."** Asking five whys but stopping at the third without commitment.
- **No failed-mitigation paragraph.** Escalation reads as panic; the post lacks the cadence that licenses the next mitigation section.
- **Mismatched closing CTA.** A 4,000-word postmortem ending with *"Sign up for a free trial"* alienates both audiences. Postmortems close with engineering reflection or prevention commitments.
