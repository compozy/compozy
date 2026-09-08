# Pre-publication review

Review the actual artifact and claims before external publication. Reuse earlier checks when their inputs remain current; a paragraph edit does not require all archetypes or a new report.

## Common checks

- The title, body, and stated shipping status agree. Facts and attributed quotations are supported; quantitative claims identify a relevant baseline, method, sample/window, and environment where needed.
- Examples are labeled as illustrative when not measured. Tutorial commands are verified in the intended environment; abbreviated deep-dive code identifies omissions.
- Figures are legible, accessible, and correctly attributed; redact secrets and private customer information. Explain what evidence demonstrates without forcing a diagram into every section.
- Respect applicable coordinated disclosure, embargoes, customer notification, and publication authorization. Unresolved disclosure restrictions block publication of the affected material.

## Match evidence to the claim

- **Incident:** accurate impact and timeline, supported root cause, recovery, and prevention status. Include failed mitigations only if they occurred; list planned work honestly and use owners/dates that actually exist.
- **Migration:** explain the changed boundary, relevant compatibility/cutover safeguards, measured results, and current status. Use as many phases as the real migration had.
- **Performance:** disclose what was measured, comparison conditions, and limitations. Tail-latency claims need the relevant distribution; fixed byte counts do not need a latency percentile chart.
- **AI/agents:** distinguish a demonstrated capability from benchmark superiority or causal attribution. Comparative claims need a representative evaluation and baseline; attributing a gain to components needs an ablation or other valid causal evidence. A capability launch can use operational evidence without claiming benchmark gains.
- **Security:** state the threat model, evidence for capabilities and mitigations, residual exposure, and applicable disclosure status. Cite CVE/upstream fixes when they exist and are relevant; do not invent them for general reliability guidance.
- **Tutorial/research/launch:** verify promised runnable behavior, credit the underlying research, and match capability/availability claims to what exists.

## Editorial checks

Use only the diagnostics needed: clarity, pacing, depth transitions, caption usefulness, and publisher requirements. No mandatory archetype metadata, section-count quota, H2 annotation, failed-mitigation paragraph, or callback vocabulary overlap.

`python3 <writing-tech-post-dir>/scripts/lint-post.py <draft.md>` is a read-only heuristic scan. By default, findings are warnings and exit 0; `--strict` makes findings fail when the user or publisher chooses that editorial policy. Review false positives in context. Neither mode checks actual disclosure permission or proves that nearby evidence keywords support a numeric claim.

Report readiness and any material unresolved issue briefly. Do not publish unsupported claims, restricted disclosures, or unapproved external posts merely because a linter passes.
