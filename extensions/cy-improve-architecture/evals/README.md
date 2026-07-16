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

`COMPOZY_E2E_IDE` selects the ACP runtime. Set `COMPOZY_E2E_MODEL` to override that runtime's
model; when it is unset, the selected runtime resolves its registered default.
It verifies the installed-skill boundary, report publication, required report sections, the protected
instruction and configuration files, and `archmap.Parse` over the produced `ARCHITECTURE.md`.
`SCENARIOS.md` keeps the full behavioral contract visible for the agent-evaluation run; E2E-028 remains
deferred by the product specification.
