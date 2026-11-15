---
title: Reuse existing GitHub credentials and honor quota deadlines
type: fix
---

GitHub update checks, marketplace sources and extension discovery use `GITHUB_TOKEN` from the
daemon environment, then the daemon user's `gh auth token` login for `https://api.github.com`,
then anonymous access. Custom API origins never receive the CLI's github.com credential.
Credentials require HTTPS, including publishing to custom endpoints; HTTP endpoints remain anonymous.

Quota failures honor GitHub's retry deadlines, preserve the last good plugin catalog, and explain
how to authenticate or retry. A new login takes effect after the retry deadline or a daemon restart.
