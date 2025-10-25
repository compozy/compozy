# calltask builtin

Implements the `cp__call_task` native tool. The handler mirrors the structure of
`cp__call_agent`, delegating execution to `toolenv.TaskExecutor` and enforcing
configuration defaults from `config.NativeToolsConfig`.
