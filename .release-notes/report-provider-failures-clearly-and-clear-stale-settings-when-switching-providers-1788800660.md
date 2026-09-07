---
title: Report provider failures clearly and clear stale settings when switching providers
type: fix
---

Loop actions preserve quota, authentication, transport, timeout, and refusal failures before validating the model's output. An expired login or usage limit is no longer disguised as an invalid JSON result; genuine malformed model output still fails schema validation.

Changing an Agent's provider with `compozy agent update` clears the previous provider's model, command, reasoning effort, and ACP options unless you explicitly supply replacements. Providers that do not support reasoning configuration reject it during resolution instead of failing later at session startup.

PRs: [#545](https://github.com/compozy/compozy/pull/545), [#546](https://github.com/compozy/compozy/pull/546).
