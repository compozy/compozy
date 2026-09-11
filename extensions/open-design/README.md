# Open design

A built-in CompozyOS extension for designing interfaces, sites, HTML presentations, and visual
documents. It ports OpenDesign's design intelligence into ordinary CompozyOS sessions and Loops.
It does not run the OpenDesign application, daemon, project database, or MCP server.

## Use it

Select the `open-design` profile and start a session with `open-design-designer`. Use a short
request, an existing HTML, or a `_uiux.md` with its companion context:

> Design a small HTML reference for session bulk actions in docs/design/session-bulk-actions/index.html.
> Include selecting several sessions, the action bar, and deletion confirmation. Keep our current UI styling.

The designer reads the workspace and project design authority and the curated craft and artifact references,
then writes ordinary HTML under `docs/design/`. Explicit output paths within that directory win;
otherwise it uses `docs/design/<slug>/index.html`. Related states can share one file. The user
opens the HTML directly and requests changes in the same session. Drawing a feature does not
authorize changing production application code.

For a complete independent review, ask the designer to use `open-design-review`. Its native Loop
runs designer → original lint → independent critic → digest verification. It completes only
after the critic approves the exact checked files; rejection starts another full pass. The skill
supplies a per-run limit of three passes, allowing at most two refinements.
Compozy runtime defaults can override values declared by a Loop; when starting it directly,
supply `iteration_cap: 3` and `reattempt_strategy: full_body` through
`config_overrides` (or the CLI's `--config-file`). The critic covers craft, purpose/states, brand,
accessibility, and copy. It must resolve
applicable findings and cite the source for intentional exceptions. An exhausted or failed run
retains its actual outcome and latest files; it does not restore an earlier file version.

## Resources and prerequisites

| Resource                                                          | Purpose                                                        |
| ----------------------------------------------------------------- | -------------------------------------------------------------- |
| Profile `open-design`                                             | Default designer and extension resource placement              |
| Agents `open-design-designer`, `open-design-critic`               | Authoring and independent review                               |
| Skills `open-design`, `open-design-review`, `open-design-browser` | Curated guidance, explicit review workflow, browser inspection |
| Loop `open-design-review`                                         | Bounded, opt-in refinement                                     |
| Tool `ext__open_design__lint_artifact`                            | Read-only original design lint on workspace HTML               |

Use an authenticated agent provider configured in CompozyOS. This extension does not choose a
provider/model or create credentials. The linter requires Node.js on the CompozyOS process PATH.
The bundled JavaScript needs no npm packages or OpenDesign service. Bun is a development/codegen
dependency only.

Visual inspection uses the `open-design-browser` skill, a compact local adaptation of the official
`agent-browser` guidance
and a separately installed CLI/browser. Opening a local file may require
`agent-browser --allow-file-access open file:///absolute/path/to/index.html`. The agent captures
a screenshot and loads that image through its harness's image-reading capability. Text snapshots
are not screenshots. If the CLI, browser, or image support is unavailable, HTML authoring still
works and the agent reports which checks it actually completed. No custom preview service or
mandatory screenshot gate is added.

## Lint contract

Input is `{ "paths": ["docs/design/example/index.html"] }`. The provider obtains the workspace
root from authenticated invocation context; callers cannot supply an arbitrary root. Paths must
be normalized, workspace-relative HTML names under `docs/design/`. Symlinks in the artifact path,
directories, special files, empty/non-UTF-8 content, and paths outside that directory are rejected.
The trusted workspace root itself may resolve through a normal system alias.

Limits: 32 files, 1 MiB per HTML, 8 MiB combined HTML, 1 MiB output, and 30 seconds per Node call.
Node runs the embedded program directly, receives HTML over stdin, and never evaluates artifact
scripts. Preload environment variables are removed. Missing Node, timeout, or invalid output is
an error, not a clean lint.

Each artifact result includes `path`, SHA-256 of the read bytes, original `findings`, P0/P1/P2
`counts`, adapted `feedback`, and `passed`. The aggregate `passed` is true exactly when every
artifact has zero original P0 findings. This heuristic result does not prove visual quality,
accessibility, functional correctness, or that all P1/P2 findings were resolved. The independent
critic uses the brief and project design system to decide applicability without hiding the
original findings. The tool never writes files.

## Maintained sources

The extension owns a small set of prompts, three skills, two design references, and the design
linter. The references distill typography, hierarchy, color, state coverage, accessibility,
forms, motion, and composition for the four supported formats. The designer and critic bind that
guidance to ordinary workspace files and native CompozyOS tools.

Selected material comes from OpenDesign and agent-browser; [SOURCES.md](SOURCES.md) records the
source revisions, selections, and local changes, with license texts in [LICENSES.md](LICENSES.md).
These revisions document attribution, not an update dependency. There is no mirrored repository,
remote knowledge loader, template catalog, asset library, or synchronization step. Maintain the
local files directly.

The TypeScript linter retains OpenDesign's rule IDs, severities, messages, fixes, snippets, and
CSS token/theme handling. Only its host-specific feedback instruction changes to update the
workspace HTML. Three local modules replace the original daemon dependency. `lint.gen.mjs` is
built from those modules for the Go provider; generation does not contact upstream.

## Develop and verify

```sh
# Regenerate the local lint bundle and native manifest:
go generate ./extensions/open-design

# Native boundaries, resource parsing/compiler, and original lint assertions:
CGO_ENABLED=1 go test -race ./extensions/open-design
```

The linter suite retains all 97 original test cases and runs them directly against the shipped
bundle using Bun's test API. Runtime QA also exercises the public tool, managed designer,
browser, and optional review Loop.

CompozyOS installs this kit on boot through its existing bundled-extension lifecycle. Operator
disable/enable choices and non-bundled installations follow that lifecycle; normal extension
status, inventory, profile, and logs surfaces remain authoritative.
