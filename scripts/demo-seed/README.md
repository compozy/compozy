# Demo seed

Writes the Northstar Pay scenario into a CompozyOS home: six projects, a team of always-on agents, session trees, Loops and Automations. Everything goes through the stores the daemon uses.

## Seed a home

```bash
make demo-seed DEMO_HOME=/tmp/northstar DEMO_PORT=47931
```

This seeds the home, sets the port, turns on Group by project, raises the always-on limit, and starts the daemon.

To run the steps by hand, stop any daemon on that home first:

```bash
go run ./scripts/demo-seed --home /tmp/northstar --replace
```

## Hold live states

Running turns, Waiting sessions and pending asks do not survive a daemon restart, so the seed does not write them. `--live` creates them against a real daemon, using the mock provider instead of a model:

```bash
make build web-build
go build -o bin/acpmock-driver ./internal/testutil/acpmock/cmd/acpmock-driver
go run ./scripts/demo-seed --home /tmp/northstar --live
```

Or run `make demo-live DEMO_HOME=/tmp/northstar`. Open the app on the home's port. The driver holds these until Ctrl-C:

| Project | Session | State |
| --- | --- | --- |
| payments-platform | Cut settlement reads over to ledger v2 | Waiting on three running subagents with live cards |
| payments-platform | Ship the Mexico partial-refund fix | Waiting, with two running child sessions |
| payments-platform | Investigate the rc-2026.11.3 settlement-replay failure | Working |
| payments-platform | Release Watch main session | Working |
| launch-hq | Pause the Mexico canary | Needs you: pending permission |
| checkout-web | Pick the Pix QR layout for small screens | Needs you: clarify question |
| checkout-web | Migrate the installment selector to the new tokens | Working |
| risk-engine | Rerun the October backtest by partition | Working |
| platform-infra | Rehearse the queue cutover again | Working |

Permission and clarify asks time out, so the driver sends them again when they end.

`--live` refuses a home the seed did not write. While it runs, the home's `config.toml` points Claude and Codex at the mock. Ctrl-C stops the daemon and restores the file.

Flags: `--compozy` (default `bin/compozy`), `--mock-driver` (default `bin/acpmock-driver`), `--web-dist` (default `web/dist`).
