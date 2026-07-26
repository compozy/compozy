# Storybook URL grammar (isolation mode)

## Servers AGH runs

| Workspace | Port | Reason |
| --- | --- | --- |
| `web/` | 6006 | Runtime operator UI stories (`systems-*-routes-*`, `systems-*-components-*`) |
| `packages/ui/` | 6007 | Shared primitive stories (components-*, components-custom-*, logos-*) |

Start with `cd web && bun run storybook` or `cd packages/ui && bun run storybook`. Both default to those ports; if a port is busy the dev command prompts to bump — kill the duplicate before reusing.

## Iframe form (the only form that should be screenshot)

```
http://localhost:<port>/iframe.html?id=<story-id>&viewMode=story
```

- `iframe.html` strips Storybook chrome (sidebar, addons panel, manager toolbar).
- `viewMode=story` forces the iframe to render the story preview, not the docs / autodocs view.
- Do NOT screenshot the manager URL (`http://localhost:6006/?path=/story/<id>`) — that frames the story in Storybook chrome at unpredictable widths.

## Discovering story ids

Story ids live in `index.json` exposed by each running Storybook:

```
http://localhost:6006/index.json
http://localhost:6007/index.json
```

`scripts/list-stories.mjs` parses this index and emits one id per line.

## Naming patterns observed in AGH

| Pattern | Source | Example |
| --- | --- | --- |
| `systems-<system>-routes-<route>--<state>` | `web/src/systems/<system>/routes/<route>.stories.tsx` | `systems-tasks-routes-tasks--default-list` |
| `systems-settings-routes-<group>--<state>` | `web/src/systems/settings/routes/<group>.stories.tsx` | `systems-settings-routes-settingsgeneral--default` |
| `systems-runtime-components-appsidebar--<state>` | `web/src/systems/runtime/components/stories/app-sidebar.stories.tsx` | `systems-runtime-components-appsidebar--categorized` |
| `systems-runtime-components-topbarshell--<state>` | `web/src/systems/runtime/components/stories/topbar-shell.stories.tsx` | `systems-runtime-components-topbarshell--default` |
| `systems-design-system-components-designsystemshowcase--default` | `web/src/systems/design-system/components/stories/design-system-showcase.stories.tsx` | one story only |
| `components-<primitive>--<state>` | `packages/ui/src/components/stories/<primitive>.stories.tsx` | `components-button--default` |
| `components-custom-<primitive>--<state>` | `packages/ui/src/components/custom/stories/<primitive>.stories.tsx` | `components-custom-actionresultbanner--success` |

## Verifying a story id exists before capturing

```bash
bun run scripts/list-stories.mjs http://localhost:6006 --filter systems-tasks-routes
```

If a story id you intend to capture is not in that output, the capture will land on Storybook's "Couldn't find story" fallback frame — visible as a 5–10 KB PNG instead of 100–200 KB. Treat suspiciously small PNGs as a story-id mismatch.

## URL examples that worked in design audits

```
http://localhost:6006/iframe.html?id=systems-tasks-routes-tasks--default-list&viewMode=story
http://localhost:6006/iframe.html?id=systems-tasks-routes-taskdetail--overview&viewMode=story
http://localhost:6006/iframe.html?id=systems-tasks-routes-taskcreate--default&viewMode=story
http://localhost:6006/iframe.html?id=systems-design-system-components-designsystemshowcase--default&viewMode=story
http://localhost:6007/iframe.html?id=components-button--default&viewMode=story
```
