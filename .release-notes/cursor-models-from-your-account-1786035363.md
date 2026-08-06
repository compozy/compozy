---
title: Cursor models come from your account
type: feature
---

Cursor used to look curated in Compozy but was not truthful to the account that was signed in: a small hand-written list stood in for the real catalog and, worse, acted as an allowlist that rejected valid model ids before Cursor ever saw them. Compozy now reads the account catalog from `cursor-agent models` before a session exists, and exact provider model ids are forwarded unchanged. (#320)

- The first catalog read bootstraps Cursor discovery once and persists the outcome; later reads serve the cache, and explicit refresh is the refresh boundary. In the QA account this surfaced 193 real models, including `composer-2.5`.
- Only `id - display name` rows are parsed. Headings, tips, duplicates, and empty output can never become invented models.
- Curated data is metadata again, not membership policy. Sessions and Loops accept ids like `cursor/composer-2.5`, and an unknown _provider_ still fails with a structured `unknown_provider` error.
- `providers.<id>.models.discovery` applies live — no daemon restart. A provider outage records the failure and keeps the rows you already have; disabling discovery clears them and records `disabled`.
- In the web runtime selector, "Use an exact custom model ID…" now opens a dedicated field: empty input cannot be committed, typing turns the action into `Use "<id>"`, Enter and click both commit, and closing returns to normal catalog search.
- Cursor keeps the operator `HOME` its `native_cli` login contract expects.

Migration notes: the curated Cursor allowlist and its session preflight are deleted with no compatibility bridge. If a provider rejects an id, that provider's error is now the authority.
