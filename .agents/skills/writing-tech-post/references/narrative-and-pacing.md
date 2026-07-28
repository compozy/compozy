# Narrative, Momentum, and Pacing

The five-lede taxonomy, the H2-as-question-resolution discipline, the story-shape catalogue (detective / migration / blameless / paper-link-first / tutorial arcs), momentum-stall diagnostics, the closer multiple-choice gate, and the headline taxonomy.

## Contents

- [Five lede types (paired to archetypes)](#five-lede-types-paired-to-archetypes)
- [H2-as-question-resolution discipline](#h2-as-question-resolution-discipline)
- [Story-shape catalogue](#story-shape-catalogue)
- [Momentum-stall diagnostics](#momentum-stall-diagnostics)
- [Closer taxonomy (multiple-choice gate)](#closer-taxonomy-multiple-choice-gate)
- [Headline + title patterns](#headline--title-patterns)
- [The first-200 + last-200 callback-coupling test](#the-first-200--last-200-callback-coupling-test)
- [Anti-narrative patterns](#anti-narrative-patterns)

## Five lede types (paired to archetypes)

Pick the lede by archetype. Mispairing is the first failure mode the skill catches.

| Lede type | Archetype | Mechanism | Exemplar |
|-----------|-----------|-----------|----------|
| **Result-first** | Launch deep-dive | Open with the headline number, often a multiple. The number anchors every later section. | Datadog Rust storage: "60x increase in ingestion / 5x faster queries at peak scale" (`016`); Datadog 100x process-metrics post (`067`). |
| **Mystery / surprise** | Performance, Jane Street narrative | Open with a pathology the reader has no obvious explanation for. The post commits to explaining the gap. | Datadog Postgres upsert: "We expected this new query to have minimal impact… But when we rolled out the new query, disk writes doubled" (`023`); Jane Street build system: "Ha! What actually happened is that nobody really wanted to use Jenga" (`219`). |
| **Stakes-first** | Performance, migration, postmortem (3-W variant) | Open with what is at risk for the user or the team. Earns the right to spend the next 2,000 words on engineering minutiae. | Datadog network-latency: "Getting paged to investigate high-urgency issues is a normal aspect of being an engineer. But none of us expect…" (`041`); GitHub diff-lines: "Pull requests are the beating heart of GitHub" (`054`); GitHub eBPF: "If github.com were ever to go down, we wouldn't be able to access our own source code" (`028`). |
| **Shipping-status** | Launch, product changelog | "Today we shipped" or "now generally available." Carries an explicit CTA contract. | Vercel AI Gateway GA (`059`); Vercel streaming (`062`). |
| **Paper-link-first** | AI/agent, research-to-product translation | Open with a capability claim and a citation. The paper link is structural, not decorative — transfers credibility from the literature. | Google Research MLE-STAR with arXiv link in "Quick links" block above first body paragraph (`074`). |

**Postmortem 3-W variant** (a stakes-first specialization): date + scope + impact in the first two sentences. Canva: *"On November 12, 2024, Canva experienced a critical outage that affected the availability of canva.com. From 9:08 AM UTC to approximately 10:00 AM UTC, canva.com was unavailable."* The 3-W contract: when, what, where, how-bad, all inside the first 200 words.

**Hedged-lede slop (banned):**

- *"In this post, we'll explore…"* — burns paragraph one on apologia.
- *"We wanted to share some thoughts on…"* — issues no debt to the reader.
- *"This article is written for…"* — announces an explainer rather than showing an arresting fact.

The **productive form** uses "In this post, we'll…" as a road-marker AFTER the pathology paragraph (Datadog 016 third paragraph; 023 fourth paragraph; 036 fourth paragraph), never as the lede itself.

## H2-as-question-resolution discipline

Each H2 should answer the question the previous section left dangling, opening a new question that the next H2 answers, until the post lands. The H2 chain is *not* a noun-phrase index — each header is a verb-phrase that names an action taken to resolve a question, which is what gives the reader the sense the investigation is progressing.

**Textbook execution — Datadog network-latency** (`041:50-90`):

> "Our usage estimation service at a glance" *(sets up the system)*
> → "Allocating more CPU to a remote cache dependency" *(answers: what was the first hypothesis? what did fixing it teach us?)*
> → "Patching a Linux kernel bug" *(answers: why didn't the CPU fix complete the work?)*
> → "Optimizing AWS instance network configurations" *(answers: why didn't the kernel patch land the distribution?)*
> → "Routing client requests away from terminating pods" *(answers: what was left after AWS?)*
> → "Recap" *(closes the chain)*
> → "Bolster your visibility and learn to look twice" *(lifts the lesson)*

Each section closes with the partial-victory paragraph (e.g. *"This was a slight improvement, but clearly still higher than normal"*) that licenses the next H2.

**Pre-prose outline review:** produce the H2 chain and annotate each H2 with what question it answers and what question it leaves open. Reject outlines where any H2 fails the gate.

**Counter-example — noun-phrase H2s:** AWS architecting-for-agentic-AI (`044:62-75`) runs noun-phrase H2s ("Why traditional architectures hinder agentic AI", "System architecture for fast agentic feedback loops") and resists the question-chain. The result is correct for the reference archetype (the H2s function as a TOC, not a narrative spine) but reads as catalogue-paced.

## Story-shape catalogue

Each archetype gets a default arc.

### Detective-arc (performance)

Hypothesis → instrument → measurement → partial victory → re-arm.

Signature beat: the **partial-victory paragraph** that the network-latency post `041` repeats five times. Two closing variants:

- *Honest-recap close* — bullets the chain and names the operational gap that should make next time faster (Datadog `041`, `067`).
- *Lessons-from-the-trenches close* — generalises one transferable lesson (Datadog `023`, `069`).

Both refuse the wrapped-bow close; the diff-lines post `054` says explicitly *"the improvements didn't end there"* and lists ongoing work.

### Migration-arc

Legacy-charity opening → why-now → phased plan with tracking metrics → cutover discipline → dated status snapshot → forward work.

Datadog shared-database (`036:51-83`) opens with three full sections of legacy charity: *"Why do shared databases exist?"*, *"When is it time to take apart the shared database?"*, *"What keeps teams from moving off shared infrastructure?"* — each H2 a literal question, the section answering it. Rhetorical-question H2s do the work of justifying the migration's existence before the phased-plan section begins.

Meta WebRTC (`199:50-117`) runs a tighter variant — Challenge → Solution 1 (Shim Layer) → Solution 2 (Feature Branches) → The Result → Future Work. The "Future Work: AI-Driven Maintenance" close is the trope flagged as over-weighted in 2026 posts; if used, must name at least two specific applications.

### Blameless-arc (postmortem)

3-W summary → background → the incident → timeline → contributing factors → mitigation (including mitigations that failed) → action items → optional lessons / acknowledgments.

Signature beat: at least one **failed-mitigation paragraph**. Canva `050`'s *"We attempted to work around this issue by significantly increasing the desired task count manually. Unfortunately, it didn't mitigate the issue"* is what licenses the next mitigation paragraph to exist. Without it, the escalation reads as panic; with it, it reads as ordered learning.

Datadog `034` runs an ambitious temporal-displacement variant: opens with the outage date, then traces causality back two years to a December 2020 systemd commit.

### Paper-link-first arc (AI/agent)

Capability claim → paper link → motivation → method overview → architecture diagram → named-benchmark table → ablation → named guardrails → forward-section motif.

MLE-STAR `074:43-89` runs it tightly: Quick links → Introducing MLE-STAR → Evaluations and results → In-depth analysis of MLE-STAR's gains → Conclusion → Acknowledgements. The "In-depth analysis" is the cohort's load-bearing momentum move — it decomposes the headline number so the reader can audit which part of the system delivered it.

Slack security-investigation `160:48-130` runs a related variant where the H2 chain is The Development Process → From Prototype to Production → Service Architecture → Example Report → Conclusion, with the *Critic-finds-what-Expert-missed* worked example doing the ablation work in narrative form.

### Tutorial-arc

Prerequisites → primer → stepwise procedure → working example → caveats → next steps.

AWS Lambda multi-threaded Rust (`170:36-90`) runs it cleanly: "Our Test Workload: Why Bcrypt Password Hashing?" → "Understanding Lambda's vCPU Allocation" → "Solution Overview" → "Creating a Multi-threaded Rust Lambda Function" (stepwise).

Tutorial momentum runs on different fuel from the detective-arc: the reader is asked to *do* something at each step, and the H2 chain's job is to confirm they have not lost the thread. Tutorial H2s are allowed to be more noun-phrase-heavy ("Dependencies", "Solution Overview") because the action is in the code blocks.

### Narrative-essay (corpus minor variant)

Jane Street's build-system post `219:48-77`: We had a tool → We released it and it failed → We built a smaller compatibility shim → The shim accidentally became popular → We had to rename it → We had to migrate to it → We made it scale → It worked. Six-paragraph story; H2 chain functions as chapter titles.

Rare in the corpus because most engineering organisations cannot write in this voice; Jane Street can because its publisher voice tolerates first-person individual register.

## Momentum-stall diagnostics

Four diagnostics recur.

- **Scene-setting density too high.** Pre-pathology paragraph runs longer than necessary; reader skims past the lede. Resolved by pairing one scene-setting paragraph with one pathology paragraph (one-and-one). Longer scene-setting needs a stronger pathology to balance it.
- **Payload-density ratio too low.** Sections that introduce no new fact, number, code reference, or distinction. Diagnostic: read the H2s and ask "what does the reader know after this section that they did not know before?" If the answer is "we explained the architecture in more words," cut.
- **Callback frequency too low.** A fact introduced in section 1 is not referenced again before section 5. Datadog Rust storage resolves this by introducing the "6th generation in a lineage that started 15 years ago" frame in the lede and explicitly running Gen 1 → Gen 6 H2s under "How we built the 6th generation of our real-time metrics storage" (line 77). The lede claim is paid back in the body's spine, then again in the closing.
- **Rhetorical-question budget exceeded.** Datadog shared-database `036:51-83` runs three rhetorical-question H2s — the maximum the genre tolerates before the rhetorical move starts to feel performative. Cap at three per long post.

## Closer taxonomy (multiple-choice gate)

The closer must be one of five forward-pointing shapes. Summarising closers ("In conclusion, we have shown that…") are a regression.

- **Call-to-build.** Close on an artifact the reader can run, build, or extend. GitHub eBPF `028:317-326`: *"Want to dive in? Get started by having a look through the examples in [cilium/ebpf](https://github.com/cilium/ebpf/tree/main/examples)…"*. MLE-STAR `074:81-86`: link to `google/adk-samples`. The open-source / research-translation move.
- **Call-to-adopt.** Close on a managed-service link, a sign-up button, or an "available now" CTA. Vercel product-changelog signature. Tolerable when the post has paid out the engineering substance; bait-and-switch when the substance is thin.
- **Open-question.** Close on the unresolved part. Datadog diff-lines `054:53-56`: *"even within our large and mature codebase, can deliver meaningful benefits to all users — and that sometimes focusing on small, simple improvements can have the largest impact."* Lifts the post's specific finding into a general principle without claiming the principle is settled.
- **Shipping-status / roadmap.** Close on what is now in production and what is next. Datadog Rust storage `016:194-210`: *"Looking ahead: smarter routing and integrated indexing"* with three concrete next-direction commitments. Launch and migration archetype's signature; forward-points without overclaiming.
- **Prevention-list.** Close on a bulleted action items list naming what changes are now committed. Canva `050:95-115` groups action items by category. Cloudflare Copy Fail `232:178-195`: "Remediation and follow-up steps" then "Conclusion" naming the specific resolved state. Postmortem and non-incident-response signature.

A sixth variant exists for performance posts: **distribution-chart close** — close on a graph that shows the distribution moving. The chart *is* the closing argument.

## Headline + title patterns

- **Number-first.** Lead the title with the quantified result. *"How we improved efficiency of live process metrics by 100x"* (`067`). The number is the contract; the post must pay it back. A "60x" title with no 60x evidence is **archetype-bait**; reject.
- **Decision-narrative.** Lead with the verb that names the strategic choice. *"Breaking up a monolith: How we're unwinding a shared database at scale"* (`036`). *"Escaping the Fork: How Meta Modernized WebRTC Across 50+ Use Cases"* (`199`). Migration-arc's most common title shape.
- **Surprise-clause / pathology.** Open the title with the negation of the reader's expectation. *"Not just another network latency issue: How we unraveled a series of hidden bottlenecks"* (`041`). *"When upserts don't update but still write: Debugging Postgres performance at scale"* (`023`). Detective-arc signature; pairs with the mystery lede.
- **System-name / introduction.** Lead with the named artifact. *"MLE-STAR: A state-of-the-art machine learning engineering agent"* (`074`). Launch deep-dive and research-translation signature; forces the post to define the system before the reader leaves the lede.
- **Stakes / journey.** Lead with the user experience. *"From latency to instant: Modernizing GitHub Issues navigation performance"* (`017`). Pre-commits the post to a journey shape.
- **Question-form.** Less common but distinctive. True interrogative titles invite skim-and-skip behaviour unless the question is genuinely puzzling.

## The first-200 + last-200 callback-coupling test

Extract the first 200 words and the last 200 words and pair them. If they cannot be paired into a same-thread statement, the post has no spine.

Datadog Rust storage `016` passes the test: lede introduces "6th generation in a lineage that started 15 years ago" + "60x ingestion / 5x query" → closer pays back lineage callback ("Looking ahead: smarter routing and integrated indexing") + roadmap.

Datadog network-latency `041` passes: lede issues debt ("we don't expect to get paged about every single deployment") → closer "Bolster your visibility and learn to look twice" pays the lesson.

Run this as a **pre-flight check** before any other review.

## Anti-narrative patterns

- **Buried lede.** Headline number in section three. Catalogued as launch-archetype failure.
- **Abstract-then-concrete inversion.** Post opens with a generic industry trend and waits until section two to introduce the specific system. Strategic essays do this on purpose; engineering deep-dives that imitate forfeit pull.
- **Every section a noun phrase.** H2s read as a TOC, not as the spine of an argument. Diagnostic: when the H2s can be skimmed in isolation and re-ordered without changing meaning, the question-chain is broken.
- **Callbacks that never connect.** A fact named in the lede is not referenced again. Diagnostic: extract the first 200 words and the closing 200 words — if they cannot be paired into a same-thread statement, the post has no spine.
- **Closers that summarise instead of forward-pointing.** *"In conclusion, we have shown that…"* recapitulations. Almost every read-at-depth closer in the corpus picks one of the five forward-pointing shapes.
- **Bolted-on AI-future close on a non-AI post.** Generic "AI handles the long tail" paragraph that reads as performative when the AI work is not genuinely on the roadmap. If used, the forward-section must name at least two specific applications.
- **Archetype-bait headline.** Title promises archetype A; body delivers archetype B. Erodes publisher trust over a series of posts.
