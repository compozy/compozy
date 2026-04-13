# Issue 001: update notifier pollutes headless stderr

## Summary

`compozy exec --format json` and `compozy exec --format raw-json` still emit the upgrade notifier after command completion, adding unsolicited text to stderr during machine-readable runs.

## Reproduction

```bash
./bin/compozy exec --ide codex --format raw-json "Reply with EXACTLY: STREAM_CHECK" >stdout.txt 2>stderr.txt
cat stderr.txt
```

Observed before the fix:

- `stdout.txt` contained valid JSONL events.
- `stderr.txt` contained the colored `Update available` banner.

## Expected

Machine-readable/headless modes should keep stderr quiet unless the command explicitly asked for verbose operational logs or an actual execution error occurred.

## Root cause

`cmd/compozy/main.go` always wrote the upgrade notification after `Execute`, regardless of the executed subcommand and resolved `--format`.

## Fix

Only emit the notifier when the executed command is not running in `json` or `raw-json` mode.
