---
name: writing-agents-md
description: Author lean AGENTS.md/CLAUDE.md instruction files — resident system-prompt context where every line pays rent. Use when writing an agent instruction file from scratch, auditing or trimming a bloated one, or gating whether a new rule earns residence and at which scope. Don't use for on-demand skills (use writing-skills) or human-facing docs and READMEs.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
  credits: Doctrine adapted from writing-great-skills (mattpocock/skills)
---

# Writing AGENTS.md

Keep agent instruction files lean enough that every rule in them binds.

## Physics: resident context

An AGENTS.md or CLAUDE.md (COPILOT.md, .cursorrules — any harness instruction file) is not documentation. It is a fragment of the system prompt: the harness injects it verbatim into every session, before the task is known, and it never unloads. A skill loads on demand; this file is **resident**. Three consequences govern everything below:

- **Every line taxes every task.** A rule relevant to 5% of sessions spends attention in the other 95%.
- **Rules dilute each other.** Instruction-following is a budget: the more rules resident, the weaker each one binds. A bloated file doesn't enforce more — it enforces less, and the critical rule drowns in the noise.
- **The reader is a frontier model.** It already knows git, testing, the language's idioms, and general best practice, and it can read the codebase. It needs decisions, not knowledge, and it parses a crisp rule without an example.

The file that survives these constraints is a **delta**: the difference between stock agent behavior and what this project needs. Everything that isn't delta is rent paid for nothing.

## The rent test

Run on every line written, kept, or proposed. A line stays only if all three hold; a line that fails is deleted or relocated, never softened.

1. **Delta** — it changes what the agent would otherwise do. Restated defaults, general best practices, and motivational prose fail here.
2. **Frequency** — it bears on most sessions at its scope. A rule only some tasks need moves down the scope ladder instead.
3. **Economy** — keeping it resident is cheaper than deriving it on demand. A non-obvious test invocation passes (saves a search every session); anything the toolchain announces on its own — type errors, lint output, failing CI — fails.

## Scope ladder

Place each rule at the narrowest scope that still covers the tasks needing it:

| Scope | Resident for | Belongs there |
|---|---|---|
| Global (`~/.claude/CLAUDE.md`) | every project | cross-project workflow rules — the highest bar |
| Repo root file | every task in the repo | commands, repo-wide tripwires, deviations from convention |
| Subtree file (`<dir>/AGENTS.md`) | work inside that subtree | area-specific rules, loaded only when the agent touches the area |
| Skill | on trigger | procedures and multi-step workflows |
| Linked doc | on demand | reference some tasks need; word the pointer for when to read it |

Two mechanics decide placement more than anything else: nested files load lazily, so the directory tree is the progressive disclosure these files natively support. And in Claude Code, an `@path` import expands at load time — resident, full rent — while a plain path mentioned in prose is read only on demand.

## What earns residence

- Commands that can't be guessed: build/test/run invocations with their required flags and env.
- Deviations from convention: the places this repo does X where the ecosystem default is Y.
- Tripwires: constraints whose violation is expensive — generated files never edited by hand, protected branches, irreversible commands.
- Domain vocabulary the code uses with a non-obvious meaning.
- Pointers to on-demand material, each worded for when to load it.

## Form

- One rule, one line, imperative, concrete: "Run `make check` before commit", not "ensure quality gates pass".
- State each rule once. A repeated rule doesn't reinforce — it competes with its copy, and the copies drift.
- Phrase the positive target; keep a prohibition only for tripwires, paired with what to do instead.
- Add a why-clause (one clause, not a paragraph) only where a rule reads arbitrary and agents keep "correcting" it: "Never edit `gen/` — regenerated on build."
- Reserve emphasis (IMPORTANT, NEVER, caps) for the one or two rules whose violation is catastrophic. Emphasis on everything is emphasis on nothing.
- A clear rule needs no example. If a rule seems to need one, sharpen the rule until it doesn't — an example pays double rent and pins the rule to one surface form.
- Group rules by concern under flat headings, tripwires first.
- When the root file outgrows a screenful (~60 lines), run the Trim branch before adding anything.

## Branches

**Write — a new file for a repo:**

1. Collect delta candidates from the repo itself: manifests, Makefile/CI config for real commands; layout for deviations from convention; history and existing docs for tripwires.
2. Draft each candidate per Form and place it on the scope ladder.
3. Run the rent test line by line; delete or relocate failures.

*Done when:* every line passes all three rent tests at its chosen scope.

**Trim — an existing file:**

1. Verdict every line: keep / rewrite / relocate (with target scope) / evict (with the failed test). Flag contradictory rules for the user to resolve — never silently pick a side.
2. Present the verdict table, then apply it: rewrite survivors per Form, create subtree files and pointers for relocations.

*Done when:* every original line has a verdict and the surviving file passes the Write bar.

**Gate — a single new rule, usually promoting a chat correction:**

1. Phrase the candidate as one positive imperative line.
2. Run the rent test and pick its scope-ladder rung.
3. Search the file for a rule it duplicates or contradicts and update that rule in place — appending is how sediment forms.

*Done when:* the rule lives at exactly one scope, exactly once.

## Failure modes

- **Scar tissue** — a rule added after one incident, resident forever. The Gate branch's duplicate-search is the immune response; retest neighboring rules while there.
- **Mirror documentation** — the file restates what the code already shows (architecture tours, file inventories). Fails the delta test wholesale.
- **Example creep** — good/bad code blocks multiplying under each rule. Cut the blocks, sharpen the rules.
- **Emphasis inflation** — the caps arms race; see Form. Ends with nothing binding.
- **Harness re-instruction** — re-explaining tool use or response format the harness's own system prompt already governs. Pure rent; evict on sight.
