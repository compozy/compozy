# Host API Reference

The Host API is the extension -> Compozy callback surface available after a successful `initialize` response.

In TypeScript you access it through `context.host` inside hook handlers or `extension.host` on the `Extension` instance.

```ts
const extension = new Extension("demo", "0.1.0").onPromptPostBuild(async (context, payload) => {
  const tasks = await context.host.tasks.list({ workflow: "demo" });
  return { prompt_text: `${payload.prompt_text}\nTasks: ${tasks.length}` };
});
```

## Global rules

- Calls are allowed only after `initialize` succeeds.
- Calls are denied if the accepted capability set does not authorize the method.
- Calls are rejected during shutdown draining with `shutdown_in_progress`.
- Every call is recorded in `~/.compozy/runs/<run-id>/run.db` (`hook_runs`).
- `host.artifacts.*` is scoped to the workspace root and `.compozy/`.

## Methods

| Method                  | TS client method                | Request type                                                  | Response type                                                  | Capability        | Notes                                                             |
| ----------------------- | ------------------------------- | ------------------------------------------------------------- | -------------------------------------------------------------- | ----------------- | ----------------------------------------------------------------- |
| `host.events.subscribe` | `host.events.subscribe(params)` | `EventSubscribeRequest` `{kinds: EventKind[]}`                | `EventSubscribeResult` `{subscription_id}`                     | `events.read`     | Replaces the current event filter.                                |
| `host.events.publish`   | `host.events.publish(params)`   | `EventPublishRequest` `{kind, payload?}`                      | `EventPublishResult` `{seq?}`                                  | `events.publish`  | Emits a custom extension event onto the bus.                      |
| `host.tasks.list`       | `host.tasks.list(params)`       | `TaskListRequest` `{workflow}`                                | `Task[]`                                                       | `tasks.read`      | Lists task files in a workflow.                                   |
| `host.tasks.get`        | `host.tasks.get(params)`        | `TaskGetRequest` `{workflow, number}`                         | `Task`                                                         | `tasks.read`      | Reads one task with parsed frontmatter.                           |
| `host.tasks.create`     | `host.tasks.create(params)`     | `TaskCreateRequest` `{workflow, title, body?, frontmatter?}`  | `Task`                                                         | `tasks.create`    | Host owns numbering and metadata refresh.                         |
| `host.runs.start`       | `host.runs.start(params)`       | `RunStartRequest` `{runtime}`                                 | `RunHandle` `{run_id, parent_run_id?}`                         | `runs.start`      | Returns once the child run is accepted, not when it finishes.     |
| `host.artifacts.read`   | `host.artifacts.read(params)`   | `ArtifactReadRequest` `{path}`                                | `ArtifactReadResult` `{path, content}`                         | `artifacts.read`  | Reads a workspace-scoped artifact path.                           |
| `host.artifacts.write`  | `host.artifacts.write(params)`  | `ArtifactWriteRequest` `{path, content}`                      | `ArtifactWriteResult` `{path, bytes_written}`                  | `artifacts.write` | Writes a workspace-scoped artifact path.                          |
| `host.prompts.render`   | `host.prompts.render(params)`   | `PromptRenderRequest` `{template, params?}`                   | `PromptRenderResult` `{rendered}`                              | none              | Helper-only, no side effects.                                     |
| `host.memory.read`      | `host.memory.read(params)`      | `MemoryReadRequest` `{workflow, task_file?}`                  | `MemoryReadResult` `{path, content, exists, needs_compaction}` | `memory.read`     | Reads Markdown-backed workflow memory.                            |
| `host.memory.write`     | `host.memory.write(params)`     | `MemoryWriteRequest` `{workflow, task_file?, content, mode?}` | `MemoryWriteResult` `{path, bytes_written}`                    | `memory.write`    | Uses the workflow-memory writer, not the generic artifact writer. |

## Notes by namespace

### `host.events`

- The initial subscription is the unfiltered bus.
- Calling `subscribe` narrows the filter and replaces any previous one.

### `host.tasks`

