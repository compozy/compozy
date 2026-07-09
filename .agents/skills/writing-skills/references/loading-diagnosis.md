# Loading Diagnosis — references the agent ignores

Diagnosis and repair for SKILL.md files where the body is the only thing the agent ever reads: the `references/` folder might as well not exist. A reference file the agent never loads is a reference file that does not exist.

## Contents

- The failure mode
- Symptoms
- The pointer-strength ladder
- The fixes, in order
- Anti-patterns (things that look like fixes but aren't)
- The live test
- Repair checklist

## The failure mode

Large skills ship rich reference files that hold the load-bearing detail — full ARIA patterns, banned-vocab lists, before/after examples — and a SKILL.md body that summarizes them. The agent reads the body, feels it has "enough," and never fans out. This is the **inline-substitutes-for-reference** antipattern: the inline summary competes with the file it points at, and wins.

The cure is not more scaffolding — it is collapsing the competition (one source of truth) and sharpening the pointer's wording. The doctrine's rule governs: a context pointer's *wording*, not its target, decides when and how reliably the agent reaches the material. A must-have target behind a weakly worded pointer is a variance bug — fix the wording first; inline the material only if sharpening fails.

## Symptoms (how to spot it)

A SKILL.md likely has the problem if two or more apply:

- The body inlines checklists, tables, or lists that also live in a referenced file (the duplication that lets the agent skip the file).
- References are introduced with hedged phrases: *"For depth, read X"*, *"see X for details"*, *"X has more"*.
- No task→file mapping exists anywhere — neither step-local pointers nor a router row tells the agent *when* a file is mandatory.
- The same pointer set is stated in several places at once (router table + reference index + per-step directives) — duplication that inflates some pointers and buries others.
- Reference files are linked from other reference files (nesting), not directly from SKILL.md.
- Reference files over 100 lines have no `## Contents` section at the top.
- A file in the bundle has no pointer at all (**orphaned** — it can never fire).

## The pointer-strength ladder

Grade every pointer in the skill, then match strength to stakes:

- **WEAK** — a bare link or descriptive mention with no read directive. Only acceptable for genuinely optional background.
- **MEDIUM** — imperative with a trigger condition: *"When registering a bug, read `references/bug-registry.md`."* Right for optional-branch references.
- **STRONG** — imperative + trigger + full-read + consequence: *"STOP. Read `references/accessibility-floor.md` in full before implementing or reviewing any interactive widget — the inline bullets are a tripwire, not the contract."* Reserve for **must-fire** references: files whose skipped load corrupts the output (verdict contracts, output schemas, safety gates).

Classify each reference as *must-fire* (the run is wrong without it) or *optional-branch* (only some runs need it). A must-fire behind WEAK or bare-MEDIUM wording is the bug this file exists to catch; an optional-branch behind MEDIUM is correct and needs no escalation.

## The fixes, in order

### Fix 1 — One pointer per reference, worded for when

Give every reference exactly one home, at its point of use, whose wording names the trigger. Two validated shapes:

- **Step-local pointers** for sequential skills: each step opens with the read directive for the file that owns its contract.
- **A compact router table** for flat peer-set skills (doctrine catalogs, per-library dispatchers): one table mapping *"When you are…" → "Read in full"*, with a preface that binds it (*"match the task, read the listed file in full before producing output"*).

Stating the same pointer in a router *and* an index *and* a per-step directive is duplication — collapse to the one home that fires at the right moment.

### Fix 2 — Escalate must-fire pointers

Walk the must-fire list and raise each pointer to STRONG (or MEDIUM + "in full" where the step is already imperative and local). Name the trigger condition ("when implementing an interactive widget"), never "if you want depth." State that the inline content is a tripwire, not the source of truth.

### Fix 3 — Trim duplicated inline content to gist tripwires

When a step carries a 10-bullet checklist that also lives in a reference, the agent reads the checklist and skips the file. Cut the inline to a 2-3 line **tripwire** — enough to detect violations while scanning, obviously incomplete — and pair it with the sharpened pointer. If the inline list and the reference cover the same ground, the reference always wins.

### Fix 4 — Flatten reference depth and add TOCs

Link every `references/*.md` directly from SKILL.md, one level deep — when references link to references, the agent partial-reads (`head -100`) and misses content. Give any reference over 100 lines a `## Contents` section at the top, so partial reads still surface its scope.

## Anti-patterns (things that look like fixes but aren't)

- **Adding "(important)" or emoji to trigger lines.** The agent reads verbs, not decoration — mandatory phrasing requires *"Read… in full before…"*, not adjectives.
- **Shortening references so the body can stay inline.** The opposite of the goal: references grow as the body shrinks.
- **Adding a TL;DR at the top of each reference.** The agent reads the TL;DR and skips the body. A Contents TOC lists section names, not summaries.
- **Renaming files to suggest urgency** (`MUST-READ.md`). The filename never changes the read decision; the pointer wording in SKILL.md does.
- **Restoring heavyweight scaffolding everywhere.** A mandatory router + index + STOP on every step re-creates the pointer duplication of Fix 1 and inflates prominence past real rank. Escalate the risky sites; leave correct MEDIUM pointers alone.

## The live test

Ask a fresh agent instance to perform a task that requires a specific reference file, and observe whether it actually reads the file before producing output. If it doesn't, the pointer for that file is too weak — strengthen the trigger condition and retest. This is the only ground truth; wording debates are settled by running, not arguing.

## Repair checklist

- [ ] Every reference and asset has exactly one pointer, placed at its point of use, whose wording names the trigger.
- [ ] Every must-fire pointer says "in full" (or STOP-grade wording); no must-fire sits behind a bare link.
- [ ] No hedged phrasing remains (*"for depth, see X"*).
- [ ] Inline content duplicating a reference is trimmed to a ≤3-line tripwire labeled as such.
- [ ] No reference links to another reference; every file is reachable directly from SKILL.md.
- [ ] Reference files > 100 lines carry a `## Contents` TOC.
- [ ] No orphaned bundle files — anything unreachable is wired in, labeled author-tooling, or removed.
- [ ] The live test passes for at least one must-fire reference.
