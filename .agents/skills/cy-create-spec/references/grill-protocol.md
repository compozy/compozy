# Focused Spec Questions

Use when a product or design decision remains unresolved after the available evidence is read. Reuse accepted answers and ask only the next decisions needed to proceed; a small set of focused questions usually suffices.

Prefer concrete proposals and trade-offs. Investigate facts locally or through an authorized bounded explorer; do not ask the user questions the code answers. Continue independent work while awaiting a necessary answer. A feature that looks simple still gets the questions its unexamined assumptions need; skip a question because the evidence settles it, not because the feature is small.

## Conduct

- Chase a vague answer with the specific follow-up: "it depends" gets "on what?", "probably" gets pinned to a decision.
- Stress-test a load-bearing decision with one concrete scenario at the boundary between concepts ("two runs claim the same worktree at once; what does the second one see?").
- When the user states how something works, check the code and surface any contradiction before building on the claim.
- Check new or overloaded terms against `docs/_memory/glossary.md`; when the session coins or shifts a meaning, propose the glossary edit before the spec closes.

A current approved direction satisfies earlier checkpoints. Do not force a new interview because a stage or skill started. Record ordinary choices in the spec; use `references/adr-template.md` for hard-to-reverse decisions with meaningful alternatives and rationale.

Product questions address outcomes and scope, surface questions use reviewable CLI/API/UI examples, and technical questions concern unresolved architecture risks. Stop interviewing when the requested artifact can be written correctly. A genuinely blocking product/authorization choice stays explicit; it is not silently assumed.
