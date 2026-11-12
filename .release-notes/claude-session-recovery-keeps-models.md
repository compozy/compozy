---
title: Claude sessions recover with their exact model after you sign in again
type: fix
---

A Claude session pinned to a full model ID such as `claude-opus-5-5` now resumes on that model after a sign-in recovery. Before, a reloaded session could see only a stale or offline model list that left out the pinned model, so recovery failed with a model error even after you signed back in and **Usage** showed the account as healthy.

CompozyOS now checks Claude's local sign-in state before it selects a model, keeps the session's exact model through the reload, and reports the real cause when recovery still fails: a missing sign-in, a model the provider refuses, or the last replacement error. A parent watching a child that is waiting for you to sign in is now told so, instead of being told the child is waiting for permission.

Migration notes: no action needed.
