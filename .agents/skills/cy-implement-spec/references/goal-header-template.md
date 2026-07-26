# Goal Header Template

`cy-implement-spec` is invoked manually. The most common invocation pattern
is to paste the codex-loop activation header at the top of the prompt plus
an explicit reference to the skill. The plugin itself is **not** modified;
nothing in `~/.codex/codex-loop/config.toml` needs to change.

## Canonical header (copy-paste)

For a feature with slug `<slug>` whose techspec lives at
`.compozy/tasks/<slug>/_techspec.md`:

```text
[[CODEX_LOOP name="<slug>" goal="ship <slug> end-to-end via cy-implement-spec, straight from the spec documents with no task decomposition: every iteration runs .agents/skills/cy-implement-spec/scripts/detect-phase.py and executes the printed action; every Phase B milestone runs scoped validation then cy-final-verify before its checkpoint commit; continue until every spec criterion is met, qa-report and qa-execution are complete, consecutive deep-review rounds reach SHIP, and the verification gate is PASS"]]

Use the cy-implement-spec skill at .agents/skills/cy-implement-spec/SKILL.md.
The skill is a self-healing continue loop — repair command and gate failures inside the current phase action, then continue until Phase E or a proven external blocker. Slug: <slug>.
```

The `goal=` text becomes `state.yaml.goal_signature` and is shown to the
goal-check confirmation prompt as the success criterion. Keep it specific
(mentions slug, the criteria gate, the QA gate, and the peer-review SHIP
gate) so the verdict is grounded.

## Invoking without the plugin (manual run)

```
Activate the cy-implement-spec skill at .agents/skills/cy-implement-spec/SKILL.md
for slug <slug>. Repair in-scope failures autonomously and continue until Phase E or a proven external blocker.
```

The agent runs detect → one action → summary, then continues at detect in
the same session. Filesystem state still resumes if the session ends early.

## When NOT to use the goal header

- `_techspec.md` does not exist yet — `cy-implement-spec` will refuse and
  record a blocker. Author the techspec first via `cy-create-techspec`,
  then start the loop.
- The slug already carries `_tasks.md` / `task_*.md` — `init-state.py`
  refuses (exit 5). Drive an authored task graph with `cy-loop-tasks`.
