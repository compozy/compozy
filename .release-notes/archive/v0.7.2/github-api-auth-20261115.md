---
title: Reuse existing GitHub credentials and honor quota deadlines
type: fix
---

GitHub update checks, marketplace sources and extension discovery use `GITHUB_TOKEN` from the
daemon environment, then the daemon user's `gh auth token` login for `https://api.github.com`,
then anonymous access. Custom API origins never receive the CLI's github.com credential.
Credentials require HTTPS, with existing local HTTP registries preserved on `127.0.0.1`, `::1`,
or `localhost` at the exact configured origin and port, including publishing. Other HTTP GitHub
API endpoints fail before sending a request and explain how to switch to HTTPS.

Quota failures honor GitHub's retry deadlines, preserve the last good plugin catalog, and explain
how to authenticate or retry. A new login takes effect after the retry deadline or a daemon restart.
