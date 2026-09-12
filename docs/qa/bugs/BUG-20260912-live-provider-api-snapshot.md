# Live provider additions remain unavailable to agent and auth APIs

- **Impact:** Blocks-Completion.
- **Scenario:** RT-model-catalog-cold-open.
- **Status:** fixed; public retest passed.
- **Report:** ../reports/2026-09-12-pr-630-634-integration.md.

After Settings adds a Claude overlay with `applied=true` and `lifecycle=live-add`, model refresh discovers five rows and model curation succeeds. Creating a global agent using the overlay returns HTTP 400 with `unknown provider`; the auth route returns 404 `provider_not_installed`.

The shared API handlers retained the boot configuration even after Settings published the active generation. Provider inventory, probes and global agent admission/projection now read the existing active-settings snapshot. A snapshot-read failure remains an API failure instead of falling back to stale configuration. No settings lifecycle, public schema or persistence format changes.

The existing `TestProviderAuthHandlers` suite reproduces addition visibility, auth lookup and agent admission before the fix, then covers removal and active-snapshot failure. Full API-core race checks pass. Evidence is in the isolated run's `overlay-settings-create.json`, `overlay-list.json`, `overlay-curation.json`, `overlay-auth.stderr` and `overlay-agent-create.stderr`. The rebuilt daemon accepted a newly added `claude-work` overlay without restarting: auth succeeded,
`project_writer` was created, and session `sess-7cf94f8e5a225034` completed a native prompt. HTTP
readback retained the overlay and logical model while ACP reported `haiku`. Removal and same-command
offline recreation also passed. Delivery CI is tracked separately from this behavioral result.
