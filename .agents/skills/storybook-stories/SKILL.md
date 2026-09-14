---
name: storybook-stories
description: "Create, update, refactor, or troubleshoot Storybook stories using project conventions. Excludes component implementation, token changes, E2E browser tests, and other documentation."
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Storybook Stories

Extend the nearest existing story using the Storybook instance and naming conventions for its layer. Locate the owning Storybook configuration for primitives versus app/domain stories. Reuse global QueryClient/router/MSW decorators instead of duplicating providers.

- Inspect the shared primitive inventory before composing examples. Use its public exports; domain components remain local. Follow the repository's token source, design documentation, and surface instructions.
- Use typed CSF metadata and `StoryObj<typeof meta>`. Choose layout/decorators for the component's real surface; centered layout is useful for some controls, not full routes.
- Show the smallest set of supported states that explains meaningful behavior. No minimum or maximum story count, exhaustive prop matrix, or per-export JSDoc requirement.
- Use `args` for controlled props and `render` for compound composition. A render-only story does not need empty `args`. Add descriptions when they explain behavior, constraints, or usage that the canvas cannot show.
- Keep examples interactive, accessible, and realistic. Use the actual component API, not assumed variants copied from a template.
- Follow the project's existing autodocs policy. Enable or omit `tags: ["autodocs"]` consistently with that owner; avoid duplicate pages.
- Run the owning Storybook/build or visual check for the changed behavior. Reuse existing evidence; stories do not automatically require new unit or end-to-end suites.

Read `references/patterns.md` only for the story composition pattern being used. Examples are adaptable, not a required scaffold.
