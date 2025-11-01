# Compozy SDK v2 Examples

Every example in this directory is runnable with the functional-options SDK. Execute a scenario using

```bash
go run sdk/examples --example <name>
```

## Prerequisites

- Go 1.25.2+
- Bun installed locally (required for `runtime-native-tools`)
- `OPENAI_API_KEY` exported when running `knowledge-rag` or `complete-project`

## Example Catalog

| Example | Command | What it Demonstrates |
|---------|---------|----------------------|
| simple-workflow | `go run sdk/examples --example simple-workflow` | Minimal mock-backed agent executed synchronously |
| parallel-tasks | `go run sdk/examples --example parallel-tasks` | `task.NewParallel` fan-out/fan-in with aggregation |
| knowledge-rag | `go run sdk/examples --example knowledge-rag` | Markdown ingestion, OpenAI embeddings, retrieval-grounded answer |
| memory-conversation | `go run sdk/examples --example memory-conversation` | Session memory with multi-turn dialogue |
| runtime-native-tools | `go run sdk/examples --example runtime-native-tools` | Native Go tool + Bun inline script working side-by-side |
| scheduled-workflow | `go run sdk/examples --example scheduled-workflow` | Cron schedule config and deterministic first-tick simulation |
| signal-communication | `go run sdk/examples --example signal-communication` | Signal and wait task coordination with payload hand-off |
| model-routing | `go run sdk/examples --example model-routing` | Per-task model overrides for routing and fallback chains |
| debugging-and-tracing | `go run sdk/examples --example debugging-and-tracing` | Capturing exec telemetry for troubleshooting |
| complete-project | `go run sdk/examples --example complete-project` | Project-level config spanning tools, knowledge, memory, and schedules |

## Development Notes

- Constructors mirror runtime defaults—no builders or `Build()` calls.
- Context flows top-down from `main`; the helpers avoid `context.Background()` in execution paths.
- Examples respect `.cursor/rules` (no stray blank lines, functions < 50 lines, named constants for non-trivial values).
- `knowledge-rag` and `complete-project` create temporary markdown content and rely on OpenAI to embed and answer questions.
