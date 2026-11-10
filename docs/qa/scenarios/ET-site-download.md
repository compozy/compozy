---
id: ET-site-download
area: ET
title: /download offers one download for the visitor's machine from the latest release
persona: Dora
journey: J-evaluate-compozy-beta
expected: /download matches the download draft states macos, intel, linux, windows, mobile and unavailable. The primary button reflects the detected platform and the latest release's real file, size and version; the platform switch changes the offer; "All downloads", the terminal methods (each with copy) and the after-download steps follow the draft. When the GitHub release cannot be read, the page falls back to the releases link instead of a broken button.
entry_points: compozy.com /download; draft web/src/drafts/download (states macos, intel, linux, windows, mobile, unavailable)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-site-landing; REL-beta-install-paths
---

E2E-002 of the site redesign spec, story US-004. Exercise the platform switch and every copy button (the clipboard holds the exact command shown).

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft. Reduced motion must keep every section readable.
