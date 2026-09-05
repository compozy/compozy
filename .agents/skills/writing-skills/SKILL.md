---
name: writing-skills
description: "Create, refactor, shorten, or debug skills, descriptions, and reference loading. Use writing-agents-md for AGENTS.md/CLAUDE.md; excludes general documentation and READMEs."
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
  credits: Doctrine adapted from writing-great-skills (mattpocock/skills)
---

# Writing Skills

A skill supplies task-specific knowledge or a reliable operation the model would otherwise have to rediscover. Optimize for correct outcomes, useful evidence, and low overhead. Identical process on every run is useful only where ordering is part of the contract.

## Choose the Scope

- Follow the user's requested outcome and existing authorization. A skill does not add approval steps or override repository policy.
- Keep a rule only when it changes a relevant decision or prevents an evidenced failure. Delete generic advice, duplicated rules, obsolete model assumptions, and rhetorical pressure.
- Name concrete triggers and exclusions in the description. Avoid firing on every code edit, every test, or every completion when the procedure is only needed for a narrower problem.
- Distinguish shared guidance from repository-specific policy. Edit shared sources first; preserve local variants when synchronizing. Check whole-directory differences before an installer overwrites a skill.

## Create or Revise

1. Identify the outcome, owning files, known failure, and existing instructions. Reuse the current conversation's evidence; recollect only missing or stale material.
2. Keep the minimum common contract in `SKILL.md`; link branch-specific references with a condition and the section needed. Read a complete schema or tightly coupled procedure when its whole contract is necessary, not every reference by default.
3. State required order only for dependencies, mutations, authorization, or cleanup. Let independent searches batch and let the agent choose routine implementation details.
4. Define completion by the requested deliverable and its applicable checks. Reuse evidence for unchanged inputs. Do not add a mandatory report, subagent round, test suite, or confirmation merely because the skill is installed.
5. Check changed triggers, links, metadata, and any affected helper behavior. For a small prose edit, summarize this review; for a new skill or broad rewrite, use `references/checklist.md`. Fix material failures at their source.

For a new package, read `references/authoring-procedure.md`; `assets/SKILL.template.md` is an optional skeleton. Resolve bundled helpers from the skill's actual directory. The read-only metadata helper is `python3 <writing-skills-dir>/scripts/validate-metadata.py --name "[name]" --description "[description]"`.

## Diagnose and Measure

If a necessary reference was missed, use `references/loading-diagnosis.md` to distinguish a bad trigger, inaccessible path, conflicting rule, stale content, and model error. Strengthen the relevant pointer or fix the helper; extra emphasis is not the default repair.

Evaluate behavior-changing guidance on representative tasks under the same model/harness when practical: correctness, rework, unnecessary questions, tool calls, and elapsed time. Include successful runs as controls. Count reference reads and skill invocations as costs or diagnostics, never success by themselves. State when only static review was performed; do not claim measured improvements without comparative runs.

Terminology, when needed: `references/glossary.md`.
