# calltasks builtin

Implements the `cp__call_tasks` native tool, executing multiple tasks in
parallel using a weighted semaphore and the shared task executor. The
implementation mirrors `cp__call_agents`, emitting telemetry for each task and
respecting native tool configuration limits.
