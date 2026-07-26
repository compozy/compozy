# Goal Header Template

`cy-loop-tasks` is invoked manually. The most common invocation pattern is
to paste the codex-loop activation header at the top of the prompt plus an
explicit reference to the skill. The plugin itself is **not** modified;
nothing in `~/.codex/codex-loop/config.toml` needs to change.

## Canonical header (copy-paste)

For a feature with slug `<slug>` whose techspec lives at
`.compozy/tasks/<slug>/_techspec.md`:

```text
[[CODEX_LOOP name="<slug>" goal="ship <slug> end-to-end via cy-loop-tasks: every iteration runs .agents/skills/cy-loop-tasks/scripts/detect-phase.py and executes the printed action; every Phase B checkpoint runs scoped validation then cy-final-verify before commit; continue until qa-report and qa-execution are complete, consecutive deep-review rounds reach SHIP, and make verify is PASS"]]

Use the cy-loop-tasks skill at .agents/skills/cy-loop-tasks/SKILL.md.
The skill is a self-healing continue loop — repair command and gate failures inside the current phase action, then continue until Phase E or a proven external blocker. Slug: <slug>.
```

The `goal=` text becomes `state.yaml.goal_signature` and is shown to the
goal-check confirmation prompt as the success criterion. Keep it specific
(mentions slug, the QA gate, and the peer-review SHIP gate) so the verdict
is grounded.

## Frontend lane (`--frontend <claude|cursor>`)

To orchestrate frontend tasks through herdr worker TUIs, append the
parameter to the invocation line:

```text
Use the cy-loop-tasks skill at .agents/skills/cy-loop-tasks/SKILL.md.
The skill is a self-healing continue loop — repair command and gate failures inside the current phase action, then continue until Phase E or a proven external blocker. Slug: <slug>. --frontend claude
```

- `--frontend claude` → Claude Code workers (`claude
  --dangerously-skip-permissions --model opus --effort xhigh`)
- `--frontend cursor` → Cursor workers (`cursor-agent --yolo --model
  grok-4.5`)

Bootstrap passes the value to `init-state.py --frontend`; it lands in
`state.yaml.frontend_agent` and holds for the whole loop. Omit the parameter
to run every task locally.

## Invoking without the plugin (manual run)

```
Activate the cy-loop-tasks skill at .agents/skills/cy-loop-tasks/SKILL.md
for slug <slug>. Repair in-scope failures autonomously and continue until Phase E or a proven external blocker.
```

The agent runs detect → one action → summary, then continues at detect in
the same session. Filesystem state still resumes if the session ends early.

## When NOT to use the goal header

Do not paste the `[[CODEX_LOOP ...]]` header for the bootstrap iteration if
`_techspec.md` does not exist yet — `cy-loop-tasks` will refuse and record a
blocker. Author the techspec first via `cy-create-techspec`, then start the
loop.
