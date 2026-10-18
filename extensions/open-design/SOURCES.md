# Sources and local ownership

This is an independently maintained CompozyOS extension. Source revisions below record where
selected material came from; they are not runtime, build, or update dependencies. Edit this
extension directly. There is no upstream mirror, download, catalog, or synchronization process.
License texts and retained notices are in [LICENSES.md](LICENSES.md).

## OpenDesign

Source: [nexu-io/open-design, revision 933dc96038a4ee7a30c56d479f3497ad2716cbb3](https://github.com/nexu-io/open-design/tree/933dc96038a4ee7a30c56d479f3497ad2716cbb3).
Copyright 2026 Open Design contributors; Apache-2.0.

| Local material                                                             | Selected source                                                                                                                                                                                                                                 | Local changes                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| -------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Designer, critic, and design/review skills                                 | `apps/daemon/src/prompts/core-slim.ts`, `plugins/_official/atoms/critique-theater/SKILL.md`, and the craft/template guidance below                                                                                                              | Rewritten for native sessions, workspace HTML, project authority, and explicit bounded review. No OpenDesign artifact protocol, injected catalogs, mandatory brief form, or runtime integration.                                                                                                                                                                                                                                                                   |
| `skills/open-design/references/craft.md`                                   | `craft/typography.md`, `typography-hierarchy.md`, `typography-hierarchy-editorial.md`, `color.md`, `anti-ai-slop.md`, `accessibility-baseline.md`, `state-coverage.md`, `form-validation.md`, `rtl-and-bidi.md`, `animation-discipline.md`      | Condensed into one reference. Retains concrete craft guidance; distinguishes accessibility requirements from aesthetic heuristics. Omits host protocols, dated claims, and prescribed stacks.                                                                                                                                                                                                                                                                      |
| `skills/open-design/references/craft.md`, “Choice, memory, and convention” | `craft/laws-of-ux.md` at revision 78d5637cb49da07e81314d1544b5fc6d37c95084 (unchanged since the original revision)                                                                                                                              | Condensed composition heuristics on grouping, decision load, chunking, primary actions, conventions, and honest progress. Omits citations and folklore commentary; defers target floors to the existing accessibility guidance.                                                                                                                                                                                                                                    |
| `skills/open-design/references/motion.md`                                  | `skills/animate/SKILL.md` (Emil Kowalski, `emilkowalski/skills` revision e8a175de22ae1e49370fc144c1f3bb9aeedf988d, MIT) as carried by OpenDesign revision 78d5637cb49da07e81314d1544b5fc6d37c95084; `craft/animation-discipline.md`             | Rewritten as adaptable guidance for workspace HTML: frequency/purpose gate, tool choice, properties, easing and duration starting points, springs, interruption, and reduced motion. Omits recipes, framework-specific code, companion-skill routing, and host execution context. Project motion tokens take precedence.                                                                                                                                           |
| `skills/open-design/references/artifacts.md`                               | `design-templates/dashboard/SKILL.md`; `web-prototype/` and `simple-deck/` skills, layouts, and checklists; `simple-deck/assets/template.html`; `docs-page/SKILL.md` and `example.html`; `html-ppt-taste-editorial/SKILL.md` and `example.html` | Selected composition and behavior principles for four formats, written as adaptable guidance. No examples, assets, fonts, fixed palettes, or templates are distributed.                                                                                                                                                                                                                                                                                            |
| `linter/index.ts`, `rules.ts`, `css.ts`                                    | `apps/daemon/src/lint-artifact.ts`                                                                                                                                                                                                              | Split into local modules; local finding type replaces daemon contracts; unused variable removed. Original rule IDs, severity, messages, fixes, and snippets are retained. Local correctness fixes honor specificity within the supported global-theme selectors, scan every style block for raw colors, and detect slide classes independently of attribute order. Feedback asks the agent to update workspace HTML instead of re-emitting an OpenDesign artifact. |
| `linter/__tests__/index.test.ts`                                           | `apps/daemon/tests/lint-artifact.test.ts`                                                                                                                                                                                                       | All 97 original cases retained, plus local regressions for CSS specificity, multiple style blocks, and section attribute order. Imports use Bun's test API and the shipped local bundle; repository formatting and test placement applied.                                                                                                                                                                                                                         |

The motion reference adapts Emil Kowalski's MIT-licensed [animate skill](https://github.com/emilkowalski/skills/tree/e8a175de22ae1e49370fc144c1f3bb9aeedf988d/skills/animate),
© 2026 Emil Kowalski, as distributed in OpenDesign revision 78d5637cb49da07e81314d1544b5fc6d37c95084. Its notice is retained in LICENSES.md.

The craft sources credit the MIT-licensed [Refero Design refero_skill](https://github.com/referodesign/refero_skill),
© Refero Design. Its notice is retained in LICENSES.md. The editorial deck source credits
[Leonxlnx/taste-skill](https://github.com/Leonxlnx/taste-skill), `skills/minimalist-skill/SKILL.md`;
that attribution is retained here. This extension includes no code or assets from that repository.

The condensed accessibility guidance also checks text contrast, target sizes, and automatic
motion against the W3C explanations for [contrast minimum](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html),
[target size minimum](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html), and
[pause, stop, hide](https://www.w3.org/WAI/WCAG22/Understanding/pause-stop-hide.html).

## Agent-browser

Source: [vercel-labs/agent-browser, revision 8c15ff9f71ae60c7e99e66afe1e2d4b9bf414fe2](https://github.com/vercel-labs/agent-browser/tree/8c15ff9f71ae60c7e99e66afe1e2d4b9bf414fe2),
`skills/agent-browser/SKILL.md`, and its official CLI guidance. Apache-2.0.

`skills/open-design-browser/SKILL.md` is a small, locally maintained adaptation for opening workspace
HTML, interacting with it, capturing screenshots, and closing an isolated browser session.
It requires an existing CLI/browser and actual image inspection before a visual claim. It does
not install dependencies, fetch skill text, or depend on OpenDesign's browser service.

## Generated files

`scripts/build-lint.ts` bundles only the local linter modules into `lint.gen.mjs` for the Go
provider. `scripts/generate-manifest.go` derives `extension.json` from the native SDK definition.
Neither generator accesses upstream. Prompts, skills, and references are maintained Markdown.
