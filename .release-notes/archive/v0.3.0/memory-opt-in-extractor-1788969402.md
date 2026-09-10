---
title: Memory and dreaming require explicit opt-in
type: fix
---

CompozyOS is a control plane, so it no longer starts persistent memory extraction or background
dreaming from omitted settings. `memory.enabled` and `roles.dream.enabled` now default to `false`.
To enable memory, set `memory.enabled = true` in `config.toml` and restart the daemon. Dreaming
requires the separate `roles.dream.enabled = true` opt-in; enabling memory alone leaves it off.

The enabled extractor now requests an explicit no-candidate result, accepts conventional empty
responses and JSONL fences, and preserves valid candidates from mixed output. Malformed lines
remain diagnosable failures instead of silently disappearing. Extraction failures and timeouts
appear in the configured DLQ and extractor failure listing. Child-stop details distinguish failed
or timed-out extraction from a parsed child response; successful inbox production owns the
`memory.extractor.completed` event.

Session pressure compaction remains independently controlled by `session.compaction.enabled`.
It reuses checkpoint coverage and may launch a summary child when an active session reaches the
pressure threshold, even with persistent memory disabled. Idle sessions and session-end memory
updates do not start that work. An explicit checkpoint-role opt-out or a failed summary leaves
uncovered events unarchived.

### Migration notes

No configuration is rewritten and no stored memory is deleted. Existing explicit `true` or `false`
values retain their meaning; omitted values use the new disabled defaults. Profile and workspace
role overrides keep their precedence under the daemon memory master switch. Existing public keys,
tools, routes, and response shapes remain available. Raw extraction failures require a new
extraction; replay remains limited to normalized candidate inbox failures. (#561)
