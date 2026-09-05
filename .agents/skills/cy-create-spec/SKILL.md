---
name: cy-create-spec
description: Create or update a requested CompozyOS spec and its applicable companion contracts, reusing current decisions and research. Not a prerequisite for ordinary coding tasks.
---

# Create Spec

Use for a requested specification, not as a prerequisite to an ordinary fix. Author `_spec.md` with Product and Technical parts and the applicable companion contracts under `.compozy/tasks/<slug>/`.

1. Reuse the user's decisions, existing spec, and current research. Inspect relevant code/contracts for unresolved questions. Market research is needed only for a market/product uncertainty or an explicit request; delegation is optional and bounded.
2. State the motivating problem, observable outcome, scope, and non-goals. Ask focused questions only for decisions the evidence cannot settle; apparent simplicity is not evidence. An approved direction satisfies the product checkpoint; do not repeat an interview or approval already completed. For an open-ended product interview, use the relevant branch of `references/grill-protocol.md`.
3. Write Part I and `_user_stories.md` from `references/spec-template.md` and `references/user-stories-template.md`. Product requirements may name a technology when it is an actual public contract or fixed user constraint; incidental implementation belongs in Part II.
4. Define the changed public surface before its internal implementation: `_dx.md` uses `references/dx-template.md`; UI-bearing work uses `_uiux.md` and `references/uiux-template.md`. For an internal-only spec, a brief no-public-surface entry is sufficient. Preserve named visual references and production owners.
5. Write Part II with the concrete contracts the design changes. Read only relevant ADRs/analysis. Use `references/adr-template.md` for consequential decisions with real alternatives; a routine choice needs no standalone ADR. Include SD-013 compatibility for changed user state/public surfaces, delete targets, and one owning impact analysis with links from companions.
6. Write `_tests.md` using `references/tests-template.md`: distinct invariants, owning suites, inputs/results, and necessary integration journeys. Reuse existing coverage; no quotas per component, error class, or layer.
7. Check applicable spec markers with `cy-spec-preflight`, contract consistency, and the earliest useful end-to-end outcome. Choose slice count from dependencies/risk; `slice_budget` is a planning preference, not a forced split or permission loop.
8. Save reviewable files and summarize unresolved decisions. After the user approves the saved spec, offer optional `cy-spec-peer-review`; apply only selected findings. Do not start implementation merely because spec writing is complete.

Templates are section guides: keep shared outcome/contract content, omit irrelevant sections or record a short reason where a downstream schema requires a slot. Read each reference when its branch is used, not the whole package. Maintain a concise File References index naming contract inputs and why each matters; tasks copy their relevant subset. Preserve requested scope; narrowing an accepted motivating problem requires the user's recorded decision.

For updates, edit affected sections/companions only. Missing or contradictory inputs are resolved from user decisions and current policy; report a genuinely blocking missing contract without inventing it.
