# QA Run Report — 2026-09-10 — mixed session work

- **Scope:** Issue #605, reasoning and tools in the same expandable work segment.
- **Cadence tier:** targeted.
- **Build:** issue branch based on `ed7f2d7a`, working tree changes.
- **Environment:** isolated daemon and workspace, current-source Vite frontend, installed daemon beta.24, real Codex provider. Native CUA Chrome driver.
- **Status:** targeted QA completed; delivery validation tracked in PR #610.

## Personas and flows

Rafa, transcript reviewer, desktop on local fast network, en-US. Existing J-14 and CH-017 own RT-048 and RT-055. Scope is mixed grouping, ordered detail, collapse/expand and search/reveal at wide and narrow widths. Existing suites own terminal/permission boundaries and scroll anchoring.

## Session Matrix & Results

| Charter | Scenario | Tour | Status | Evidence |
|---|---|---|---|---|
| CH-017 | RT-048 | Feature | Pass | wide-expanded.png; narrow-search-tool.png |
| CH-017 | RT-055 | Feature | Pass | narrow-search-reasoning.png |

## Session Debriefs

Preparation: a real read-only provider turn emitted three shell tools, one reasoning segment between tools, and a final assistant response. This engineering probe named the QA directory in its prompt; it is not counted as a persona session. It confirms real mixed activity exists for renderer inspection. The persona walk read the resulting transcript through its public permalink.

Rafa opened the finished session through its public permalink. The first turn showed one collapsed work disclosure and a separate final answer. Expansion revealed Read → reasoning → line count → checksum in the recorded order. Reasoning retained its bold markdown; tool detail and Copy tool payload remained operable (the UI confirmed the copy). Search for the reasoning phrase opened its exact body from a closed turn fold. Search for the checksum command opened that tool's detail instead. Closing the disclosure and reloading kept the same transcript and separate response.

The narrow walk used 390×844; the wide walk used 1440×1000 (initial detail capture used the browser's 1920×907 viewport). The group labels stayed bounded and controls remained reachable. Two further read-only prompts exercised tool updates and separate turn folds. The live mixed interval was too short to capture an expansion during that exact interval; the canonical interaction suite covers mixed reasoning settlement and appended entries, while real browser evidence covers persisted mixed detail, search and boundaries. The final tool-only monitoring turn remained separately inspectable.

Screenshots: `docs/qa/evidence/2026-09-10-issue-605/{wide-expanded,narrow-search-reasoning,narrow-search-tool,wide-three-turns}.png`. The evidence directory is managed by the repository sidecar. Publication is currently blocked: installed `skeeper 0.3.1` rejects this checkout with `core.repositoryformatversion does not support extension: worktreeconfig`. Images remain available in the worktree; they are not claimed as published sidecar artifacts. Cleanup: `docs/qa/evidence/2026-09-10-issue-605/teardown.json`, `clean: true`, zero surviving lab processes.

## Runtime Errors Observed

The isolated daemon initially inherited a host restart-operation identifier and could not find that operation in its empty home. The lab process was launched without that unrelated internal restart identifier. Host daemon and configuration were untouched. Browser-extension CUA requests timed out; native CUA access to Chrome succeeded. No verdict is inferred from failed browser attempts. The inspected tab reported no browser console errors. The installed provider runtime produced unusual whitespace/code fences in later final answers; those original text parts are outside this grouping change and were not altered by it. Tool detail showed the payload supplied by that runtime, rather than claiming richer result content than it supplied.

## Automated evidence

Repository-root Turborepo runs passed the canonical projection (47), scroll anchoring (21), navigation (6), and thread interaction (133) cases: 207 tests across these suites. The thread suite includes mixed streaming-to-settled DOM identity, focus and disclosure retention. Web TypeScript passed. The runtime-provider integration suite also passed all 58 tests after updating its mixed-work disclosure interaction without removing any ordering assertion. No test expectations were weakened to suppress a production failure.

## Final Status

Rendered targeted QA passed and cleanup is clean. The first local gate passed lint/typecheck and 7,189 tests, then exposed one old integration expectation for uncollapsed mixed activity. That interaction was updated and its 58-test canonical suite passed. The user requested remaining gates run in PR CI to avoid the shared local machine queue; no local gate rerun is claimed. The first CodeRabbit review of `13f653a` completed with four findings, addressed together: namespaced mixed-entry keys, progress-tick live-tail detection, cancellation-state normalization/detail rendering, and explicit expanded reasoning-body/order assertions. Existing projection and thread suites were extended for those invariants; these remediation changes were inspected but are not claimed as locally re-tested. Greptile completed the initial review with no actionable findings. Per the revised delivery contract, current-head GitHub CI owns remediation validation; no second review round is requested or required. Sidecar image publication remains unavailable as documented above.
