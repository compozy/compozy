---
title: Synthetic wakes reach long turns with provider defaults
type: fix
---

CompozyOS promotes queued session replies, monitor wakes, and child completion wakes into long-running turns even when the provider adds ACP defaults to the live runtime. Inputs with an explicit different runtime still wait for their own turn, and debug logs identify runtime-mismatch skips without exposing runtime values.

Migration notes: no action needed.
