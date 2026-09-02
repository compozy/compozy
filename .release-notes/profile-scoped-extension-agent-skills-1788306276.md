---
title: Profile-scoped extension Agents keep their own Skills
type: fix
---

An extension Agent scoped to one Profile leaked into `default`, and Agent-local Skill lookup could read the wrong Agent definition. CompozyOS now publishes the already-projected Agent set and resolves Agent-scoped Skill queries through the selected global, Profile, or Workspace lens, passing the concrete Agent definition to the Skill registry. (#516)

- Two Agents with the same name in different Profiles stay isolated, and `skill list|where|view --for-agent` returns each one's own Skill body.
- Profile-only scope keeps both the Profile ID and the Profile name.
- No command, flag, route, or payload shape changed — existing reads simply return the correctly scoped resources.
