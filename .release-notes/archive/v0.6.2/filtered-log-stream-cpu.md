---
title: Idle filtered log streams use less daemon CPU
type: fix
---

Keeping the desktop or web app open no longer makes a quiet, filtered log stream rescan retained event history every second. Live streams start at the physical ledger head and advance past events that do not match their filters, while full pages and reconnect cursors continue draining without skipping delayed events. No migration or operator action is required.
