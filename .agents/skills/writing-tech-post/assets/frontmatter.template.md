# Frontmatter template

Publish-time metadata block. Drop this at the top of any draft authored with `writing-tech-post`. The skill validates the draft against the values committed here.

```yaml
---
title: [POST TITLE — should match the headline conventions in narrative-and-pacing.md]
slug: [kebab-case-slug]
authors:
  - name: [Author 1]
    role: [Engineer / Staff Engineer / Principal SRE]
  - name: [Author 2]
    role: [...]
acknowledgements: [Optional — for cross-team contributions or external researchers]

# Archetype commitment (gates every later phase)
archetype:
  primary: [launch | postmortem | migration | performance | tutorial | research-translation | ai-agent | security]
  absorbed: [optional secondary archetype if hybrid]
  hybrid-note: [one sentence if hybrid — e.g., "launch + migration: the lineage section is structural ornament; the launch contract is load-bearing"]

# Audience commitment
audience:
  rung-target: [product-user | engineer-adopter | peer-engineer-deep | infra-or-research-peer]

# Depth four-tuple (commit before drafting prose)
depth-tuple:
  opening-rung: [R1 | R2 | R3 | R4 | R5]
  body-residency: [e.g., "R3 → R4 → R5 with R3 re-measurement"]
  closing-rung: [R1 | R2 | R3 | R4 | R5]
  traversal: [staircase | yo-yo | spiral | anchor-and-dive | sidebar-interlude | braided]

# Publisher voice target
voice:
  publisher: [datadog | vercel | github | aws | meta | cloudflare | jane-street | canva | docker | slack | tailscale | other]
  register: [systems-pragmatic | product-tight | team-narrative | deliberate-measured | cross-organisational | technical-confident | precise-academic]

# Length budget
length-band:
  estimated-words: [number]
  archetype-band: [e.g., "5,000–8,000 for launch deep-dive"]

# Evidence forms declared upfront
evidence-forms:
  - [architecture-diagram | sequence-diagram | flowchart | data-flow-diagram | before-after-migration-diagram | code-snippet | shell-session | assembly | chart | distribution-chart | table | screenshot | embedded-quote | named-benchmark-table | ablation-matrix | agent-trace | role-graph | knowledge-pyramid | eval-harness-evolution | structured-output-schema | alert-screenshot]

# Disclosure layer (per archetype)
disclosure:
  blameless-register: [required-for-postmortem | not-applicable]
  coordinated-disclosure: [required-for-cve-response | not-applicable]
  paper-link-first: [required-for-ai-agent-or-research | not-applicable]
  what-wed-do-differently: [required-for-migration-or-retrospective | not-applicable]
  vendor-naming-discipline: [audit-required]

# Closing register
closer:
  shape: [call-to-build | call-to-adopt | open-question | shipping-status-roadmap | prevention-list | distribution-chart-close]

# Pre-publish gate status
status: [drafting | outline-review | evidence-pass | voice-pass | narrative-pass | pre-publish-gate | publishable | hold-for-review | rework]

# Optional: SEO / metadata
meta:
  description: [≤160 char description — first 200 words of the lede compressed]
  tags: [optional list]
---
```
