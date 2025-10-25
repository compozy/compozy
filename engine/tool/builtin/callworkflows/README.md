# callworkflows builtin

Implements the `cp__call_workflows` native tool. The handler mirrors the
parallel execution model from `cp__call_tasks`, delegating to
`toolenv.WorkflowExecutor` while respecting concurrency and timeout
configuration from `config.NativeToolsConfig`.
