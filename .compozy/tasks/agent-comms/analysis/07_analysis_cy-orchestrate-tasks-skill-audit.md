# `cy-orchestrate-tasks` Skill Audit

Audit scope: the rebase adaptation from the removed public spawn surface to typed agent calls.

## Part A — Doctrine

- **A1 Invocation earned — Pass.** The bundled Loop must invoke this skill without operator discovery.
- **A1 Leading word front-loaded — Pass.** The description opens with “Conducts”.
- **A1 One trigger per branch — Pass.** One orchestration branch and one explicit exclusion set remain.
- **A1 Triggers only — Pass.** The description contains dispatch criteria, not body-level identity prose.
- **A2 Content typed — Pass.** The body is an ordered execution workflow plus bounded reference rules.
- **A2 Completion criteria — Pass.** Every workflow step ends with an observable done condition.
- **A2 Disclosure by branch — Pass.** The single branch keeps its required contract inline.
- **A2 Pointers worded for when — Pass.** Worker skills are named at the point where the worker must load them.
- **A2 Co-location — Pass.** Runtime selection, call admission, wait, proof, stop, and reporting rules each have one owner section.
- **A3 Single source of truth — Pass.** Worker briefing content is defined once and referenced by the call step.
- **A3 Relevance — Pass.** Every line contributes to graph order, containment, evidence, cleanup, or reporting.
- **A3 No-op hunt — Pass.** Redundant spawn-era prose and duplicate call-admission text were removed.
- **A3 Negation — Pass.** Remaining prohibitions are hard boundaries paired with the required action.
- **A3 Leading words — Pass.** “Conductor”, “call”, “proof”, and “stop” anchor the execution model.

## Part B — Specification Compliance

- **B1 Naming — Pass.** `name: cy-orchestrate-tasks` matches its directory and allowed syntax.
- **B1 Description length — Pass.** The description is below 1,024 characters.
- **B1 Trigger coverage — Pass.** Positive “Use when” and negative “Do not use” triggers are present.
- **B1 Third-person tone — Pass.** Metadata contains no first- or second-person pronouns.
- **B2 Standard folders, flat — Pass.** The skill contains only `SKILL.md`; no non-standard directory exists.
- **B2 No human docs — Pass.** No README, changelog, or installation guide exists in the skill.
- **B2 Forward slashes — Pass.** Every path uses `/`.
- **B2 Explicit helper paths — Pass.** The skill has no bundled helper script with an ambiguous path.
- **B2 No orphans — Pass.** The skill has no bundled file outside `SKILL.md`.
- **B3 Lean body — Pass.** `SKILL.md` is 123 lines, below the 500-line ceiling.
- **B3 Imperative mood — Pass.** Workflow actions use direct imperative verbs.
- **B3 Domain-native terms — Pass.** Task graph, call, child session, result, and task frontmatter match runtime contracts.
- **B3 CLI design — Pass.** No bundled CLI script exists; runtime commands use the shipped `compozy` CLI.
- **B3 Helper roles — Pass.** No helper script exists that requires a role label.
- **B3 Failure states — Pass.** Ambiguous admission, timeout, failed proof, and failed stop each name their recovery or blocked outcome.

All items pass.
