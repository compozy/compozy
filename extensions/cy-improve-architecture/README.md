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
