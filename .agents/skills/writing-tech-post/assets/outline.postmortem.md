# [POSTMORTEM TITLE — date + scope, no jargon, no blame]

<!--
archetype: incident-postmortem
depth-tuple: (R2 date+scope, R3 timestamps + R4 causality walk + R5 named commit, R2 prevention, Staircase with R2 anchor at both ends)
length-band: 3,000–4,000 (standalone) / 11,000+ (paired diptych)
byline-norm: single senior author (principal SRE, platform lead) — or 2-3 authors for reliability essay variant
-->

## Summary

<!-- R2 — Third-person ("Datadog experienced an outage"). 3 W's + impact inside the first 200 words. -->

On [DATE], [COMPANY] experienced [WHAT] that affected [SCOPE]. From [START UTC] to [END UTC], [USER-VISIBLE IMPACT].

## Background

<!-- R3 — System context the reader needs. Name the services, the architecture tier, the customer-facing surface. -->

[The system being affected; what it does; why the failure mattered.]

## The incident

<!-- R3 + R4 — Narrative of what happened. System-subject sentences. -->

[Narrative of the unfolding event.]

## Timeline

<!-- R3 — UTC timestamps, minute-resolution. -->

| Time (UTC) | Event |
|-----------|-------|
| [HH:MM]   | [System-subject sentence describing the event.] |
| [HH:MM]   | [...] |
| [HH:MM]   | [Recovery milestone.] |

## Contributing factors

<!-- R4 + R5 — Multiple factors, named specifically. Specific root-cause artifact (commit SHA / PR / CVE / kernel function). System-subject sentences. -->

1. **[Factor 1]** — [System-subject description, linking to specific artifact, e.g., PR #17477 / CVE-2026-XXXXX].
2. **[Factor 2]** — [...]
3. **[Telemetry / detection factor]** — [...]

## Mitigation (including failed attempts)

<!-- R4 — At least one failed-mitigation paragraph is mandatory. -->

[Description of the first mitigation attempt.]

We attempted [X]. Unfortunately, it didn't mitigate [Y].

[Description of the next attempt that worked.]

## Action items

<!-- R2 — Per category, with owners and ETAs. -->

### [Category 1, e.g., Incident response process improvements]

- [Action with owner / ETA]
- [...]

### [Category 2, e.g., Resilience improvements]

- [Action with owner / ETA]

### [Category 3, e.g., Collaboration with vendor]

- [Action with owner / ETA]

## Lessons (optional — reliability essay variant)

<!-- R2 — Bulleted design principles. -->

- [Principle 1, e.g., "Always start with what's important to the end user."]
- [Principle 2]

## Acknowledgements (optional)

[Named cross-team contributors; partner companies; external researchers.]
