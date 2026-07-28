# Depth and Abstraction

The five-rung abstraction ladder, the five traversal patterns the corpus surfaces, the rung-whiplash diagnostic, the anchor-N rule, and the per-archetype default depth profile.

## Contents

- [The five rungs](#the-five-rungs)
- [The four-tuple commitment](#the-four-tuple-commitment)
- [Traversal patterns](#traversal-patterns)
- [Per-archetype default depth profile](#per-archetype-default-depth-profile)
- [Rung-whiplash diagnostic](#rung-whiplash-diagnostic)
- [The anchor obligation (empirical N)](#the-anchor-obligation-empirical-n)
- [Decision tree — pick the depth](#decision-tree--pick-the-depth)
- [Vendor-name placement rule](#vendor-name-placement-rule)

## The five rungs

Every paragraph of an engineering post lives at exactly one of these five rungs. Mixing rungs at the sentence level is rare and deliberate; mixing them inside a paragraph without a transition is a craft failure.

- **R1 — User experience.** What the reader feels when the system is good or bad. Voice is empathetic, often second-person. The only rung where addressing the reader directly is unambiguously appropriate.
  - Exemplar: *"When you're working through a backlog—opening an issue, jumping to a linked thread, then back to the list—latency isn't just a metric. It's a context switch."* — GitHub Issues navigation (`raw/articles/017-…from-latency-to-instant…md:53`).
- **R2 — Business / product framing.** Why the organisation invested engineering time. Headcount, fleet size, opportunity cost, lineage. Voice is institutional first-person plural.
  - Exemplar: *"When the code you ship serves more than 3 billion people, even a 0.1% performance regression can translate to significant additional power consumption."* — Meta capacity efficiency (`raw/articles/031-…capacity-efficiency…md:97`).
- **R3 — System behaviour.** Externally observable properties with units, percentiles, baselines. Voice is technical-quantitative; adverbs of certainty attach only to numbers.
  - Exemplar: *"Instant: HPC < 200 ms / Fast: HPC < 1000 ms / Slow: HPC >= 1000 ms"* — GitHub Issues (`raw/articles/017-…:65-67`).
- **R4 — Architecture.** Components, boundaries, data flow. Subjects are systems; verbs describe interaction patterns.
  - Exemplar: *"client-side caching layer backed by IndexedDB, added a preheating strategy … and introduced a service worker so cached data remains usable even on hard navigations"* — GitHub Issues (`raw/articles/017-…:54`).
- **R5 — Implementation detail.** Code, library names, exact function calls, named tooling. Subjects are functions, types, files.
  - Exemplar: *"`sntrup761x25519-sha512` … combining a new post-quantum-secure algorithm, Streamlined NTRU Prime, with the classical Elliptic Curve Diffie-Hellman algorithm using the X25519 curve"* — GitHub PQ SSH (`raw/articles/053-…`).

A post does not have to visit every rung, but the *opening rung* and the *closing rung* together encode its archetype: open R1 / close R5 reads as a deep-dive; open R1 / close R1 reads as strategic; live entirely at R5 is documentation; live entirely at R1–R2 is marketing.

## The four-tuple commitment

Before writing prose, commit a tuple:

```
(opening rung, body residency band, closing rung, traversal pattern)
```

Examples:

- Launch deep-dive: `(R2, R3–R5, R2 roadmap, Staircase with R2 anchor)`
- Performance deep-dive: `(R1, R3 + R4 + R5 with R3 re-measurement, R1 return, Staircase braided with Spiral)`
- Migration: `(R2 legacy charity, R4 dominant, R2 dated status, Anchor-and-dive)`
- Postmortem: `(R2 date+scope, R3 + R4 + R5, R2 prevention, Staircase with R2 anchor at both ends)`
- Tutorial: `(R2 use case, R5 dominant, R5 next-steps OR R2 results, Anchor-and-dive into R5)`
- Research translation: `(R2 discipline framing — skips R1, R3 eval + R4 method, R2 acknowledgments, Anchor-and-dive with no R1 visit)`
- AI/agent: `(R2 workload scale, R4 + R5, R1 in book-ended form, Yo-yo)`
- Security: `(R2 threat framing, R5 dominant + thin R4, R2 forward posture, Anchor-and-dive into R5)`

Writing to the tuple is the procedural move. The tuple replaces "discovering it mid-draft."

## Traversal patterns

The corpus surfaces five traversal patterns. Most elite posts braid two or more.

- **A — Staircase (descending ladder).** Monotonic descent R1 → R2 → R3 → R4 → R5, with optional one-paragraph return to R1 in the close. The default pattern because it mirrors the reader's natural curiosity arc: convince me this matters, then teach me how it works.
  - Canonical: GitHub Issues navigation — R1 lede → R2 amplifier → R3 HPC buckets → R4 IndexedDB+service-worker → R5 service-worker request-header code → R1 return.
- **B — Yo-yo (book-ended ladder).** R1/R2 → sustained middle at R3–R5 → ascent back to R1 in close. The close is a *deliberate* move that tells the reader the architecture they just learned is, finally, in service of human time.
  - Canonical: Meta capacity efficiency — opens R2 ("3 billion people"), descends through R4 (offense/defense AI-agent platform with FBDetect), closes R1 ("Engineers who spent mornings on defensive triage now review AI-generated analyses in minutes").
- **C — Spiral (iterative re-measurement beat).** Revisit the same R3/R4 anchor multiple times across sections; each loop tightens on a new fix. The genre's defining shape for performance deep-dives.
  - Canonical: Datadog network-latency — five fixes, each closing with the same cadence ("This was a slight improvement, but clearly still higher than normal").
  - Licensed only when the work is *investigative* (no foreknowledge of destination). Reads fake if applied to teleological work.
- **D — Anchor-and-dive (depth-first / pillar-and-spike).** Brief R2 opening (one to three paragraphs), then sustained R3–R4 residency with R5 in named tooling. Licensed by audience model (publisher's reader is presumed already convinced) and archetype (migration tolerates depth).
  - Canonical: Datadog "Breaking up a monolith" — R2 charity ("limps along for as long as it possibly can — and then some"), then sustained R4 through the three-phase plan.
- **E — Sidebar interlude.** R3–R5 chain pauses to drop in a *labelled* R2 or pure-explanation block, then resumes. Diátaxis would forbid this; engineering-blog craft permits it when explicitly labelled.
  - Canonical: "A crash course on JIT in Postgres" sidebar inside the Postgres segfault deep-dive.

**Braiding is normal.** Meta WebRTC migration runs anchor-and-dive at the document level but spiral inside its dual-stack difficulty disclosures. Datadog Rust storage runs staircase at the document level but injects a Gen 1 → Gen 5 R2 lineage interlude before reaching the Gen 6 R4 architecture.

## Per-archetype default depth profile

| Archetype | Opening rung | Body residency | Closing rung | Default traversal |
|-----------|-------------|----------------|-------------|------------------|
| Performance case study | R1 (felt experience) | R3 → R4 → R5 with R3 re-measurement after each R4 move | R1 return | Staircase, often braided with Spiral |
| Launch deep-dive | R2 (scale or lineage) | R3 → R4 → R5 with R3 headline number in lede | R2 roadmap | Staircase with R2 anchor |
| Architecture migration | R2 (legacy charity) | R4 dominant; R3 only as phased tracking metrics; R5 only in named tooling | R2 dated status snapshot | Anchor-and-dive |
| Incident postmortem | R2 (date + scope) | R3 (timestamps, duration) → R4 (causality walk) → R5 (named commit, package version) | R2 prevention | Staircase with R2 at both ends |
| Developer tutorial | R2 (use case + promise) | R5 dominant; R3 surfaces to prove the lesson | R5 next-steps OR R2 results | Anchor-and-dive into R5 |
| Research-to-product translation | R2 (discipline framing — skips R1) | R3 (evaluation tables) → R4 (method overview) | R2 acknowledgments + footnote | Anchor-and-dive without R1 visit |
| AI / agent post | R2 (workload scale) | R4 (agent platform components) → R5 (specific skills) | R1 in book-ended form | Yo-yo |
| Security post | R2 (threat framing) | R5 dominant (algorithm names, exact commands) + thin R4 mechanism walk | R2 forward security posture | Anchor-and-dive into R5 |

**Sibling genres (refuse these in this skill):**

- Strategic essay (R1/R2 only; sustained R1/R2; R2 product positioning; flat — no descent).
- Customer story (R1 testimonial → R5 vendor product names with no R3 bridge → R1 testimonial; **whiplash** when sold as engineering).

A migration post that opens R1 is suspect (the legacy-charity opener is the migration's load-bearing voice move). A tutorial that opens R1 with a user testimonial has misidentified its archetype.

## Rung-whiplash diagnostic

The reader bounces between R1 and R5 inside a single section. A paragraph about user pain followed by a paragraph of code with no R2/R3/R4 bridge.

- **Slop example (whiplash unresolved):** `raw/articles/033-vercel-com-how-scale-ai-unifies-design-and-performance-with-next-js-and-vercel-vercel.md:55-77` — opens R1 ("only three designers"), descends to vendor-product names at R5 ("Preview Deployments, performance analytics, Vercel CLI") in three sentences. Zero R3 evidence; zero R4 walkthrough. 670 words total. The post collapses under the **"remove the vendor's name" test** (see below).
- **Fixed example (whiplash repaired in one paragraph):** `raw/articles/043-research-google-coral-npu-a-full-stack-platform-for-edge-ai.md` opens with a generalised R1 industry claim, then by paragraph two has descended to three specific R2 constraints ("the performance gap … the fragmentation tax … the user trust deficit"), and by paragraph four is at R4 ("RISC-V ISA compliant architectural IP blocks"). Recovery rescues the opener because the descent is *gradual* — no jump exceeds one rung.
- **Subtler whiplash — the missing-rung descent.** The post moves through rungs in order but skips one. Most often, **R3 (system behaviour) is the missing rung**. Diagnosis: the reader is left holding a design they cannot evaluate. Repair: insert an R3 paragraph that states the measurable property in units, with a baseline.
- **Premature R1 ascent in close.** "We rewrote the storage engine in Rust, which means our customers can sleep at night" — skips R2-R4. The strong move is graduated ascent: end R5, name R3 result, name R2 implication, only then close at R1.
- **Vendor-name padding (publisher-specific whiplash).** Vendor product names appear at the R1/R2 *framing* rung instead of the R5 *implementation* rung. Repair: name the abstract capability first, then disclose the specific product.

## The anchor obligation (empirical N)

Cross-referencing the corpus for "how many paragraphs of R4/R5 a post can sustain before re-surfacing":

- **Inside a single paragraph:** at most one rung-shift, and the shift must be motivated by a transition sentence or section break. Abrupt rung-shifts inside a paragraph are usually a craft failure — the reader has to recalibrate mid-sentence.
- **Inside a section:** ~3–5 paragraphs of R4/R5 before re-anchoring at R3 (a number) or R2 (a user/business referent).
- **Across sections:** every section heading is itself an anchor checkpoint — the heading restates an R2 motivation, names an R3 result, or labels an R4 component. Pure-R5 sections without an anchor heading are rare and read as missing structure.
- **Performance deep-dives** are the strictest — they re-measure (R3) *after every intervention*, making the spiral's anchor N effectively ≤ 2 paragraphs per loop.
- **Architecture migrations** are the loosest — phased-plan structure lets them sustain R4 across 8–15 paragraphs between R2 anchors, because each phase heading is implicitly an R2 anchor.
- **Research translations** are paradoxical — they often sustain R3 (eval tables) without ever surfacing to R1, because the audience expects academic-gloss residency.

**Operational rule of thumb:** if a draft has more than four consecutive R4/R5 paragraphs without surfacing to R3 (a metric) or R2 (a user/business referent), the next paragraph must either re-anchor or open a new section with an anchored heading.

## Decision tree — pick the depth

Run in order; the tree's output is the four-tuple.

1. **Q1 — Audience.** Mixed-readership (engineers + decision-makers, public landing-page traffic) → mandatory R1 anchor in opener *and* closer. Peer-engineer only → R2 anchor sufficient; R1 optional.
2. **Q2 — Publisher credibility on this topic.** Publisher established the *why* in prior posts → Anchor-and-dive licensed. Publisher never published on this topic → Staircase required to earn the descent.
3. **Q3 — Work nature.** Investigative (no foreknowledge of destination) → Spiral. Teleological (planned outcome) → Staircase or Anchor-and-dive. Mixed → braided.
4. **Q4 — Artifact obligation.** Owes user-perceived impact (perf, AI/agent) → R1 anchor mandatory in opener and closer; Yo-yo is the safest. Owes organisational-impact (capacity, scale, migration) → R2 anchor sufficient.
5. **Q5 — Closing handoff.** Operational change (perf, postmortem, migration) → close at R2 status snapshot or R3 distribution result. Advertised capability (launch, research) → close at R2 roadmap or footnote. User-time recovery (AI/agent) → close at R1.
6. **Q6 — Length budget.** ≤2,000 words → pick one rung band (R2–R3 or R4–R5) and stay; trying to traverse all five guarantees whiplash. 3,000–5,500 words → full staircase or spiral feasible. ≥5,000 words → braided patterns mandatory or the reader fatigues.

## Vendor-name placement rule

**Vendor product names appear at R5 (implementation), never at R1/R2 (framing).** The rule separates engineering posts from customer stories.

**The "remove the vendor's name" test.** If removing the vendor's name from every sentence still leaves a coherent post, the post has engineering depth. If the post collapses without the vendor's name, it is marketing in engineering-blog clothing.

- Engineering: "We selected a CDN with edge compute support; we use Vercel." (vendor name at R5; capability described first.)
- Marketing: "Vercel's Preview Deployments enabled our designers to ship faster." (vendor name at R1/R2; capability collapses without it.)

Apply the test as the **final pre-publish gate** for any post written from a vendor or hosted platform.
