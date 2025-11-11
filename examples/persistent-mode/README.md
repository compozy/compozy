# Persistent Mode Playground

Demonstrates Compozy's persistent mode, which keeps database, Temporal, and
Redis state under the local `.compozy/` directory. Use this profile for daily
development when you want workflows, schedules, and cache data to survive
restarts.

## Prerequisites

- Go 1.25.2+
- Local filesystem write access to the project directory
- OpenAI API key (or update the model provider in `compozy.yaml`)

## Setup

```bash
cd examples/persistent-mode
cp .env.example .env
export OPENAI_API_KEY="sk-your-key"
../../bin/compozy start
```

First startup creates the `.compozy/` folder with:

- `compozy.db` — SQLite datastore for application state
- `temporal.db` — Temporal history and visibility storage
- `redis/` — Miniredis BadgerDB snapshot files

To confirm persistence across restarts:

```bash
../../bin/compozy workflow trigger summarize --input '{"note":"Ship the new modes"}'
../../bin/compozy stop
../../bin/compozy start
../../bin/compozy workflow show summarize --last
```

The workflow history remains available after the restart.

## Cleanup

After testing you can remove the `.compozy/` directory:

```bash
rm -rf .compozy/
```

## Troubleshooting

- `permission denied`: Ensure the project folder allows writes for your user.
- `missing OPENAI_API_KEY`: Export a valid key or swap the provider.
- `database busy`: Persistent mode relies on SQLite; limit concurrent writes or
  serialise heavy operations when running locally.
