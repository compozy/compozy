# Engineering Post Diagnostics

Use only the diagnostics relevant to the draft. These are editorial signals, not word bans, archetype gates, or additional publication steps. `pre-publish-checklist.md` owns publication requirements.

## Claims and evidence

- A headline promises a result the body cannot support. Align the claim with the actual measurement and scope.
- A percentage lacks a denominator, baseline, sample/window, or relevant environment. Supply the context needed to interpret that particular metric.
- A mean is offered as proof of tail latency or population-wide improvement. Use a suitable distribution/percentile comparison or narrow the claim; not every performance metric requires percentiles.
- An AI capability comparison lacks a documented evaluation and baseline. Operational examples can establish demonstrated behavior without establishing superiority. Causal component-attribution claims need an ablation or another valid causal design.
- Charts imply improvement through inconsistent units or cropped axes. Code examples lack provenance, context, or clear omission markers. Screenshots hide copyable code rather than showing the relevant UI evidence.
- A diagram or caption adds no information, or lacks useful accessible text. Clarify its actual contribution instead of requiring the same prose wrapper around every figure.

## Structure and reader context

- A generic template displaces the argument. Reuse a clear existing structure; archetype and depth patterns are options.
- The draft jumps from user pain to implementation without explaining the connection. Add the missing mechanism or result where useful; do not count abstraction-level changes or require a particular opening/closing level.
- Vendor names substitute for explanations. Name actual products where relevant and explain how the system works; a vendor-focused article can depend on specific names.
- A section repeats prior material without adding a fact, distinction, or consequence. Cut repetition or make its contribution clear. Headings may name topics, actions, results, or questions as appropriate.
- The ending adds an unsupported roadmap, generic AI promise, or irrelevant CTA. End at the supported result or actual next step; no fixed number of applications, callbacks, or rhetorical questions is required.

## Incident, migration, and security writing

- Personal blame replaces system causality, or defensive wording conceals a contributing failure. Explain mechanisms and ownership accurately.
- A migration percentage has no population, or rollout claims omit remaining scope and compatibility implications. State what shipped and what remains.
- A draft invents failed attempts, residual problems, multi-author credits, or a CVE to fit a genre. Preserve the actual work, attribution, and authorized disclosure; a single successful fix is a valid account.
- Redaction hides a material limit without explanation, or security detail exceeds the agreed disclosure boundary. Explain the permitted scope without exposing withheld material.

Use the relevant specialty reference only when it helps resolve one of these problems. A successful check does not call for a second checklist or new evidence collection for unchanged claims.
