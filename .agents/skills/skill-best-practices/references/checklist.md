# Agent Skill Validation Checklist

Use this checklist to perform a final audit of the generated skill before deployment. Every item must be marked as "Pass" to ensure the skill is discoverable, lean, and deterministic.

## 1. Metadata & Discovery
*   [ ] **Naming:** The `name` field is 1-64 characters, lowercase, and contains only numbers or single hyphens.
*   [ ] **Directory Match:** The `name` field exactly matches the parent directory name.
*   [ ] **Description Length:** The description is under 1,024 characters.
*   [ ] **Trigger Optimization:** The description includes both use cases ("Use when...") and negative triggers ("Don't use for...").
*   [ ] **Third-Person Tone:** The description avoids "I", "me", "my", "you", or "your".

## 2. File Structure & Paths
*   [ ] **Flat Hierarchy:** All files in `scripts/`, `references/`, and `assets/` are exactly one level deep (no nested subfolders).
*   [ ] **Standard Folders:** Only use `scripts/`, `references/`, and `assets/`.
*   [ ] **No Human Docs:** The directory contains NO `README.md`, `CHANGELOG.md`, or `INSTALLATION_GUIDE.md`.
*   [ ] **Forward Slashes:** All file paths in `SKILL.md` use forward slashes (`/`) regardless of the operating system.

## 3. Logic & Instructions (SKILL.md)
*   [ ] **Lean Context:** The `SKILL.md` file is under 500 lines.
*   [ ] **Imperative Mood:** Instructions use direct commands (e.g., "Extract," "Run," "Validate").
*   [ ] **Deterministic Steps:** The workflow is a numbered, chronological sequence with clear decision trees.
*   [ ] **Progressive Disclosure:** Large schemas, templates, or rule sets are stored in `references/` or `assets/` and read only when needed.
*   [ ] **Specific Terminology:** Uses domain-native terms consistently (e.g., "component" instead of "file").

## 4. Scripts & Determinism
*   [ ] **CLI Design:** Scripts in `scripts/` are designed as tiny CLIs that take arguments.
*   [ ] **Feedback Loop:** Scripts provide descriptive `stdout` for success and `stderr` for failure to allow agent self-correction.
*   [ ] **No Library Code:** Scripts are single-purpose; complex logic is offloaded to the repository's standard CLI or external tools.

## 5. Error Handling
*   [ ] **Edge Cases:** The `SKILL.md` includes an "Error Handling" section addressing common failure states or missing configurations.
*   [ ] **Validation:** The `SKILL.md` includes a step to run validation scripts where applicable.

