# cy-improve-architecture

`cy-improve-architecture` audits a module or area for shallow interfaces, recommends one high-leverage deepening first, and leaves durable architecture guidance for later agent sessions. The extension is a self-contained pack containing the audit plus the `cy-codebase-design` and `cy-domain-modeling` skills.

## Install

Replace `<tag>` with the Compozy release you want to install:

```bash
compozy ext install --yes compozy/compozy --remote github --ref <tag> --subdir extensions/cy-improve-architecture
compozy ext enable cy-improve-architecture
compozy setup
```

Re-running `compozy setup` is safe and refreshes the three installed skills without creating duplicate skill directories.

## Keep the depth map in agent memory

The audit writes its terse, always-on guidance to `.compozy/ARCHITECTURE.md` and detailed report bodies to `.compozy/arch-reviews/`. Wire the depth map into each agent instruction file you use; the extension does not edit these files for you.

For Claude Code, add this line to `CLAUDE.md`:

```markdown
@.compozy/ARCHITECTURE.md
```

For Codex and other agents that read repository guidance, add the same line to `AGENTS.md`:

```markdown
@.compozy/ARCHITECTURE.md
```

The depth map is advisory. Re-run the audit for an area when its guidance becomes stale; the area's report and map section are refreshed while other areas remain intact.

## Commit the durable artifacts

If your repository already tracks `.compozy/`, no gitignore change is needed. If it ignores `.compozy/**`, add these negations after the ignore rule so the depth map and report history remain reviewable:

```gitignore
!.compozy/
!.compozy/ARCHITECTURE.md
!.compozy/GLOSSARY.md
!.compozy/arch-reviews/
!.compozy/arch-reviews/**
```

Commit `.compozy/ARCHITECTURE.md`, `.compozy/GLOSSARY.md`, and `.compozy/arch-reviews/` with the code they describe. The markdown reports are the offline-safe source of truth; the HTML reports are their visual twins. The extension never edits `.gitignore`, `CLAUDE.md`, or `AGENTS.md` silently.

## Optional settled-decision awareness

We recommend `cy-capture-decisions` as an optional companion. Its `.compozy/DECISIONS.md` index lets the audit recognize architecture decisions that are already settled and shipped, so it can avoid reopening them. The companion is not required: without it, the audit still produces the complete reports and depth map.

The two memory surfaces have distinct ownership. `cy-capture-decisions` remains the sole writer of durable shipped decisions, while rejected deepening proposals stay in `.compozy/ARCHITECTURE.md` with their load-bearing reasons.

## Credits

This extension is a port of [Matt Pocock](https://github.com/mattpocock)'s open-source skills to the Compozy workflow, used under the MIT License, and extended with Compozy-native additions (see the closing paragraph). See [`NOTICE`](NOTICE) for the upstream copyright and license text. Upstream source: <https://github.com/mattpocock/skills>.

- [`improve-codebase-architecture`](https://github.com/mattpocock/skills/tree/main/skills/engineering/improve-codebase-architecture) — the deep/shallow-module audit, deletion test, and HTML report.
- [`codebase-design`](https://github.com/mattpocock/skills/tree/main/skills/engineering/codebase-design) — the deep-module vocabulary and design-it-twice guidance.
- [`domain-modeling`](https://github.com/mattpocock/skills/tree/main/skills/engineering/domain-modeling) — glossary upkeep.

The optional interactive grilling step can use Matt Pocock's [`grill-me`](https://github.com/mattpocock/skills/tree/main/skills/productivity/grill-me) skill when it is installed; a built-in interrogation runs without it.

The Compozy adaptation adds the durable, `@import`-able `.compozy/ARCHITECTURE.md` depth map and its Markdown report twin, a test-only grammar validator (`archmap`), packaging/install tests, and a model-backed evaluation harness.
