---
title: Extension replacement respects profile ownership before publication
type: fix
---

An agent can replace a local extension only when every installation belongs to its workspace and
its own profile or all profiles. An attachment in another profile requires operator authority.
Replacement also resolves the selected installation before staging, so a denied or missing target
never temporarily publishes candidate code to another installation.
