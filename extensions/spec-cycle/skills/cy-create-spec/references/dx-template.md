# DX Template

Structure for `_dx.md` — the developer-experience contract that freezes the public surface before internals are designed. It shows every interface exactly as its consumer will use it, written **as if the feature already shipped**. Downstream consumers: the Stage 2 surface grill reworks it, Part II of `_spec.md` is designed to serve it, `_tests.md` derives CLI/API E2E journeys from its examples, and `packages/site` docs pages seed from it.

## Rules

- **As-if-shipped**: real names, real values, real output — never `<TBD>`, never "for example, something like". If a value is undecided, that is a surface-grill question, not a placeholder.
- **Zero internals**: no Go interfaces, no storage, no architecture, no package names. Those live in `_spec.md` Part II. This document never explains how — only what the user types and what they see.
- **Pair every example**: what the user writes → what comes back. A command without its output, a request without its response, or a YAML file without its observed effect is half an example.
- **Runtime truth**: every surface shown must be one the design will actually ship; agents are users too — `compozy__*` native tools and structured CLI output are part of the DX.
- **Vocabulary**: `docs/_memory/glossary.md` terms and the `COPY.md` register throughout.
- **Quality test**: pasted into the docs site as the feature's usage page, a new user succeeds without asking anything.

## Document Skeleton

Keep only the sections whose surface the feature has; delete the rest — an empty section is a signal the surface was not designed, not a formality to fill.

```markdown
# Developer Experience: [Feature Name]

Public-surface contract for [feature]. Companion to `_spec.md` (Part II serves
this surface) and `_tests.md` (E2E journeys use these exact invocations).

## Golden Path

The 30-second end-to-end journey — the shortest real sequence from nothing to
the feature's core value, as one runnable transcript: commands typed, files
written, output seen.

## YAML

The complete final file(s) the user authors — full, valid, commented where a
line is non-obvious. State where the file lives and what loads it.

## CLI

Per verb: the invocation, the human output, the structured output
(`--json`), and the exit code. Include the failure invocations a user will
actually hit, with their exact error output.

## HTTP / UDS API

Per route: method, path, request body, response body — as the client sees
them, with realistic values and status codes for success and each documented
failure.

## SDK / Bridges

The code a bridge or SDK consumer writes, compilable as shown, with the
value or event it receives back.

## config.toml

Every key with its default, as one pasteable block, plus one line per key on
observable effect.

## Native Tools

Per `compozy__*` tool an agent uses to operate the feature: the call
arguments and the structured result.

## Errors

The deterministic failure surface across all surfaces above: condition → the
exact message/code/shape the user or agent sees, and the action it points to.
```
