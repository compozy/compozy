---
id: ET-site-download
area: ET
title: /download offers one download for the visitor's machine from the latest release
persona: Dora
journey: J-evaluate-compozy-beta
expected: /download matches the download draft states macos, intel, linux, windows, mobile and unavailable. The primary button reflects the detected platform and the latest release's real file, size and version; the platform switch changes the offer; "All downloads", the terminal methods (each with copy) and the after-download steps follow the draft. When the GitHub release cannot be read, the page falls back to the releases link instead of a broken button.
entry_points: compozy.com /download; draft web/src/drafts/download (states macos, intel, linux, windows, mobile, unavailable)
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/site-redesign/qa/R4/ET-site-download/walk.jsonl; .compozy/tasks/site-redesign/qa/R4/ET-site-download/macos-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-download/macos-1440-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-download/intel-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-download/linux-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-download/windows-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-download/mobile-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-download/mobile-390-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-download/unavailable-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-download/unavailable-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-download/all-downloads-1440-site.jpg
last_report: docs/qa/reports/2026-11-10-site-redesign-r4.md
overlaps: ET-site-landing; REL-beta-install-paths
---

E2E-002 of the site redesign spec, story US-004. Exercise the platform switch and every copy button (the clipboard holds the exact command shown).

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft. Reduced motion must keep every section readable.

QA impact 2026-11-10 (review fixes R1): the primary button and the "All downloads" rows download the file in place (no new tab); GitHub pages still open in a new tab. Copy buttons show Copied or Copy failed and return to idle.

QA result 2026-11-10 (R4 walk, `next start`, 1440 and 390, dark): pass. macOS Apple silicon (DMG 215 MB), Intel (DMG 227 MB), Linux (AppImage 209 MB, deb link), Windows (command ZIP 59 MB) and phone states were detected from emulated user agents and client hints and matched their drafts (height within 9px); the platform switch changes the offer and the after-download steps; every "All downloads" row points at the v0.5.5 asset without `target`; all four terminal methods copy exactly the command shown; the phone state copies `https://compozy.com/download`. With GitHub unreachable (a throwaway copy of the same build with `api.github.com` fetches failing, ISR regenerated) the page offers "Open the latest release" and every row links to GitHub releases instead of a broken button. No overflow, no console error besides the documented noise.
