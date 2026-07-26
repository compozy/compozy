---
name: cy-web-docs-impact
description: >-
  Audits a backend AGH change for downstream impact on web/, packages/site,
  agent-operable CLI/HTTP/UDS surfaces, extensibility surfaces, and config.toml.
  Adds explicit impact subitems to backend tasks, even when the conclusion is no
  impact. Use when drafting tasks or reviewing TechSpec coverage for changes
  touching contracts, handlers, CLI verbs, config, extensions, hooks, skills,
  tools, resources, bundles, registries, bridges, MCP, or agent workflows. Do
  not use for purely internal refactors with no public, agent, config, docs, or
  extensibility surface.
trigger: explicit
argument-hint: "[task-or-spec-path]"
---

# Web/Docs and Agent Surface Impact

Pedro asks "não é preciso mudar nada na UI do web/ ... e nem melhorar nada no packages/site?" on every backend task. The same structural question applies to agent manageability, extensibility, and config lifecycle. Every backend task carries explicit impact subitems listing affected web routes, components, hooks, MSW fixtures, doc pages, generated TS types, CLI/HTTP/UDS surfaces, extension points, and `config.toml` keys — even when the final answer is "none".

## Required Inputs

- **task-or-spec-path** (optional): path to a single task file (`.compozy/tasks/<slug>/task_NN.md`) OR an entire `_techspec.md` to audit. When omitted, audit the most recently modified `_tasks.md`/`_techspec.md`.

## Procedures

**Step 1: Resolve Audit Scope**

1. Resolve the path. If a directory, audit every `task_NN.md` inside.
2. For each target file, locate the section that lists files/packages the task will touch.
3. If the task body does not enumerate touched files, refuse to audit and request a more concrete task description first.

**Step 2: Detect Surface Triggers**

1. For each task, scan touched files for backend triggers. Read `references/audit-triggers.md` for the canonical list.
2. Record the matched triggers per task. Tasks with zero triggers skip Step 3 through 5 (still get a "no impact" line in Step 6).
3. Tasks with one or more triggers continue.

**Step 3: Enumerate Web Surface**

1. For each touched contract/handler, identify affected `web/src/systems/<system>/` modules:
   - `adapters/` (HTTP + UDS clients)
   - `types.ts` (manually mirrored DTOs that should be removed in favor of generated types)
   - `query-keys.ts` and `query-options.ts` (TanStack Query)
   - `hooks/` (mutations and queries)
   - `components/` (presentational UI consuming hooks)
2. List affected `web/src/generated/` types (these regenerate from `make codegen`).
3. List affected MSW fixtures under `web/src/systems/<system>/msw/` and Storybook stories.
4. Cross-reference `web/CLAUDE.md` skill dispatch rules to determine which web skills must activate.

**Step 4: Enumerate Site Surface**

1. For each touched CLI verb, identify affected pages under `packages/site/content/runtime/cli/` (auto-generated from cobra JSON export — flag if `make cli-docs` should re-run).
2. For each touched HTTP endpoint, identify affected pages under `packages/site/content/runtime/` and `packages/site/content/protocol/`.
3. For each touched config key, identify affected `packages/site/content/runtime/configuration/` pages.
4. List affected Fumadocs MDX files and any Remotion/protocol illustration.

**Step 5: Enumerate Agent, Extensibility, and Config Surface**

1. For each user-visible or operator-visible capability, list CLI verbs, HTTP endpoints, UDS routes, structured outputs (`-o json` / `-o jsonl`), status/config discovery commands, and deterministic error contracts agents will use.
2. List extensibility surfaces affected by the change: extension manifests, hooks, skills/capabilities, tools/resources, bundles, registries, bridge SDKs, MCP sidecars, protocol docs.
3. For each touched config key or default, list structs, defaults, merge/overlay behavior, validation, examples, docs, and tests that must move with the change.
4. If a feature is intentionally not agent-operable or extensible, flag it as a design blocker unless the TechSpec explains why.

**Step 6: Append Impact Subitems**

1. For each task file, append (or update) a `### Web/Docs Impact` subsection with two sub-lists:
   - `web/`: affected systems, hooks, types, fixtures, stories. List each as a path.
   - `packages/site`: affected MDX pages, generated CLI docs, illustrations. List each as a path.
2. Append (or update) a `### Extensibility / Agent Manageability / Config Lifecycle` subsection with three sub-lists:
   - `Extensibility`: affected extension points, hooks, skills/capabilities, tools/resources, bundles, registries, bridge SDKs, MCP sidecars, protocol docs.
   - `Agent manageability`: affected CLI verbs, HTTP endpoints, UDS routes, structured output modes, status/config discovery paths, error contracts.
   - `Config lifecycle`: affected `config.toml` keys/defaults, structs, merge/overlay, validation, examples, docs, tests.
3. Append a `QA impact` line to the `### Web/Docs Impact` subsection: when the task changes user-visible behavior (UI, CLI verb, API route, config key, copy), list the `docs/qa/scenarios/*.md` ids the executor must reset to `untested` at completion (or `new scenarios — add content-addressed untested files at completion`); otherwise write `QA impact: none — no user-visible behavior change`. Flag, don't retest — retests belong to the next QA cycle.
4. If all lists are empty, write explicit no-impact evidence: `none — checked surfaces: <list>; reason: <why>`. Pedro requires the explicit "none" rather than silent omission.
5. Do not propose abstract impact ("might affect web") — only list concrete files, routes, commands, config keys, or named extension surfaces.

**Step 7: Cross-Check Codegen Co-Ship**

1. If any task touches `internal/api/contract/**`, ensure another (or the same) task explicitly lists `make codegen`, `make codegen-check`, `make web-typecheck`, and `make web-test` under its Tests section.
2. If missing, recommend adding them. Cite the contract co-ship obligation.

## Error Handling

- **Audit target lacks file enumeration:** abort and request enumeration before guessing.
- **`web/src/systems/<system>/` missing for a touched API:** flag as a missing system module; do NOT auto-create one.
- **Generated TS types path missing:** the project's codegen layout may have changed. Read `internal/codegen/openapits/generate.go` to confirm before writing impact items.
- **Conflicting impact assertions across tasks:** when two tasks both claim ownership of the same web hook, surface the conflict for the user to resolve.
- **Agent manageability missing for a user-visible capability:** block the task shape until CLI/HTTP/UDS structured paths are planned or the TechSpec gives a concrete exception.
