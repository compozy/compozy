# Authoring Procedure — creating a new skill

Spec mechanics for the create branch, following the agentskills.io specification. The doctrine in `SKILL.md` governs every content decision (what to inline, what to disclose, how to word descriptions and pointers); this file governs structure and compliance.

## Bundled Path Rule

Resolve bundled paths relative to the directory that contains the `writing-skills` SKILL.md. When invoking a helper from another working directory, expand `<writing-skills-dir>` to that directory first so the command is unambiguous.

## Step 1: Initialize and Validate Metadata

1. Define a unique `name`: 1-64 characters, lowercase, numbers, and single hyphens only. It must exactly match the skill's directory name.
2. Draft a `description`: max 1,024 characters, written in the third person, with positive triggers ("Use when…") and negative triggers ("Don't use for…"). Word it per the doctrine's *Writing the description* section — front-loaded leading word, one trigger per branch.
3. **Execute validation script (read-only helper):**
   `python3 <writing-skills-dir>/scripts/validate-metadata.py --name "[name]" --description "[description]"`
4. If the script returns an error, self-correct the metadata based on the `stderr` output and re-run until successful.

*Done when:* the validator exits 0.

## Step 2: Structure the Directory

1. Create the root directory using the validated `name`.
2. Use only these subdirectories, each flat (one level deep):
   - `scripts/`: tiny CLI tools and deterministic logic.
   - `references/`: on-demand context — schemas, catalogs, deep contracts.
   - `assets/`: output templates, JSON schemas, static files.
3. The skill ships agent-facing files only — SKILL.md plus the three folders; human docs (README, CHANGELOG, installation guides) live outside the skill.

*Done when:* the tree contains SKILL.md and only the standard folders.

## Step 3: Draft SKILL.md

1. Start from `assets/SKILL.template.md`.
2. Write all instructions in the **third-person imperative** ("Extract the text," "Run the build").
3. Keep SKILL.md under 500 lines (the spec ceiling — the doctrine's *sprawl* bar usually lands far lower). Decide what sits inline vs. behind a pointer using the doctrine's *Information hierarchy* section, and word every pointer so it encodes when to load the file.
4. End each step on a checkable completion criterion (*Done when:* …).

*Done when:* every doctrine section that bears on the content has been applied — hierarchy, pruning, leading words, description, negation.

## Step 4: Identify and Bundle Scripts

1. Identify "fragile" tasks (regex, complex parsing, repetitive boilerplate) and outline a single-purpose script for `scripts/` for each.
2. Scripts communicate through standard streams: descriptive `stdout` on success, `stderr` on failure, so the agent can self-correct.
3. In SKILL.md, reference each helper by a path that is unambiguous from the agent's working directory (the same path the agent will type), never a bare `scripts/helper.py` that silently depends on CWD.
4. Label each helper as **read-only**, **bootstrap**, or **mutating** so downstream agents know whether it inspects state or changes it.

*Done when:* every bundled helper has an unambiguous invocation path and a role label in SKILL.md.

## Step 5: Final Validation

1. Review SKILL.md for "hallucination gaps" — points where the agent is forced to guess.
2. Verify all file paths are relative and use forward slashes (`/`).
3. Read `references/checklist.md` and cross-reference every checklist item against the final output; ship only when every item passes.

*Done when:* the checklist is fully satisfied.

## Error Handling

- **Metadata failure:** identify the specific validator error (e.g., "STYLE ERROR") and rewrite the field — remove first/second-person pronouns, shorten, or fix the name charset.
- **Context bloat:** if the draft exceeds 500 lines, extract the largest block that only some branches need and move it to `references/`, leaving a pointer worded for when to load it.
