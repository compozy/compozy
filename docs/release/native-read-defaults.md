# Bounded native read defaults

## Migration notes

Automation run lists and job/trigger histories now return 50 records per page by default, with a
maximum of 200. Omitted and zero limits use 50. Follow `page.next_cursor` as `cursor` until
`page.has_more` is false; keep filters and profile/workspace unchanged. CLI users pass `--last`
and `--cursor`; every output format exposes continuation metadata. When paging with time
filters, use the same absolute RFC3339 timestamp starting with the first request. Relative durations such as
`--since 1h` cannot be reused with `--cursor`. All records remain stored.

Worktree lists omit removed records. Set native `include_removed: true`, HTTP/UDS
`include_removed=true`, or CLI `--include-removed` to include them. Other states and discovered
checkouts retain their visibility.

Tool search defaults to 20 matches and caps a page at 100. Set `offset` to `next_offset` while
`has_more` is true, keeping the same query. CLI uses `--limit` and `--offset`. Complete descriptors
remain available on every returned row and through tool info.

These are regime-2 public default changes, shipped directly under the scoped operator-approved
exception for these three read families, with this migration block. Other public surfaces retain
SD-013's normal migration and deprecation requirements. There is no state migration, deletion,
result-view selector, or compact/full schema change.

Workspace description also uses the registration ID for session ownership, including when the
directory metadata has another ID. Session reads honor the selected profile; HTTP/UDS callers
can use `profile` or operator-only `all_profiles`, and `compozy workspace info` supports
`--profile` and `--all-profiles` to select the required sessions.

Loop runs pin tool schema digests at Start. An in-flight run pinned to an older descriptor keeps
its stored state but may stop with `action_contract_stale` after this upgrade. After updating the
caller to page results, start a new run (`compozy loop run <name>`) to capture the current contracts.
Stored Loop definitions are not rewritten, and previous run history remains intact.
