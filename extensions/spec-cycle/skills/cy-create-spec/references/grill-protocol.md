# Grill Protocol

How every `cy-create-spec` grill runs — product, surface, and technical alike. Interview the user relentlessly until shared understanding: every load-bearing decision resolved, not a question budget met.

## The design tree and the frontier

- Map the stage into a **design tree**: every decision branches into the decisions that hang off it.
- The **frontier** is every decision whose prerequisites are already settled — the questions you can ask _now_ without guessing at answers you haven't heard yet.
- Work the tree in **rounds**: ask the whole frontier in one round, then wait for the user's answers. Each round reshapes the tree — settled decisions push the frontier outward and unblock questions that depended on them. Recompute and ask the next round.
- A question whose answer depends on another question still open in this round belongs to a _later_ round, not this one.

## Round format

A round lives on one surface. When every question in the round is closed-choice and the runtime's interactive question tool can carry them all without loss — bodies, option labels, and recommendations intact (in practice: ≤4 short multiple-choice questions) — ask the round through the tool, recommended option first and marked as recommended, with its one-line why. Otherwise present the round as one plain message and stop generating until the user answers, formatting each question:

```
❓ **Q1 — <question title>**: <question body; labeled choices when the options can be predetermined, with a fallback "Other — describe">

➡️ <your recommended answer and the one-line why>
```

Leading with a recommendation lets the user react to a position instead of facing a blank menu — on either surface.

## Conduct

- **Facts are your job, never the user's.** When a frontier question needs a fact from the codebase, docs, or environment, dispatch a read-only subagent — never ask the user what you can look up. Don't block the round on it: only the questions downstream of that fact wait for the report; ask the rest of the frontier now.
- **Decisions are the user's.** Put each one to them and wait — intent, priorities, risk appetite, trade-offs.
- **Chase vague answers**: "it depends" gets "on what?", "probably" gets pinned down. A load-bearing branch left fuzzy resurfaces as rework after the spec ships.
- **Stress-test with concrete scenarios**: invent edge scenarios that probe the boundaries between concepts and force precision ("two runs claim the same worktree at once — what does the second one see?").
- **Cross-reference with code**: when the user states how something works, check whether the code agrees; surface every contradiction ("the code stops the session on removal, but you just said sessions survive — which is right?").

## Vocabulary discipline

- Challenge terms against `docs/_memory/glossary.md` the moment they appear: when a term conflicts with the glossary, call it out ("the glossary defines _capability_ as X — you seem to mean Y, which is it?"); when a term is fuzzy or overloaded, propose a precise canonical term.
- When the session coins a term or changes a meaning, propose the matching `glossary.md` edit to the user before the stage closes.

## ADR capture

Record an ADR the moment a significant decision crystallizes — never batch them to the end of a stage. A decision earns an ADR when all three hold:

1. **Hard to reverse** — changing it later costs something real.
2. **Surprising without context** — a future reader would ask "why this way?".
3. **A real trade-off** — genuine alternatives existed and one was chosen for specific reasons.

Mechanics: read `references/adr-template.md`, take the next zero-padded 3-digit number from `.compozy/tasks/<slug>/adrs/`, fill the template (chosen direction as Decision, weighed alternatives with trade-offs, outcomes as Consequences; Status "Accepted", Date today), write `adrs/adr-NNN.md`. Every spec records at least one ADR — the primary technical approach — even when no other decision passes the test.

## Stage lenses

- **Product (Stage 1)**: WHAT, WHY, WHO. Purpose, personas, behavior, business rules, scope boundaries, constraints. Translate implementation-sounding topics into the user-experience question behind them; databases, APIs, frameworks, and architecture wait for Stage 2.
- **Surface (Stage 2, first)**: the drafted `_dx.md`/`_uiux.md` are the question — grill naming, invocation shape, YAML feel, output ergonomics, golden-path friction, failure copy, per-surface states. Rework the drafts between rounds so the user always reacts to a concrete artifact.
- **Technical (Stage 2, second)**: HOW, WHERE, WHICH. Architecture and component boundaries, data models and storage, integration points, testing strategy, performance. The frozen surface is a constraint here, not a topic — internals serve it.

## Done

A grill ends when its frontier is empty: every branch confirmed or explicitly parked in Open Questions with the user's consent, nothing left silently assumed. The question count is an output of the tree, never a budget. Do not write a stage's artifacts before its frontier is empty.
