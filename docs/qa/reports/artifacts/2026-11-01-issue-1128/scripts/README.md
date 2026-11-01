# Isolated lab replay scripts

These scripts were corrected after review. Existing sibling evidence files are historical captures;
syntax checks of these corrected scripts do not establish a new runtime pass.

Bootstrap a new isolated lab and supply its absolute path with `LAB` and its registered workspace ID
with `WS`. The scripts source `qa-artifacts/qa/bootstrap.env` inside that lab. Install the saved agent
fixtures and lab binary first. Never point these scripts at the operator runtime. Invoke phases with
`LAB=/absolute/path/to/isolated-lab WS=workspace-id bash scripts/phaseN.sh`.

Set `MONITOR_WAIT_SECONDS` to cover the expected timer deadline (default 1800 seconds). Failed steps
return nonzero. Restart waits require the exact operation to reach `ready`; timer timeout saves a
marker and diagnostic snapshots before returning failure. Run long phases through a managed terminal
and register a monitor. Retain fresh outputs separately from the historical captures.
