---
title: Removing the suggested Home folder no longer breaks the desktop
type: fix
---

Onboarding seeded every daemon registration into the selectable project draft, including the internal operator-home registration that Global runs on. Removing the suggested Home folder deleted that registration and left a desktop where dock apps took focus but opened nothing. (#440)

- Onboarding now partitions project workspaces from the operator home, so that row can never be seeded, added, or deleted as a project.
- The fix is covered by the canonical onboarding suite, with three cases that fail against the previous behavior.
