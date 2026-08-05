---
title: Live changelog and composer fixes
type: fix
---

The changelog on `compozy.com` now reads published releases directly from GitHub at request time instead of depending on a bot pushing a generated page back into `main` after every release. Each release gets its own page with rendered Markdown, category sections, evidence, compare links, and downloadable assets, plus an RSS feed at `/changelog/feed.xml`, and releases now appear in site search, the sitemap, and the text feeds that agents read. (#292)

- Typing in the session composer no longer swallows spaces.
- A window-manager WebSocket upgrade that fails for a missing workspace now returns a proper preflight error frame, and the web client refreshes a stale workspace list when it sees that error instead of staying stuck.

Migration notes: the release workflow no longer publishes a site changelog receipt commit, and the generator scripts behind it are removed.
