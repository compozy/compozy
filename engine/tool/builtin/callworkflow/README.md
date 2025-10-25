# callworkflow builtin

Implements the `cp__call_workflow` native tool. The handler mirrors the
structure of `cp__call_task`, delegating execution to `toolenv.WorkflowExecutor`
and applying configuration defaults from `config.NativeToolsConfig`.
