---
name: writing-agents-md
description: "Create, audit, shorten, or scope AGENTS.md and CLAUDE.md instructions. Use writing-skills for on-demand skills; excludes human-facing documentation and READMEs."
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
  credits: Doctrine adapted from writing-great-skills (mattpocock/skills)
---

# Writing Agent Instructions

Keep `AGENTS.md`/`CLAUDE.md` focused on project decisions the agent cannot reliably infer from nearby code or tooling. Instruction loading differs by harness; verify which root and nested files apply before counting their context cost.

## Keep, Move, or Remove

- Keep non-obvious commands, expensive tripwires, domain vocabulary, and project-specific ownership/compatibility decisions.
- Move a rule to the narrowest subtree or on-demand skill that needs it. Link long rationale or incident evidence instead of embedding it.
- Remove generic engineering advice, duplicated rules, stale model pins, rhetorical emphasis, fixed response ceremonies, and procedures already owned by the harness or toolchain.
- A rule must name when it applies. Availability of a skill does not justify requiring it for every task. Preserve concrete safety and product invariants while narrowing their procedure.

## Work

1. Read the relevant instructions, current repository contracts, and user decisions. Check recent changes before pruning policy; keep historical evidence separate from active rules.
2. Review rules by concern: keep, rewrite, relocate, or remove. Record material policy changes and relocations in a concise summary; a line-by-line public verdict table is unnecessary unless requested.
3. Resolve contradictions using the user's current direction and the repository's authoritative policy. Ask only if a remaining conflict changes the outcome and has no established owner.
4. Edit the owning file once and update its pointers. Preserve symlinks and tool-generated blocks; avoid parallel copies that drift.
5. Check that important invariants and real commands remain reachable, nested instructions agree, and removed rules are not silently reintroduced by skills or memory. Review links and changed behavior; prose-only edits do not need tests that freeze wording.

Use short, concrete rules with emphasis reserved for costly violations. A command or example earns space when it prevents an actual ambiguity. Optimize the total material loaded for representative tasks, not line count alone or moving an unconditional full read into a reference.

Done when the requested scope has one clear owner per rule, no unresolved policy contradiction, and proportionate validation. Report what changed and any remaining limitations without creating a recurring audit ritual.
