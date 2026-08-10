# Skill Audit — spec compliance + doctrine

The exit gate for a created or refactored skill. Walk it against the final output and mark every item **Pass** or **Fail** in a written audit — item by item, no summarizing, no skipping sections. Fix every Fail and re-audit; the work is done only when every item reads Pass. Doctrine terms in bold (**no-op**, **negation**, **leading word**…) are defined in `glossary.md` — read the entry before marking an item you cannot state the rule for.

## Part A — Doctrine audit

The doctrine of `SKILL.md` in checkable form. A skill that satisfies the spec but fails this part is not done.

### A1. Invocation & description

*   [ ] **Invocation earned:** the skill is model-invoked only because the agent (or another skill) must reach it on its own; otherwise `disable-model-invocation: true` is set and the description is a one-line human-facing summary.
*   [ ] **Leading word front-loaded:** the description opens with the skill's **leading word**, doing its invocation work.
*   [ ] **One trigger per branch:** every trigger names a genuinely distinct **branch**; synonym triggers restating one branch are collapsed.
*   [ ] **Triggers only:** the description carries no identity prose that already lives in the body.

### A2. Information hierarchy

*   [ ] **Content typed:** every block is a **step** (ordered action ending on a completion criterion) or **reference** (consulted on demand). A flat reference peer-set is a valid shape for a whole skill — no step sequence fabricated to look procedural.
*   [ ] **Completion criteria:** every step's criterion is checkable (done vs. not-done is decidable) and, where it matters, exhaustive ("every X accounted for", not "produce a list").
*   [ ] **Disclosure by branch:** material every branch needs is inline; material only some branches reach is disclosed behind a **context pointer**.
*   [ ] **Pointers worded for when:** every pointer names its trigger condition; must-fire pointers say "in full"; no hedged "see X for more" remains.
*   [ ] **Co-location:** each concept's definition, rules, and caveats sit under one heading, not scattered across the file.

### A3. Pruning — run sentence by sentence, not section by section

*   [ ] **Single source of truth:** no meaning lives in two places; inline content duplicating a reference is trimmed to a ≤3-line tripwire.
*   [ ] **Relevance:** every line still bears on what the skill does.
*   [ ] **No-op hunt:** the **no-op** test was run on each sentence in isolation ("does it change behaviour versus the default?"); failing sentences were deleted whole, not trimmed.
*   [ ] **Negation:** prohibitions are rephrased as the positive target behaviour; any that remain are hard guardrails paired with what to do instead.
*   [ ] **Leading words:** qualities restated across the text are collapsed into single pretrained words, each strong enough to pass the no-op test.

## Part B — Spec compliance (agentskills.io)

### B1. Metadata & discovery

*   [ ] **Naming:** the `name` field is 1-64 characters, lowercase, only numbers or single hyphens, and exactly matches the parent directory name.
*   [ ] **Description length:** under 1,024 characters.
*   [ ] **Trigger coverage:** the description includes both positive triggers ("Use when…") and negative triggers ("Don't use for…").
*   [ ] **Third-person tone:** the description avoids "I", "me", "my", "you", "your".

### B2. File structure & paths

*   [ ] **Standard folders, flat:** only `scripts/`, `references/`, and `assets/`, each exactly one level deep.
*   [ ] **No human docs:** no README.md, CHANGELOG.md, or installation guides inside the skill.
*   [ ] **Forward slashes:** all paths in SKILL.md use `/` regardless of OS.
*   [ ] **Explicit helper paths:** bundled helpers are referenced unambiguously from the agent's working directory (the same path the agent will type), never bare `scripts/…` that silently depends on CWD.
*   [ ] **No orphans:** every bundled file is reachable from SKILL.md by a pointer.

### B3. Body & scripts

*   [ ] **Lean body:** SKILL.md is under 500 lines (the spec ceiling — the doctrine's **sprawl** bar usually lands far lower).
*   [ ] **Imperative mood:** instructions use the third-person imperative ("Extract," "Run," "Validate").
*   [ ] **Domain-native terms:** uses the domain's own vocabulary consistently (e.g., "component" instead of "file").
*   [ ] **CLI design:** scripts are tiny single-purpose CLIs taking arguments; descriptive `stdout` on success, `stderr` on failure, so the agent can self-correct.
*   [ ] **Helper roles:** each referenced helper is labeled read-only, bootstrap, or mutating in SKILL.md.
*   [ ] **Failure states:** where the skill runs scripts or has known failure states, SKILL.md says how to recover — and a skill with neither carries no error-handling boilerplate.
