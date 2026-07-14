# Architecture audit E2E evaluation

This directory is the durable, model-backed evaluation boundary for the skill-only extension. It
does not mock an audit: the test installs the exact shipped skill pack into a disposable workspace,
invokes it through `compozy exec`, and inspects the reports and the generated depth map.

The two fixture workspaces are deliberately language-distinct:

- `testdata/typescript` contains a broad checkout facade whose callers repeat a multi-step protocol.
- `testdata/go` contains an idiomatic functional-options constructor that must be treated as healthy
  thinness rather than a shallow-module finding.

Run the agent-backed check only with an available ACP runtime. The default test suite skips it because
model execution is external and nondeterministic; it never reports a skipped model evaluation as a pass.

```bash
make go-build
COMPOZY_RUN_SKILL_E2E=1 COMPOZY_E2E_BINARY="$PWD/bin/compozy" \
  go test -count=1 -v ./extensions/cy-improve-architecture/evals
```

Set `COMPOZY_E2E_IDE` or `COMPOZY_E2E_MODEL` when another configured ACP runtime or model is needed.
The test defaults to Codex's supported `gpt-5.6-sol` model rather than inheriting a workspace default.
It verifies the installed-skill boundary, report publication, required report sections, the protected
instruction and configuration files, and `archmap.Parse` over the produced `ARCHITECTURE.md`.
`SCENARIOS.md` keeps the full behavioral contract visible for the agent-evaluation run; E2E-028 remains
deferred by the product specification.
