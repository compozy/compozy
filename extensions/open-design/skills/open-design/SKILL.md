---
name: open-design
description: Create or refine interfaces, sites, HTML slides, and visual documents from a short brief, existing design, or _uiux.md, as directly openable HTML or as prototypes built with the project's own component library. Use for design artifacts and prototypes; excludes production application implementation and automatic independent review.
---

# Open design

Create the smallest complete artifact that answers the request. A short brief and a `_uiux.md` are equally valid inputs. Use the user's language unless the requested content specifies another. Ask only when an unresolved decision prevents useful work; otherwise state a reasonable assumption and proceed.

## Bind the design to its project

Read the supplied context and the relevant existing UI, components, tokens, assets, and approved references. Current project authority governs visual choices; do not impose CompozyOS styling on another product or let a generic aesthetic rule replace the approved system. If no direction exists, choose one coherent approach suited to the audience and content.

For `_uiux.md`, preserve requested surfaces, states, artifact paths, component mappings, and relevant behavior from linked spec/story sections. Respect superseded entries and group related states where useful. Treat illustrative references as visual guidance, not permission to invent features. Keep unresolved product decisions visibly provisional.

Load only the reference sections needed for this artifact, relative to this skill's directory:

- [craft.md](references/craft.md): hierarchy, composition, choice and decision load, content, and color for a new direction; semantics, focus, and states for interactive work; language and the reduced-motion floor when affected.
- [artifacts.md](references/artifacts.md): the matching lane for an interface, site, HTML slide deck, or visual document, including its relevant delivery checks.
- [motion.md](references/motion.md): whether something should animate, the cheapest tool, curves, durations, springs, interruption, and reduced motion when the artifact adds or changes motion.

Use `compozy__skill_view` with `name: open-design` and `file: references/craft.md`, `references/artifacts.md`, or `references/motion.md` when available. If the native skill reader is unavailable or denies access, report the reference as unavailable; do not bypass the managed-session reader. Reuse already-read guidance while it remains relevant.

## Choose the target

Two targets exist. Pick one per request and say which you used:

- **HTML:** a directly openable file under `docs/design/`. Use it for sites, slides, visual documents, quick sketches, projects without a component library, or when the user asks for HTML.
- **Library:** a prototype built from the project's own components and rendered by the project's toolchain. Use it when the user asks for it, or when `docs/design/prototyping.md` sets `target: library` and the request does not ask for HTML.

An explicit user choice always wins. When the project has a component library but no record, mention that the library target is available and continue with HTML unless the user asks for it.

## Author an HTML artifact

Write `docs/design/<slug>/index.html` in the active workspace by default. Explicit requested paths under `docs/design/` take precedence. Keep supporting assets there with ordinary relative links, or inline them. The output must open from a local file without a build, service, custom viewer, installed-extension paths, or remote runtime dependencies.

## Author a library prototype

Read `docs/design/prototyping.md`. When it is missing, or its commands or preview fail, load `open-design-setup` and finish that setup before designing. Then:

- Write the prototype under the recorded `prototypes` directory using its `entry` pattern, importing UI only from the recorded `library` paths. Learn components, props, and examples from the recorded catalog before using them. Do not copy, fork, or locally restyle library components; when the library lacks something the design needs, compose existing parts and state the gap. Add a missing component through the project's recorded mechanism only when the user approves that change.
- Use the library's tokens and variants instead of raw colors, sizes, or fonts. Use local fixtures or the project's mocks; never call product APIs.
- Run the recorded typecheck; invented props or imports must fail here, not in review.
- Start the recorded dev command when the preview URL does not respond, and stop servers you started when the work ends.
- Refresh the rendered evidence at `docs/design/<slug>/index.html` with the recorded export or snapshot method, so lint and review inspect what users see. Explicit requested evidence paths under `docs/design/` take precedence.

## Shared authoring rules

Use complete, brief-derived content and accurate assets. Label illustrative data; never invent sources, endorsements, metrics, or backend guarantees. A named real person, product, brand, artwork, or place needs its correct asset, not a fabricated look-alike. Reuse approved local assets first; disclose a necessary labeled placeholder when the correct asset is unavailable. Preserve image proportions and required attribution.

Give meaningful regions and controls stable `data-od-id` values for precise feedback. Implement local demonstrations of the requested interactions without calling product mutation APIs. Keep prototype or presenter controls out of the artifact unless needed by its format or requested by the user.

Refine the same files in place, preserving approved decisions and unrelated content. Do not create a version system, extra registry, or mandatory companion document. This workflow creates design artifacts and prototypes; it does not edit production application code or publish them. Library setup changes only the prototyping environment the user approved.

## Verify and deliver

Read back the changed files, check assets and content completeness, and exercise the primary interaction when tooling permits. For a library prototype, open the recorded preview URL; the rendered evidence must match the current source. Invoke `ext__open_design__lint_artifact` with the actual workspace-relative HTML or rendered-evidence paths. Inspect its findings, fix applicable defects, and explain an intentional exception with the specific user or project authority. In rendered library evidence, a finding caused by the library's own shipped styles is an exception sourced to the library, not a prototype defect. Report unavailable lint tooling as unverified.

When rendered inspection would resolve a visual or interaction risk, use the bundled `open-design-browser` skill with an isolated task-owned CLI session. Do not substitute `browser-use`, attach to an existing browser, or operate the user's live browsing session. Load the resulting screenshot through the harness's image-reading capability before claiming visual inspection; an accessibility snapshot or screenshot path is not visual proof. Keep checks proportional to the change. Missing browser support need not block an HTML sketch, and does not justify an automatic installation.

Link the actual HTML files, or the prototype sources and preview URL, and briefly describe the result, checks performed, and material limitations. Do not repeat their source in chat. Recheck affected areas after corrections and reuse valid evidence for unchanged areas.

## Explicit independent review

Load `open-design-review` only when the user requests the independent review and refinement cycle. Routine creation, edits, or `_uiux.md` input alone stay in the designer conversation. When acting inside that loop, follow its supplied schema without starting another run.
