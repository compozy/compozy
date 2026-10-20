---
name: open-design-setup
description: Prepare or repair a project's prototyping environment so open-design can build prototypes with the project's own component library and render them through the project's toolchain. Use when the library target is needed and docs/design/prototyping.md is missing, stale, or failing; excludes HTML-only design work and production feature implementation.
---

# Set up library prototyping

Library prototypes are ordinary source files that import the project's real components and render through the project's own toolchain. This skill prepares that environment once per project and records it in `docs/design/prototyping.md`. Later sessions read the record instead of repeating the setup. Work like an engineer joining the repository: inspect what exists, reuse it, and add the smallest missing piece.

## Inspect the project

Read the project's agent instructions, `DESIGN.md` or equivalent design authority, and package manifests before choosing anything. Establish:

- **Library:** the import path of the components a prototype should use, for example a workspace package, a local `components/ui` directory, or an npm design system. Note shadcn-style `components.json` aliases and registries; they also tell you how missing components are added.
- **Catalog:** where the agent learns components, props, and examples: component source and types, a registry, a Storybook or other component manifest, documentation, examples, or a project skill describing the library.
- **Toolchain:** package manager, bundler and framework, TypeScript configuration and path aliases, the global stylesheet or token entry, and the providers the library needs (theme, router, data, i18n).
- **Existing preview hosts:** Storybook, Ladle, React Cosmos, a playground route, a sandbox app, or an examples site. A host the project already maintains is the first choice.
- **Constraints:** ignored directories, lint rules such as unused-file checks, test runners or visual baselines that would collect new files, and rules about adding dependencies.

If the project has no component library, say so and use the HTML target. Do not invent one.

## Choose the host

Prefer the least new machinery, in this order:

1. **Reuse an existing preview host.** Place prototypes where that host already discovers files, under a clearly named prototype group, and reuse its decorators, providers, and mocks.
2. **Add a playground to an existing app or package** that already compiles the library, such as a development-only route or entry.
3. **Create a minimal prototype app** in the repository, using the same framework, bundler, styles, and versions the project already uses.

[references/recipes.md](references/recipes.md) describes each host shape, rendered-evidence export, and common pitfalls. Read only the section you need through `compozy__skill_view` with `name: open-design-setup` and `file: references/recipes.md`.

Prototypes must not call real product APIs. Use the project's existing mock layer, or typed local fixtures built from the project's generated API types when they exist.

## Get approval before changing the repository

Setup changes the user's project. Before adding dependencies, packages, scripts, routes, or configuration, present the plan: chosen host, files to add or edit, dependencies with versions, and exclusions from tests, lint, or production builds. Wait for the user's approval. A request that already explicitly authorizes the setup counts as approval. Use the project's package manager and conventions, and keep prototype-only code out of production bundles.

## Prove it works

Before recording anything:

1. Start the development command. Reuse a server that already serves the expected URL.
2. Add a small smoke prototype that renders several real library components and uses the global styles and tokens.
3. Run the recorded typecheck command against it and fix errors.
4. Open the preview URL with `open-design-browser` and load a screenshot. Confirm that the components, tokens, and fonts render, not unstyled markup.
5. Produce the rendered-evidence HTML described in the recipes, and confirm `ext__open_design__lint_artifact` accepts it.

Remove the smoke prototype unless the user wants to keep it. Stop any server this setup started. If a step cannot pass, report the blocker and do not write a record that claims it works.

## Record the environment

Write `docs/design/prototyping.md` with YAML front matter that later agents read, followed by short notes on decisions and pitfalls:

```markdown
---
target: library # default target for new requests: library or html
library: ["@acme/ui"] # import paths prototypes may use for UI
catalog: # where components, props, and examples are documented
  - packages/ui/src/components
  - packages/ui/components.json registries
authority: [DESIGN.md]
host: vite-app # existing host or the shape created
prototypes: apps/prototypes/src/designs # prototype sources, one folder per slug
entry: "{slug}/index.tsx" # entry file relative to prototypes
providers: apps/prototypes/src/providers.tsx
dev: bun run --cwd apps/prototypes dev
preview: "http://localhost:4400/{slug}"
typecheck: bun run --cwd apps/prototypes typecheck
evidence: snapshot # export command, or snapshot when the DOM is captured
verified: 2026-10-20
---

Notes on decisions, exclusions, and pitfalls.
```

Use workspace-relative paths and commands runnable from the workspace root. If `docs/design/` is ignored by the project's version control, tell the user the record stays local. Record only what you verified. Update the record when a later session repairs the environment.

## Repair

When the designer reports a failing record, rerun only the failing checks, fix the environment within its existing shape, and update `verified`. Changing the host shape is a new setup and needs approval.
