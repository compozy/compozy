---
name: open-design
description: Create or refine interfaces, sites, HTML slides, and visual documents from a short brief, existing HTML, or _uiux.md. Use for directly openable workspace artifacts under docs/design; excludes production application implementation and automatic independent review.
---

# Open design

Create the smallest complete artifact that answers the request. A short brief and a `_uiux.md` are equally valid inputs. Use the user's language unless the requested content specifies another. Ask only when an unresolved decision prevents useful work; otherwise state a reasonable assumption and proceed.

## Bind the design to its project

Read the supplied context and the relevant existing UI, components, tokens, assets, and approved references. Current project authority governs visual choices; do not impose Compozy styling on another product or let a generic aesthetic rule replace the approved system. If no direction exists, choose one coherent approach suited to the audience and content.

For `_uiux.md`, preserve requested surfaces, states, artifact paths, component mappings, and relevant behavior from linked spec/story sections. Respect superseded entries and group related states where useful. Treat illustrative references as visual guidance, not permission to invent features. Keep unresolved product decisions visibly provisional.

Load only the reference sections needed for this artifact, relative to this skill's directory:

- [craft.md](references/craft.md): hierarchy, composition, content, and color for a new direction; semantics, focus, and states for interactive work; language and motion when affected.
- [artifacts.md](references/artifacts.md): the matching lane for an interface, site, HTML slide deck, or visual document, including its relevant delivery checks.

Use `compozy__skill_view` with `name: open-design` and `file: references/craft.md` or `references/artifacts.md` when available. If the harness has no native skill-reading tool, read from the installed skill directory; never bypass an explicit denial. Reuse already-read guidance while it remains relevant.

## Author a directly openable artifact

Write `docs/design/<slug>/index.html` in the active workspace by default. Explicit requested paths under `docs/design/` take precedence. Keep supporting assets there with ordinary relative links, or inline them. The output must open from a local file without a build, service, custom viewer, installed-extension paths, or remote runtime dependencies.

Use complete, brief-derived content and accurate assets. Label illustrative data; never invent sources, endorsements, metrics, or backend guarantees. A named real person, product, brand, artwork, or place needs its correct asset, not a fabricated look-alike. Reuse approved local assets first; disclose a necessary labeled placeholder when the correct asset is unavailable. Preserve image proportions and required attribution.

Give meaningful regions and controls stable `data-od-id` values for precise feedback. Implement local demonstrations of the requested interactions without calling product mutation APIs. Keep prototype or presenter controls out of the artifact unless needed by its format or requested by the user.

Refine the same files in place, preserving approved decisions and unrelated content. Do not create a version system, extra registry, or mandatory companion document. This workflow creates design artifacts; it does not edit production application code or publish them.

## Verify and deliver

Read back the changed HTML, check local assets and content completeness, and exercise the primary interaction when tooling permits. Invoke `ext__open_design__lint_artifact` with the actual workspace-relative HTML paths. Inspect its findings, fix applicable defects, and explain an intentional exception with the specific user or project authority. Report unavailable lint tooling as unverified.

When rendered inspection would resolve a visual or interaction risk, use the bundled `agent-browser` skill with an isolated task-owned CLI session. Do not substitute `browser-use`, attach to an existing browser, or operate the user's live browsing session. Load the resulting screenshot through the harness's image-reading capability before claiming visual inspection; an accessibility snapshot or screenshot path is not visual proof. Keep checks proportional to the change. Missing browser support need not block an HTML sketch, and does not justify an automatic installation.

Link the actual HTML files and briefly describe the result, checks performed, and material limitations. Do not repeat their source in chat. Recheck affected areas after corrections and reuse valid evidence for unchanged areas.

## Explicit independent review

Load `open-design-review` only when the user requests the independent review and refinement cycle. Routine creation, edits, or `_uiux.md` input alone stay in the designer conversation. When acting inside that loop, follow its supplied schema without starting another run.
