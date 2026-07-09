# [SECURITY TITLE — "How [COMPANY] responded to [VULNERABILITY]" or threat-model-first framing]

<!--
archetype: security-reliability
depth-tuple: (R2 threat framing, R5 algorithm names + thin R4 mechanism, R2 forward security posture, Anchor-and-dive into R5)
length-band: 1,000–7,000
byline-norm: multi-author + named acknowledgements paragraph (CVE response) — or single/paired (security launch)
-->

## TL;DR (severity + scope)

<!-- R2 — Threat-model summary; scope caveats. -->

On [DATE], a [VULNERABILITY TYPE] vulnerability was publicly disclosed under the name [CVE-XXXX-XXXXX]. The vulnerability affected [SCOPE]. By the end of the rollout, [OUTCOME — including absence of customer impact when preparedness paid off].

## Threat model

<!-- R2 — Adversary capability + asset + impact horizon. -->

[Adversary capability — what an attacker can do. Use probabilistic register: "could", "might", "if X then Y".]

[Asset at risk — what is being defended.]

[Impact horizon — 10–15 years (PQC) / immediate (CVE response).]

## Background — the affected system

<!-- R4 + R5 — Kernel subsystem / cryptographic primitive / library; just enough context. -->

[How the affected system works. Name the specific kernel subsystem (`AF_ALG`), cryptographic algorithm (`sntrup761x25519-sha512`), or library version.]

## How the vulnerability works

<!-- R4 — High-level mechanism. Defer exploit mechanics to upstream researcher write-up. -->

[Mechanism walkthrough at the right level of abstraction.]

A comprehensive write-up of the exploit can be found in the original [researcher disclosure post](LINK).

## How we responded (UTC timeline)

<!-- R3 — UTC timeline with parallel workstreams. -->

| Time (UTC)   | Event |
|--------------|-------|
| [DATE TIME]  | [Public disclosure event.] |
| [+N hours]   | [Detection and triage.] |
| [+N hours]   | [Surgical mitigation deployed (e.g., bpf-lsm program).] |
| [+N hours]   | [Patched-LTS rollout begins.] |
| [+N days]    | [Patched-LTS rollout completes across fleet.] |

## Layered defense

<!-- R4 + R5 — Each defense named; what it catches. -->

### Defense 1: [Name]

[What it does; how it caught / prevented the issue.]

### Defense 2: [Name]

[...]

## Scope and caveats

<!-- R2 — Naming the boundary. -->

This [analysis/mitigation/disclosure] only affects [SPECIFIC SCOPE] and does not impact [ADJACENT SCOPE]. [Specific FIPS / regional / product caveats.]

## Remediation and follow-up steps

<!-- R2 — Specific next-direction commitments. -->

- [Better visibility into kernel-API dependencies / similar.]
- [Better runtime mitigation infrastructure.]
- [Reduce attack surface — specific architectural change.]

## Acknowledgements

[Named external researchers + multi-engineer response team (e.g., 26 engineers + Linux upstream maintainers).]

---

*[Disclaimer — "The information in this article is shared for informational purposes only and does not constitute professional, technical, or legal advice, nor does it constitute a guarantee of any particular security outcome."]*
