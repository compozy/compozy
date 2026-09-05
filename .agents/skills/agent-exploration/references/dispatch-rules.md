# Dispatch Rules

Use native subagents. The parent names each slice's question, source scope, ordinal/slug, and exact analysis output path. If a required boundary is ambiguous, the worker asks the parent before writing.

- Workers may read/search the named sources and use read-only Git inspection. Web lookup applies only to web-scoped questions. They may write or refine only their named analysis file; all product code, other artifacts, and version-control state stay untouched.
- The parent creates the output directory and assigns independent slices within available capacity, up to 8 concurrent workers. No fixed minimum or same-batch requirement.
- Inherit the configured model and reasoning unless an explicit supported override was supplied. Do not silently replace a user-selected model.
- State the contract once per dispatch, without embedding redundant rule documents. A worker may refine its own artifact after inspection; one authorized file does not mean one lifetime write.
- Verify substantive claims against cited evidence and required output structure. Missing sources become explicit limitations. Repair only the affected slice; preserve valid sibling outputs and reuse current context.
- Unexpected writes outside scope are investigated before continuing. Do not discard another agent's work or run destructive Git as cleanup.
- Synthesis describes what was actually investigated, including unresolved slices. It must never present a missing worker's output as completed research.
