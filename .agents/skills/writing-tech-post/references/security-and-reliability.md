# Security and Reliability

The security/reliability specialty surface's contract: threat-model opening, layered-defense walkthrough, coordinated-disclosure four-panel, probabilistic register for adversary capabilities, CVE + upstream-commit citation contract, and the three sub-variants (preparedness, industry-migration, educational-explainer).

## Contents

- [Opening: threat model first, not the fix](#opening-threat-model-first-not-the-fix)
- [Four-panel canonical structure](#four-panel-canonical-structure)
- [Coordinated-disclosure contract (four moves)](#coordinated-disclosure-contract-four-moves)
- [Probabilistic register for adversary capabilities](#probabilistic-register-for-adversary-capabilities)
- [Evidence obligations](#evidence-obligations)
- [Three sub-variants](#three-sub-variants)
- [Closing move: follow-up + disclaimer](#closing-move-follow-up--disclaimer)
- [Silent failure modes (security slop)](#silent-failure-modes-security-slop)

## Opening: threat model first, not the fix

The security genre opens with *adversary capability + asset + impact horizon* before describing the work.

Exemplars:

- **Cloudflare Copy Fail** — opens with `CVE-2026-31431`, the kernel subsystem (`AF_ALG` and the crypto API), and the exploit primitive (*"4-byte write past boundary"*).
- **GitHub PQ SSH** — opens with the SNDL (Store Now, Decrypt Later) attacker who stores ciphertext today to decrypt later.
- **Meta PQC** — opens with the 10–15-year quantum timeline and SNDL paragraph before any Meta-specific work.
- **Docker horror stories** — structures the entire post as six numbered threat categories with sub-shapes.

Opening with the fix instead of the threat model reads as marketing; the reader is told what to deploy without being told what they are defending against.

## Four-panel canonical structure

Not every panel is used by every post; preventive posts skip disclosure timing, response posts foreground it.

1. **Threat-model opening** — adversary capability + asset + impact horizon.
2. **Background / system context** — what the reader needs to know about the system being defended.
3. **Layered-defense walkthrough** — explicit enumeration of the defenses, each named.
4. **Disclosure-timing section** (for CVE responses) — UTC timeline with parallel workstreams.
5. **Mitigations close** — follow-up + disclaimer (see closing move).

## Coordinated-disclosure contract (four moves)

When responding to a public CVE, four moves are non-negotiable.

1. **Threat-model opens before the fix.** GitHub PQ-SSH opens with SNDL adversary capability, not with the cipher rollout. Cloudflare Copy Fail opens with the CVE number and exploit primitive.

2. **UTC timeline non-negotiable.** Cloudflare's table starts at `2026-04-29 16:00 — Copy Fail publicly disclosed` (the moment the secret leaves the coordinated-disclosure circle) and runs through patched-LTS rollout. The timeline shows the gap between public disclosure and protection, names parallel workstreams, and proves the response was not retroactive narrative.

3. **Upstream attribution is the canonical source.** Cloudflare defers exploit mechanics with *"A comprehensive write-up can be found in the original Xint Code disclosure post"* and links the upstream kernel-tree commit `a664bf3d603d`. The post participates in responsible disclosure by pointing at upstream sources rather than re-publishing a self-contained exploit.

4. **Scope caveats made explicit.** GitHub PQ-SSH: *"This only affects SSH access and doesn't impact HTTPS access at all… does not affect GitHub Enterprise Cloud with data residency in the United States region. Only FIPS-approved cryptography may be used within the US region, and this post-quantum algorithm isn't approved by FIPS."* Cloudflare PQ-IPsec openly admits the Palo Alto Networks interop gap.

Naming the boundary stops the reader from generalising past it.

## Probabilistic register for adversary capabilities

Threat-modeling uses conditional verbs. The hedge is not weakness; it is calibration.

- Meta: *"sensitive information could be eventually at risk even if quantum computers are still years away."*
- GitHub: *"an attacker could save encrypted sessions now and, if a suitable quantum computer is built in the future, decrypt them later."*

**Mature form:** *"Research indicates that quantum computers will eventually break conventional public-key cryptography… Although experts estimate this could happen within 10–15 years, sophisticated adversaries could collect encrypted data today"* — produces alarm without panic.

**FUD anti-pattern:** *"Quantum computers will break the Internet's encryption and your secrets are at risk right now."*

## Evidence obligations

- **CVE number** — explicit. *"CVE-2026-31431."*
- **Upstream commit link** — Cloudflare cites `a664bf3d603d`.
- **Named adversary capabilities** — Shor's algorithm, SNDL, 4-byte write past boundary.
- **Behavioural-detection validation timestamps** — when monitoring/lab tests confirmed the mitigation.
- **Named external researchers** — Cloudflare credits *"the Linux upstream maintainers and Copy Fail researchers"*.
- **Explicit scope caveats** — see coordinated-disclosure rule 4.

## Three sub-variants

1. **Preparedness / non-incident response** — Cloudflare Copy Fail. No customer impact; structure mirrors postmortem; "outcome" reports the *absence* of impact (*"By the end of the rollout, every machine in our fleet was protected by either a patched kernel or a bpf-lsm program"*). Demonstrates preparedness paid off.

2. **Industry migration / maturity-model framework** — Meta PQC (PQ-Unaware → PQ-Enabled ladder), Cloudflare PQ-IPsec. Industry-wide framework instead of an immediate response. Must populate the framework with the publisher's own trajectory (no framework-without-instance).

3. **Educational explainer / failure analysis** — Docker horror stories (six numbered categories anchored in dated cited incidents), Datadog eBPF hardening 5-year retrospective. Pedagogical register; cites public coverage of named incidents.

## Closing move: follow-up + disclaimer

Notably more modest than launch-post closes.

- **Cloudflare Copy Fail** closes with three remediation items and *"at Cloudflare we're always learning and improving."*
- **Meta PQC** closes with *"Sharing our strategy and learnings doesn't mean the process is complete"* followed by formal disclaimer paragraph.

The **disclaimer paragraph** is a genre signal: the post is sharing a framework, not warrantying an outcome. Meta closes its PQC post with an italicised paragraph stating the article *"does not constitute professional, technical, or legal advice, nor does it constitute a guarantee of any particular security outcome."*

**Acknowledgements** of named external researchers + a multi-engineer team are part of the credibility contract. A risk story by one named author looks fragile; one credited across a full response team looks load-tested.

## Silent failure modes (security slop)

- **FUD-style threat framing** — *"the next quantum apocalypse,"* *"the AI threat is reshaping…"*
- **Over-disclosure of working PoC** — re-publishing the exploit before upstream patch is widely deployed.
- **Under-disclosure paraphrased into uselessness** — *"there was a vulnerability that has been addressed."*
- **"We have you covered" closures** without naming residual gaps.
- **Missing coordinated-disclosure timeline** — for response posts.
- **Generic "industry best practices"** — without naming the specific defenses, scopes, residual gaps.
- **Capability without scope caveat** — generalising past what the deployment actually covers.
- **Single-author byline on a CVE response** — distributed responsibility expected; multi-engineer byline is part of the credibility contract.
- **Missing upstream attribution** — exploit mechanics described as if discovered by the publisher when external researchers found them.
