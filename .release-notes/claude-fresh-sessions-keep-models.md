---
title: New Claude sessions start on the exact model you selected
type: fix
---

A new Claude session started with a full model ID such as `claude-opus-5-5` could fail at startup when Claude's model list offered only older IDs or aliases, while the same model worked on another attempt. New sessions, including input-only sessions, now register the selected model the same way resumed sessions already did, so they start on that exact model. If Claude refuses or ignores the model, the session fails once with an error that names the provider and the model. It never switches to an alias or an older model.

Migration notes: no action needed.