- `create` is the correct way to add new task files from an extension.
- Do not write task files manually through `host.artifacts.write`.

### `host.runs`

- The host appends the current run to the parent-run chain.
- Calls are rejected once the chain depth reaches 3 with `recursion_depth_exceeded`.

### `host.artifacts`

- Paths must stay under the workspace root or `.compozy/`.
- Out-of-scope paths fail with `path_out_of_scope`.

### `host.prompts`

- The built-in prompt renderer is useful for helper flows and follow-up tasks.
- The method has no capability requirement because it is read-only and side-effect free.

### `host.memory`

- Memory documents are Markdown files under `.compozy/tasks/<workflow>/memory/`.
- Omitting `task_file` targets `MEMORY.md`.
- `mode: "append"` appends with a newline separator.

## Review providers

Extensions can register executable review providers that Compozy calls during `reviews fetch` and `reviews fix` workflows. The host dispatches `fetch_reviews` and `resolve_issues` JSON-RPC calls to the extension subprocess.

### Registration

**Go SDK:**

```go
ext := extension.New("my-provider", "0.1.0").
    RegisterReviewProvider("my-reviews", extension.ReviewProvider{
        FetchReviewsFunc: func(ctx context.Context, rctx extension.ReviewProviderContext, req extension.FetchRequest) ([]extension.ReviewItem, error) {
            // fetch review items from your source
            return items, nil
        },
        ResolveIssuesFunc: func(ctx context.Context, rctx extension.ReviewProviderContext, req extension.ResolveIssuesRequest) error {
            // mark issues as resolved in your source
            return nil
        },
    })
```

**TypeScript SDK:**

```ts
const extension = new Extension("my-provider", "0.1.0").registerReviewProvider("my-reviews", {
  fetchReviews: async (ctx, req) => {
    /* return ReviewItem[] */
  },
  resolveIssues: async (ctx, req) => {
    /* mark resolved */
  },
});
```

### Capability requirement

The manifest must declare `providers.register` and list the provider under `[[providers.review]]`:

```toml
[security]
capabilities = ["providers.register"]

[[providers.review]]
name = "my-reviews"
kind = "extension"
```

### Types

| Type                    | Fields                                                                                                                          | Description                                             |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| `ReviewProviderContext` | `Provider string`, `Host *HostAPI`                                                                                              | Per-call context with provider name and Host API access |
| `FetchRequest`          | `PR string`, `IncludeNitpicks bool`                                                                                             | Input for fetching review items from a PR               |
| `ReviewItem`            | `Title`, `File`, `Line`, `Severity`, `Author`, `Body`, `ProviderRef`, `ReviewHash`, `SourceReviewID`, `SourceReviewSubmittedAt` | One normalized review comment                           |
| `ResolvedIssue`         | `FilePath string`, `ProviderRef string`                                                                                         | One resolved issue reference                            |
| `ResolveIssuesRequest`  | `PR string`, `Issues []ResolvedIssue`                                                                                           | Input for marking issues as resolved                    |

### RPC methods

| JSON-RPC method  | Direction        | Request                             | Response       | Notes                                    |
| ---------------- | ---------------- | ----------------------------------- | -------------- | ---------------------------------------- |
| `fetch_reviews`  | host → extension | `{provider, pr, include_nitpicks?}` | `ReviewItem[]` | Called once per provider per fetch cycle |
| `resolve_issues` | host → extension | `{provider, pr, issues}`            | `null`         | Called after successful fix round        |

### Bridge behavior

When the host needs to call an extension-backed review provider:

1. If an active extension session already exists for the workspace, it reuses that session.
2. Otherwise, it starts a standalone single-extension manager for the duration of the review workflow.
3. The bridge is closed after the workflow completes, shutting down the standalone manager if one was started.

## Error handling

The SDK surfaces host failures as JSON-RPC errors. Common cases:

- `-32001 capability_denied`
- `-32003 not_initialized`
- `-32004 shutdown_in_progress`
- `-32601 method_not_found`
- `-32603 internal_error`
