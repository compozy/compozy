# Memory Mode Quickstart

Run Compozy with fully embedded services for instant startup. Memory mode keeps
all state in-memory, making it ideal for demos, tutorials, and CI smoke tests.
Nothing is written to disk and restarts are instant.

## Prerequisites

- Go 1.25.2+
- Temporal and Redis ports (7233, 8233, 6379) available on localhost
- An API key for the configured model (OpenAI by default)

## Setup

```bash
cd examples/memory-mode
cp .env.example .env                # Optional: fill in OPENAI_API_KEY
export OPENAI_API_KEY="sk-your-key" # Or rely on .env loading
../../bin/compozy start
```

The server should start in under a second. No `.compozy/` directory or other
artifacts are created. To trigger the sample workflow:

```bash
../../bin/compozy workflow trigger echo --input '{"message":"Memory mode works"}'
```

Expected output:

- CLI logs show embedded Temporal and Redis starting in memory mode
- Workflow completes immediately and echoes the provided message

## Troubleshooting

- `missing OPENAI_API_KEY`: Export a valid key or update the model provider in
  `compozy.yaml`.
- `address already in use`: Stop any process using the default ports or change
  them in the config.
- `workflow not found`: Ensure you are running commands from the example
  directory so the workflow definition is discovered.

## Next Steps

- Switch to [Persistent Mode](../persistent-mode/README.md) to keep state across
  restarts.
- Explore [Distributed Mode](../distributed-mode/README.md) for production-style
  deployments.
