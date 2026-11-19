---
title: See missed notices and failed automations in the attention bell
type: feature
---

A notice sent with `compozy notify` or `compozy__notify` while no CompozyOS window is open is no
longer lost. The daemon stores it, and the sending profile sees it in the attention bell after it
reconnects, even after a daemon restart. Such a notice still reports `no-client`. Muted and
rate-limited notices are not stored.

Failed job and trigger runs now appear under **Needs you** in the attention bell, across profiles,
for User and registered projects. Select a failure to open its job or trigger without switching the
selected profile. Marking it as read records the acknowledgement for the current profile and
operator and leaves the run history unchanged.
