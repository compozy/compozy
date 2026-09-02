---
title: A parent Loop can reconfigure one child run
type: feature
---

The reserved `run-loop` action accepts an optional `params.config_overrides` object, so a parent Loop can give one child run its own iteration limits, budgets, environment, reattempt behavior, and runtime selection without touching the child's stored configuration. (#494)

- Works with both `await` and `detach`.
- Exact node-output references stay typed, and unknown literal fields are rejected before the child starts.
- Override values can be filled from templates.
- The Loop editor exposes the overrides as JSON, and malformed JSON, unknown settings, wrong types, or trailing data block the run instead of failing mid-flight.
