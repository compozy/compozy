# Voice and Disclosure

House voice management plus the four disclosure contracts: blameless register (postmortems), coordinated-disclosure four-panel (security), paper-link-first attribution (AI), and "what we'd do differently" honesty (migrations).

## Contents

- [Six diagnostic axes of publisher voice](#six-diagnostic-axes-of-publisher-voice)
- [Blameless register (three rules)](#blameless-register-three-rules)
- [Coordinated-disclosure four-panel (security)](#coordinated-disclosure-four-panel-security)
- [Paper-link-first attribution (AI/agent)](#paper-link-first-attribution-aiagent)
- ["What we'd do differently" honesty (migration / retrospective)](#what-wed-do-differently-honesty-migration--retrospective)
- [Probabilistic register for threat-modeling](#probabilistic-register-for-threat-modeling)
- [Vendor-naming conventions](#vendor-naming-conventions)
- [Acknowledgements as distributed-responsibility signal](#acknowledgements-as-distributed-responsibility-signal)
- [Disclaimer paragraphs as genre signal](#disclaimer-paragraphs-as-genre-signal)
- [Charity toward the predecessor system](#charity-toward-the-predecessor-system)
- [Voice anti-patterns (consolidated)](#voice-anti-patterns-consolidated)

## Six diagnostic axes of publisher voice

Publisher voice is the residue: what persists across posts at a publisher when authors, archetypes, and subject matter change. Triangulate on six axes (no single axis is the voice):

1. **Byline weight.** Single-author / two-author / multi-author / multi-author + named acknowledgements paragraph.
2. **Sentence length distribution.** Short imperative / medium technical / long essayistic.
3. **Vendor / product mention density.** How often the publisher's own products surface in prose (low for engineering-deep-dive; high for product-changelog voice).
4. **Evidence reflex.** Which evidence form the publisher reaches for first (Datadog: percentile graphs; Vercel customer voice: block quotes; Jane Street: long code listings; Cloudflare: kernel-level C + sequence diagrams).
5. **Opening register.** Lede preference — felt experience / scale-then-number / threat-model / paper-link-first / shipping-status.
6. **Closing register.** Engineering reflection / hard CTA / forward-work / open-source repo / explicit acknowledgements / "Discuss on Hacker News" link.

Full per-publisher matrix lives in `publisher-voice-matrix.md`.

## Blameless register (three rules)

The postmortem genre's signature voice contract. Reverse the polarity (active where blame would land, passive where credit is claimed) and the genre fails.

1. **System-subject sentences.** Subjects in the causality narrative are systems, not people. *"The affected asset was a JavaScript file responsible for displaying the editor's object panel"* (Canva `050:61`) — not *"the engineer who deployed that bundle."* *"On start-up of v248, systemd-networkd flushes all IP rules it does not know about"* (Datadog `034:48`) — not *"the systemd maintainers chose to flush."*

2. **Passive voice where it isolates fault, active voice where it claims learning.** *"An automated upgrade was triggered at 6:00 UTC"* (passive — isolates fault from any individual decision). *"We have built substantially more robust persistent disk storage"* (active — claims the learning as institutional). Google's general active-voice preference relaxes specifically in postmortem causality sections.

3. **First-person plural for ownership.** Postmortems use "we" — not "Datadog" or "the SRE team" — when claiming responsibility *and* learning. Canva's *"We attempted to work around this issue… Unfortunately, it didn't mitigate"* (`050:76`) names the failed mitigation without distancing from it. Third-person ("Datadog experienced an outage") appears only in the summary.

**Tone calibration: between detached and contrite.** Blameless does not require false neutrality. Datadog's *"this incident reminded us"* and *"we have to accept"* acknowledges the event was significant. Overly neutral postmortems read as detached; overly contrite ones read as performative.

**The "read aloud and replace 'we' with the publisher's name" test.** If the sentence still scans, the register is correct. If it sounds awkward, the "we" was hiding individual blame.

## Coordinated-disclosure four-panel (security)

Four moves the security genre encodes. Not optional; reading conventions of the genre.

1. **Threat-model opens before the fix.** GitHub PQ-SSH opens with SNDL adversary capability, not with the cipher rollout. Cloudflare Copy Fail opens with the CVE number, kernel subsystem (`AF_ALG`), and exploit primitive ("4-byte write past the boundary"), then sequences how mitigation followed.

2. **UTC timeline non-negotiable for public CVE response.** Cloudflare's table starts at `2026-04-29 16:00 — Copy Fail publicly disclosed` (the moment the secret leaves the coordinated-disclosure circle) and runs through patched-LTS rollout. The timeline anchors disclosure ethics: it shows the gap between public disclosure and protection, names parallel workstreams, and proves the response was not retroactive narrative.

3. **Upstream attribution is the canonical source.** Cloudflare defers exploit mechanics with *"A comprehensive write-up can be found in the original Xint Code disclosure post"* and links the upstream kernel-tree commit `a664bf3d603d`. The post participates in responsible disclosure by pointing at upstream sources rather than re-publishing a self-contained exploit.

4. **Scope caveats made explicit.** GitHub PQ-SSH: *"This only affects SSH access and doesn't impact HTTPS access at all… does not affect GitHub Enterprise Cloud with data residency in the United States region. Only FIPS-approved cryptography may be used within the US region."* Cloudflare PQ-IPsec openly admits the Palo Alto Networks interop gap. Naming the boundary stops the reader from generalising past it.

## Paper-link-first attribution (AI/agent)

MLE-STAR places the arXiv link inside a "Quick links" block above the first body paragraph. Slack's investigation-agent post cites Meta-Prompting and Multi-Persona Self-Collaboration in its second scroll. Google's scaling-agents post cites four named benchmarks in its second paragraph.

The convention signals that the post participates in a research conversation, even when the host blog is product-facing.

**Slop variant:** name-drop the paper title without summarising the result, or worse, claim the result without linking the paper.

**Strong form:** pair the citation with a one-sentence summary of the load-bearing claim the citation supports.

## "What we'd do differently" honesty (migration / retrospective)

Migration writing retains the messiness:

- Meta WebRTC explicitly describes *"thousands of duplicate symbol errors,"* *"hundreds of thousands of lines were modified across thousands of files"* (`199:96`).
- Slack EMR discloses orphaned processes, custom SSH operators, audit complexity — rather than claiming "we migrated 700 jobs with zero issues."
- Datadog's evaluation-platform publishes a *deliberate short-term regression* (11% pass-rate drop) as a credibility signal.
- Canva's API-gateway report admits *"we'd underestimated the impact of the bug and didn't expedite deploying the fix"* — a Canva-side contributing factor alongside the upstream contribution.

Triumphal language ("successfully," "smoothly," "without issue") at any density is a regression. The "successfully / smoothly / without issue" lint catches drift into triumphal register.

## Probabilistic register for threat-modeling

Sentences like *"sensitive information could be eventually at risk even if quantum computers are still years away"* (Meta) and *"an attacker could save encrypted sessions now and, if a suitable quantum computer is built in the future, decrypt them later"* (GitHub) signal calibrated uncertainty. The verbs are conditional.

The hedge is not weakness; it is calibration.

**Mature form:** *"Research indicates that quantum computers will eventually break conventional public-key cryptography… Although experts estimate this could happen within 10–15 years, sophisticated adversaries could collect encrypted data today"* — produces alarm without panic.

**FUD anti-pattern:** *"Quantum computers will break the Internet's encryption and your secrets are at risk right now."*

**Capability vs limitation register switch.** Capability claims declarative present: *"MLE-STAR uses web search to retrieve relevant and potentially state-of-the-art approaches"*. Limitations hedged future-conditional: *"LLM-generated Python scripts carry the risk of introducing data leakage."* Capability and limitation in the same voice = over-claiming.

## Vendor-naming conventions

- **Name the partner when shared responsibility exists, and quote them verbatim.** Canva quotes Cloudflare's full statement about the stale traffic-management rule with attribution (`050:59`), then immediately follows with a Canva-side contributing factor. The verbatim quote establishes the partner's accountability without Canva making claims on the partner's behalf.

- **Name your own products as instruments, not features.** Datadog network-latency mentions Network Performance Monitoring because it is what the engineers used — not as a CTA. Vendor density is moderate and instrumental. Vercel customer posts invert this: Vercel/Next.js/Preview Deployments/performance analytics/Vercel CLI appear in close succession as a capability catalog.

- **Name competitor and third-party products specifically when they are load-bearing.** Cloudflare PQ-IPsec names Cisco 8000 Series Secure Routers (version 26.1.1+), Fortinet FortiOS 7.6.6+, Palo Alto Networks RFC 9370 implementation, including the interop gap. Specificity is the credibility move.

- **Genericise only when security requires it, and disclose the genericisation.** A postmortem that cannot name its own systems should disclose why ("for security reasons, we are not naming the specific kernel module") rather than silently substituting "a critical microservice" or "an upstream provider."

- **Vendor names live at R5 (implementation), not R1/R2 (framing).** The single rule that separates an engineering post from a customer story. Apply the **"remove the vendor's name" test** as a pre-publish gate (full text in `depth-and-abstraction.md`).

## Acknowledgements as distributed-responsibility signal

A risk story by one named author looks fragile; one credited across a full response or migration team looks load-tested.

- Cloudflare Copy Fail names 26 engineers plus *"the Linux upstream maintainers and Copy Fail researchers."*
- Meta's PQC framework lists colleagues across Transport Security, WhatsApp, Facebook/Messenger, Infrastructure, Reality Labs, Hardware, and Payments.

Multi-engineer bylines on incident and security posts act as partial defense against the ghost-written-byline anti-pattern.

## Disclaimer paragraphs as genre signal

Meta closes its PQC post with an italicised paragraph stating the article *"does not constitute professional, technical, or legal advice, nor does it constitute a guarantee of any particular security outcome."* The disclaimer signals the post is sharing a framework, not warrantying an outcome.

Launch posts routinely warranty throughput, latency, and cost claims. The security genre explicitly refuses to.

## Charity toward the predecessor system

Migration writing characterises the prior state with charity, not contempt:

- Datadog: *"The shared database limps along for as long as it possibly can — and then some."*
- Meta WebRTC: *"Permanently forking a big open-source project can result in a common industry trap. It starts with good intentions."*
- Slack: *"It worked. But it came with some potential problems."*

The charity is a craft move: migration posts are often read by the engineers who built the prior system, and contemptuous register burns credibility internally and externally.

## Voice anti-patterns (consolidated)

- **Hedged "we want to share some thoughts on…" lede** — burns the first paragraph on apologia. Replace with a stakes-and-problem two-step (GitHub) or a CVE-date framing (Cloudflare).
- **"We're excited to announce…" template** — banned at Datadog by editorial reflex; reads as marketing on any engineering post.
- **Blame-by-implication** — *"While our autoscaling capability was outpaced…"* is weaker than *"Our autoscaling was outpaced."* The "While"/"Although" construction signals more concern for reputation than learning.
- **Paper-name-dropping without summary** — cite the paper, then summarise the load-bearing claim in one sentence.
- **False-precision metrics** — *"99.3% accuracy"* is meaningless without the dataset, the baseline, and the class-imbalance disclosure.
- **AI-eval-as-anecdote** — a single transcript or screenshot is proof-of-concept, not a production claim.
- **Mismatched closing CTA** — a 4,000-word postmortem that ends with "Sign up for a free trial" alienates both audiences. Postmortems close with engineering reflection; security responses close with follow-up work + hedge; migrations close with dated remaining work; AI posts close with a callable artifact.
- **Ghost-written-byline** — a post carrying an engineer's name but written by a marketer ventriloquising the engineer. The prose loses the texture of decisions-made-under-uncertainty.
- **Triumphal migration post** — *"We migrated 700 jobs with zero issues"* reads as luck or untruth.
- **"We have you covered" security close** — promises completeness without naming defenses, scopes, and residual gaps. Strong specimens close with explicit follow-up lists and hedge clauses.
- **Over-redacted postmortem** — *"a critical microservice," "an upstream provider."* Readers notice. Disclose why redaction is necessary, do not silently genericise.
