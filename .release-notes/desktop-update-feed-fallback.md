---
title: Desktop app updates download on networks that block the update feed
type: fix
---

When the desktop app cannot reach its update feed, it now downloads the release the daemon already selected straight from the GitHub release and verifies it against the release's signed checksum. A failed update now says which network error stopped the download, such as `net::ERR_CONNECTION_CLOSED`, instead of "The app update could not be installed."

Migration notes: the fix ships in the updater itself. If updating from 0.6.0, 0.6.1, or 0.6.2 keeps failing with a connection error, download this release once from the GitHub release page and install it manually; later updates use the fallback automatically.
