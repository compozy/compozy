# Publisher Voice Matrix

Cross-publisher matrix for seven publishers (Datadog, Vercel, GitHub, AWS, Meta, Cloudflare, Jane Street) by six surface features, plus banned moves per publisher.

## Contents

- [How to use this matrix](#how-to-use-this-matrix)
- [The seven-publisher matrix](#the-seven-publisher-matrix)
- [Extended publisher signatures](#extended-publisher-signatures)

## How to use this matrix

Triangulate publisher voice on six axes (none alone is the voice):

1. **Byline weight** — single / two / multi / multi + named acknowledgements
2. **Sentence length distribution** — short imperative / medium technical / long essayistic
3. **Vendor / product mention density** — low / moderate / high
4. **Evidence reflex** — which evidence form the publisher reaches for first
5. **Opening register** — lede preference
6. **Closing register** — engineering reflection / hard CTA / forward-work / open-source / HN link / acknowledgements

When tuning a draft to a publisher's house style, audit the lede and closer against the publisher's row, then sweep the body for evidence-reflex match.

## The seven-publisher matrix

| Publisher | Register / Person / Tense | Lede preference | Evidence reflex | Hedging budget | Closing register | Banned move |
|-----------|---------------------------|----------------|-----------------|---------------|------------------|-------------|
| **Datadog** | Formal-quantitative, first-person plural, mixed-tense narrative ("we observed", "we now persist") | Problem-from-experience hook; sometimes a "square-wave failure pattern"-style coined-term lede | Annotated percentile graphs over time, named services and package versions | Moderate; uses "we have to accept", "this incident reminded us" to signal genuine learning | Engineering reflection or system-design claim — never a hard CTA on postmortems and reliability essays | Never opens an incident post with "we are excited"; refuses to genericise subsystem names |
| **Vercel** | Two sub-voices. **Customer-narrative:** marketing-adjacent, short sentences, frequent block quotes from named customer roles. **Product-changelog:** short imperative, code-first | Customer name (customer voice) OR feature framing + code block (changelog voice) | Customer block quotes; or code blocks + architecture diagrams | Low; customer voice avoids hedge entirely, asserting outcomes via quote | Hard CTA — "Start Deploying" / "Talk to an Expert" buttons; never reflection | Customer-narrative voice never narrates engineering tradeoffs in first-person engineer voice; the marketing-byline guarantees the register |
| **GitHub** | Engineer-as-individual under a personal byline; first-person plural for the team plus occasional first-person singular asides; sentence length range is widest in corpus | Stakes-then-problem two-step: one sentence on user stakes, one sentence on engineering problem | Mixed: metric tables, screenshots, code listings, INP percentile graphs | Moderate; one-line teaser subtitle often functions as the hedge ("The path to better performance is often found in simplicity") | Reflection paragraph plus a one-line invitation; never a hard CTA | An engineer is never anonymised in a GitHub post; ghost-written-byline anti-pattern is structurally prevented by named photo + per-author archive |
| **AWS** | Capability-catalog third-person prose; passive constructions; mid-to-long sentences with heavy noun phrases; "you" as reader, never first-person plural for the team | Problem-the-reader-already-has framing ("If you're architecting cloud systems for AI development on AWS, you've likely discovered that…") | Architecture diagrams + captioned figures + "**Note:**" advisory call-out boxes after each sub-section | Low; "you can", "you should" predominates over "we believe" | Roll-up of services plus documentation links; sub-blog footer; documentation *is* the CTA | Never surfaces individual author byline in the visible header; the sub-blog identity outranks the engineer |
| **Meta** | Institutional-multi-author; first-person plural across teams; ALL-CAPS "POSTED ON" prelude; medium-to-long sentences with frequent passives | TL;DR bullet summary of "we're sharing / we're proposing / we hope" *before* any prose | Labelled framework diagrams with capitalised category names; step-numbered methodology sections | Moderate; framework introductions ("PQC Migration Levels") stated assertively, every section bounded by what the framework can claim | Forward-looking paragraph on what the program will enable next; optional formal disclaimer in italics | Refuses single-author byline for migration / security / capacity-efficiency posts; if the work is institutional, the byline must reflect it |
| **Cloudflare** | Network-engineering-academic; short-to-medium high-info-density sentences; ISO date format and explicit minute read; first-person plural with occasional first-person singular in author asides | CVE date or network-event framing ("On April 29, 2026, a Linux kernel local privilege escalation vulnerability was publicly disclosed…") | Kernel-level C in the body, sequence diagrams of cryptographic handshakes, RFC + IETF draft citations as hyperlinks | High where uncertain (probabilistic framing), low where load-bearing (specific commit hash, FIPS standard number) | "Discuss on Hacker News" link, tag list, related posts; explicit acknowledgements paragraph naming all responders | Never re-publishes a working exploit before upstream patch is widely deployed; explicitly defers exploit mechanics to upstream researcher write-up |
| **Jane Street** | Technical-essayistic; single-author byline; longest sentences in corpus; willing to use first-person singular alongside editorial "we"; OCaml/compilers/hardware vocabulary | Essayistic, meta-discursive — situates the post within a larger conversation; occasional self-aware aside | Long code listings (full OCaml function definitions), trace visualisations, performance graphs | High where the assumed-reader bar is high; allows author confessional | Reflection, optional pointer to a tech talk, prev/next pair; no CTA, no Hacker News link, no marketing | Refuses to dilute author voice for a corporate template; refuses to attach a sales footer; refuses to genericise OCaml-specific vocabulary |

## Extended publisher signatures

Sketched from sampled material — useful when targeting a non-matrix publisher.

- **Canva** — single-author postmortem byline with a personal LinkedIn link and a "post incident review" pre-header. Register is institutional-restrained: *"This is our first publicly shared incident report. We're doing this as part of our commitment to transparency, accountability, and continuous improvement."* Strong vendor-naming discipline (verbatim Cloudflare quote with attribution).

- **Docker** — single-engineer voice with personal asides in a series format ("This is issue 1 of a new series"). Mixes citation-heavy explainer prose with first-person reflection ("The simplest mental model I've found"). Never names a single competitor product without immediately citing public coverage that already named it.

- **Slack** — multi-author institutional voice for security/investigation posts with named agent personas (Director/Expert/Critic) treated as discrete artifacts. Knowledge-pyramid diagrams. Three-persona structured-output rubrics named explicitly.

- **Tailscale** — wider personal register (founder-and-engineer mix) with regular monthly-update digests. Tolerates first-person singular more freely than most corporate engineering blogs.

- **Pinterest** and **Dropbox** — tend toward Meta-shape multi-author posts but with lighter framework branding.

- **Netflix** — "In a previous post…" continuation lede; mid-length; lessons-from-the-trenches close.

- **Stripe** — strategic-essay register (sibling genre — refuse to author with this skill unless explicitly labeled as strategic).

- **AWS sub-blogs** (Architecture, Database, Compute, Machine Learning) — each has slight register variation; the Architecture sub-blog tolerates more first-person plural than the parent main blog.

## When the publisher is not in the matrix

Pick the closest match across the six axes. If unclear, default to:

- **Single-engineer voice** → GitHub or Jane Street, depending on whether the post is engineer-narrative (GitHub) or essayistic (Jane Street).
- **Multi-author institutional** → Meta or Cloudflare, depending on whether the post is framework-shaped (Meta) or response-shaped (Cloudflare).
- **Product-marketing-adjacent** → Vercel customer voice (and strongly consider whether the post should be authored at all by this skill — sibling genre).
- **Tutorial / how-to** → AWS register.
- **Operational deep-dive** → Datadog register.
