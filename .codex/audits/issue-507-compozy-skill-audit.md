# Compozy skill audit

Scope: `skills/compozy/SKILL.md` and the changed `references/runtime-operations.md`.

## Part A — Doctrine audit

- Pass — Invocation earned: runtime operations must be discoverable without manual skill naming.
- Pass — Leading word front-loaded: the description opens with “Operate CompozyOS.”
- Pass — One trigger per branch: the description names distinct CompozyOS operating surfaces.
- Pass — Triggers only: the description contains triggers and one negative boundary.
- Pass — Content typed: the body is a router and operating-loop steps; runtime detail stays in references.
- Pass — Completion criteria: the operating loop requires structured confirmation after every mutation.
- Pass — Disclosure by branch: task-specific details are routed to matching reference files.
- Pass — Pointers worded for when: the router says which files must be read in full for each task.
- Pass — Co-location: session prompting and lifecycle boundaries remain together in runtime operations.
- Pass — Single source of truth: the changed behavior is stated once in the runtime reference; the router only points to it.
- Pass — Relevance: every changed sentence defines public Web prompt or lifecycle authority.
- Pass — No-op hunt: each changed sentence alters eligibility, available controls, or an exclusion.
- Pass — Negation: remaining prohibitions are authority guardrails paired with the allowed prompt controls.
- Pass — Leading words: “prompt authority” and “lifecycle authority” keep the boundary compact and stable.

## Part B — Spec compliance

- Pass — Naming: `compozy` matches its directory and allowed name syntax.
- Pass — Description length: the metadata validator accepted the description under 1,024 characters.
- Pass — Trigger coverage: the description includes “Use when” and “Don't use”.
- Pass — Third-person tone: the description contains no first- or second-person pronouns.
- Pass — Standard folders, flat: the bundle contains only `SKILL.md` and one-level `references/` files.
- Pass — No human docs: no README or changelog exists in the skill bundle.
- Pass — Forward slashes: every bundled path uses forward slashes.
- Pass — Explicit helper paths: no bundled helper is invoked by this skill.
- Pass — No orphans: every bundled reference appears in the router or reference index.
- Pass — Lean body: `SKILL.md` is 78 lines.
- Pass — Imperative mood: operating instructions use direct imperative verbs.
- Pass — Domain-native terms: session, prompt, daemon, lifecycle, and runtime match CompozyOS vocabulary.
- Pass — CLI design: no script is bundled.
- Pass — Helper roles: no helper is bundled.
- Pass — Failure states: the error-handling section names the stop-and-report path for missing references and runtime failures.

Metadata validation: `SUCCESS: Metadata is valid and optimized for discovery.`
