# CH-skill-sources-repo-teammate: Clone the repo, get the team's skills, leave the repo untouched

```yaml
charter:
  id: CH-skill-sources-repo-teammate
  mission: "As Bruno opening a teammate's repository for the first time with untouched personal settings, prove the committed skills and committed source configuration just work in that workspace, stay out of every other workspace, and that nothing personal is written back into the working tree."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-absorb-skills-from-other-tools
  scenarios: [ET-workspace-skill-source-teammate]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Clone a repository that commits both .agents/skills/review-checklist/SKILL.md and a [skills] block in <ws>/.compozy/config.toml, as an operator whose own configuration is untouched defaults. Open the workspace and confirm the skill is available in its sessions with no local setup step, and that a second workspace does not see it."
      - "Walk the workspace-relative rule from both sides: a relative custom source in the workspace config loads, while the same relative path submitted at user scope is refused with invalid_source_path explaining that workspace-relative paths require workspace scope."
      - "Confirm per-key presence semantics against the committed file — a key the repository sets replaces the personal list for that workspace, a key it omits inherits, and an empty list turns that key's roots off for this workspace even when the personal configuration enables more."
      - "Run git status in the clone at the end. The tracked tree must be byte-identical: no personal configuration, no cache, no ownership record written into the working copy. This is the evidence, not an inspection of the config file."
      - "Check out a branch that deletes the committed skill directory and confirm it leaves the catalog on the next refresh with no restart and no error to dismiss — and that a same-named global skill it was shadowing becomes the winner again."
    must_avoid:
      - "Running as the operator who authored the repository. The whole claim is that a teammate with default personal settings gets the same result; testing as the author proves nothing."
      - "Editing anything inside the clone to make a step work. A step that needs a local edit is the finding."
```

## Selection rationale

A short scenario-based session, because US-004's promise is a story rather than a matrix: someone
clones a repository and the team's skills are simply there. The value is only real when walked by
someone whose own machine was never configured for it, which is why the persona and the "do not run
as the author" constraint carry more weight here than the tour choice does.

It is separated from the other absorb-journey charters for one reason: personas. `CH-skill-sources-live-apply`
and `CH-skill-sources-settings-web` are Dora sessions about administering policy on a machine she
owns; this one is Bruno arriving at someone else's repository, and folding it into a Dora session
would quietly drop the only condition that makes it meaningful. Thirty minutes is enough because the
surface is narrow and the assertions are concrete.

The `git status` check is deliberately the terminal evidence rather than a nicety. ADR-006's
workspace override and ADR-007's replace-on-present merge both put configuration inside the
repository, and the failure mode that would hurt a team most is not a skill failing to load — it is
Compozy writing something personal into a shared working tree and every teammate seeing a dirty diff
they did not create.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
