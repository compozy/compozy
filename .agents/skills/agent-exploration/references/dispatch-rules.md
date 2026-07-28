# Dispatch Rules

The `explorer` agent launched by this skill is registered globally in the Compozy agent registry at `~/.compozy/agents/explorer/AGENT.md` (sourced from `assets/AGENT.md`). The parent dispatches it via `compozy exec --agent explorer`, never through a harness-specific subagent tool. Every dispatched run operates under a strict **scoped-write** contract — exactly one file-write to the named target path, every other action read-only. The rules below MUST be embedded in every dispatched prompt verbatim.

## Scoped-Write Contract

1. The parent prompt MUST name three things:
   - The slice scope (primary source paths, directories, URLs, or topical bounds).
   - The slug and ordinal (`NN_analysis_<slug>`).
   - The exact target analysis file path (`<path>/analysis/NN_analysis_<slug>.md`).
     If any of the three is missing or ambiguous, the agent returns a clarification request and writes nothing.
2. The agent MAY perform exactly one file-write, and only at the target path the parent named.
3. The agent MUST NOT edit any existing file. MUST NOT write to any other path. MUST NOT create directories outside the named analysis directory.
4. The agent reads only the slice scope the parent named (local paths or URLs). For web-scoped slices, web fetch/search is allowed but must stay aligned with the slice question.
5. The agent MUST NOT run state-mutating shell commands: no `git`, `make`, `bun`, `npm`, `pnpm`, `mv`, `rm`, `cp` of non-trivial trees, `>`, `>>`, or any command that touches the working tree outside `<path>/analysis/`.
6. If the agent encounters a source that requires interpretation by another tool (compiled binary, encrypted blob, paywalled URL), it records a note in the **Open Questions** section and continues.

## Tool Restrictions

- **Allowed:** read-only filesystem inspection (`Read`, `Grep`, `Glob`, `find`, `wc -l`, `head`, `cat`, `ls`, `file`, `rg`), web fetch/search when the slice scope authorizes it, and exactly one file-write at the named target path.
- **Forbidden:** edits to any existing file; writes to any path other than the named target; mutating shell commands (`rm`, `mv`, `>`, `>>`, `git`, `make`, package managers).

## Parent Responsibilities

- The parent agent MUST verify `~/.compozy/agents/explorer/AGENT.md` exists before dispatch. If absent, the parent MUST offer to install from `assets/AGENT.md` via `scripts/install-explorer.sh` before continuing. Workspace-scoped overrides at `<repo>/.compozy/agents/explorer/AGENT.md` take precedence over the global definition (per Compozy registry rules) and satisfy the existence check.
- The parent agent MUST ensure `<path>/analysis/` exists before dispatch (the agent will refuse to write into a missing directory rather than creating it).
- The parent agent MUST invoke each slice via `compozy exec --agent explorer --ide <ide> --model <model> --reasoning-effort <reasoning> "<slice-prompt>"`. The `--ide`, `--model`, and `--reasoning-effort` values are forwarded from the operator's `--ide`, `--model`, and `--reasoning` inputs (defaults: `claude`, `opus`, `xhigh`). `compozy exec` already defaults `--access-mode` to `full`, so no extra runtime-permission flag is required.
- The parent agent MUST embed all three names — slice scope, slug+ordinal, target file path — explicitly in the slice prompt, along with this `dispatch-rules.md` and the seven-section schema from `assets/analysis-template.md`, verbatim.
- The parent agent MUST scout the territory itself first (Step 3 of the SKILL.md) so each slice is non-overlapping and independently answerable.

## Parallelism

- All `compozy exec` invocations in a research round run in parallel via the harness's async/background execution facility (whatever lets the parent issue N parallel tool calls and wait for all to finish). Do not stagger.
- Wait for every `compozy exec` process to exit before verification. A partial set is unacceptable.
- The hard cap is 8 concurrent invocations per round. Use fewer when the scout reveals fewer non-overlapping slices.

## Output Validation

Each dispatched run writes a file containing all seven sections from `assets/analysis-template.md` (Overview, Mechanisms/Patterns, Relevant Sources, Transferable Patterns, Risks/Mismatches, Open Questions, Evidence). After dispatch the parent:

1. Confirms every `compozy exec` invocation exited with code 0. Any non-zero exit is a slice failure that must be re-dispatched.
2. Lists `<path>/analysis/` and confirms one file per dispatched slice at the expected `NN_analysis_<slug>.md` path.
3. Re-reads each file to confirm all seven sections are present.
4. Sample-checks at least one cited source per file — `Read` for local paths, well-formedness check for URLs — to confirm evidence is real, not fabricated.
5. If any section is empty or any cited source is fake, re-dispatches the offending slice with the schema and a request to fill the gap. The parent never authors the missing content — the dispatched agent owns the write.

## Failure Handling

- If a `compozy exec` invocation exits non-zero or returns malformed output, retry once with a stricter prompt restating the scoped-write contract.
- If the dispatched agent reports the slice scope is empty or unreachable, it returns a clarification request and writes nothing. The parent decides whether to merge that slice into an adjacent slice or drop it.
- If the dispatched agent violates the scoped-write contract (writes outside the named path, edits an existing file, runs `git`/`make`/etc.), treat it as a contract violation: stop, re-read this file, and re-dispatch with the contract restated verbatim in the slice prompt.
- If the `compozy` binary is missing from `PATH`, abort the round with a one-line message instructing the operator to install Compozy. Do not attempt to fall back to harness-native subagent tools.
- Do not synthesize a missing slice as if its analysis succeeded.
