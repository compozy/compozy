# UI Audit — <surface name>

**Date:** <YYYY-MM-DD>
**Scope:** <affected routes, components, states, and explicit exclusions>
**Authorities:** <accepted brief, design/token sources, applicable project requirements>

<!-- Optional outline for a requested audit. Keep only sections that clarify the result. Do not infer release readiness from a focused inspection. -->

## Summary

<Lead with the highest-impact finding and the extent of the inspection. State release readiness only when that decision is in scope and supported by the project's required evidence.>

## Findings

Classify confirmed defects using project policy and user impact. A design preference is an advisory, not a blocker. Record an accepted intentional choice only when its consequence matters.

| Finding | Impact / severity | Where | Observed evidence | Proposed fix or decision |
| --- | --- | --- | --- | --- |
| <finding> | <consequence and applicable severity> | <path/selector> | <reproduction or visual evidence> | <action> |

## State Coverage

| Component / journey | State or interaction | Result | Evidence / limitation |
| --- | --- | --- | --- |
| <affected surface> | <state exercised> | <observed outcome> | <link or untested condition> |

## Design and Accessibility Details

<!-- Include only when these details help assess a finding. Derive required values from the actual design system and accessibility contract. -->

| Concern | Current behavior | Owning requirement | Evidence / correction |
| --- | --- | --- | --- |
| <token, copy, hierarchy, keyboard, focus, contrast, or layout> | <observation> | <source> | <evidence or change> |

## Verification Evidence

- <Relevant command/run with build, inputs, and result>
- <Screenshot or interaction evidence for the changed states>
- <Untested device, mode, language, or dependency that limits the claim>

## Recommended Changes

<Smallest useful changes, grouped by behavior; omit if the findings already explain them.>

## Completion

<State whether the requested audit is complete and what findings remain. For implementation or release work, cite the applicable checks and outstanding requirements. Tracking issues and approvals follow the authorized project workflow; this template does not introduce them.>
