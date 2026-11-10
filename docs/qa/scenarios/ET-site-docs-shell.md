---
id: ET-site-docs-shell
area: ET
title: Docs shell moves between sections, reads a page beside its outline, and searches every page
persona: Dora
journey: J-evaluate-compozy-beta
expected: The docs header shows the seven groups of content/docs/meta.json as section tabs with page counts and selects the current page's tab. The sidebar lists only the current section, expanded to the current page with the lit marker; on mobile it opens from the header button as a drawer. /docs matches the docs draft home (search, install command, first-run commands, the three paths, the numbered section map, then the prose). A docs page matches the tutorial, concept, hub, workflow, reference, config and short states: breadcrumb, title, description, facts, page actions (Copy as Markdown), prose, "On this page" with the beam, previous/next. Numbered "## N." headings render as steps, one-line shell commands as a prompt line with copy, and the MDX blocks as redrawn in the draft. ⌘K or the search field opens the palette with results grouped by section, a section filter, keyboard navigation and Escape to close; results include page bodies. Hash anchors land below the fixed header.
entry_points: compozy.com /docs; /docs/getting-started/installation; /docs/loops; a page with Mermaid and Workflow blocks; ⌘K palette; draft web/src/drafts/docs (states home, tutorial, concept, hub, workflow, reference, config, short, search; the mobile drawer is walked at 390)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-site-docs-single-tree-ia; ET-site-docs-search-context; ET-site-docs-first-session; ET-site-docs-reference-pages; ET-site-docs-sdk-reference
---

E2E-006 of the site redesign spec, stories US-011, US-012, US-013 and US-016. This shell replaced the Fumadocs notebook chrome; the OpenDesign sidebar, masthead and typography contracts it superseded are recorded in their own scenario files as history.

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft.
