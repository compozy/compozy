# Hosted MCP list baseline

This fixture captures the JSON result returned by the MCP SDK client against the
unbudgeted server from commit `09565e71d8d1abf0a9897fd116dca331719e740b`.
The generator uses the base commit's registration, tool metadata, result framing,
and private cache hints.

From the repository root, generate it in a fresh scratch directory.

```sh
repo="$PWD"
base_snapshot="$(mktemp -d)"
git archive 09565e71d8d1abf0a9897fd116dca331719e740b | tar -x -C "$base_snapshot"
cp internal/mcp/testdata/hosted_tools_list_09565e71d.go.txt \
  "$base_snapshot/internal/mcp/hosted_list_baseline_test.go"
HOSTED_LIST_BASELINE_OUTPUT="$repo/internal/mcp/testdata/hosted_tools_list_09565e71d.json" \
  go -C "$base_snapshot" test -p 2 -count=1 -run '^TestEncodeHostedListingBaseline$' ./internal/mcp
```
