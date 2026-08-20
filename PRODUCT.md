# Product

## Register

product

> Default register is `product` — the CompozyOS runtime UI in `web/` + `packages/ui`, where design SERVES the product. CompozyOS also ships a brand surface at `compozy.com` through the Fumadocs site in `packages/site`, where design IS the product. When a task targets that site, override the register to `brand` for that task. PRODUCT.md keeps `product` as the standing default.

> A third register, `plain`, overrides `product` on first-run, onboarding, empty states, approvals, and notifications. Its rule: zero runtime nouns in the primary line. The canonical term stays reachable one step deeper — tooltip, detail view, inspector, or linked reference — so plain never means less true. See Plain-language surface register below for the full depth ladder.

## Users

**Primary — Developers and technical operators.** Engineers, technical founders, and operators who delegate real work to AI agents and need that work durable, visible, and steerable rather than trapped in throwaway terminal tabs. They run ACP-compatible agent CLIs (Claude Code, OpenClaw, Hermes) through CompozyOS without needing to know the protocol underneath. Their context: a personal machine running a background daemon, often several agent sessions at once, where the job is to start work, see what agents are doing, step in when something needs them, and trust what finished. Design so the UI never requires terminal literacy to read state. That obligation is met now, and it is staged: **register is present tense** — every end-user surface reads plainly today — while **audience is future-framed**, because install, setup, and configuration still run through a terminal and `config.toml`. The full no-terminal workflow is served when that path ships, not before.

**Emerging — People who don't write code.** Their job is to understand and steer work an agent is doing for them: see what is running, step in when something needs them, and trust what finished. Their context has no terminal and no config file. Today they are served by register, not by workflow — every end-user surface is written in plain words now, while the path that would get them to those surfaces without a terminal is planned, not shipped. Design for their default read; do not describe or render a no-terminal experience as current behavior.

**Secondary — Agent/runtime developers.** Engineers extending CompozyOS against daemon contracts: extensions, hooks, skills, capabilities, bridges, and SDKs. They need the UI to expose the same structured surfaces the daemon exposes, not a UI-only shortcut.

**Also first-class — Agents themselves.** Agents operate CompozyOS through structured surfaces (CLI `-o json`, HTTP/SSE, UDS, tool registry). The UI is one view over state that agents can equally drive; it is never the only path to a capability.

## Product Purpose

CompozyOS is one complete environment to create, automate, and supervise agent work, without scripts, plugin chains, or orchestration frameworks. Loops, triggers, memory, permissions, automation, and supervision come built in rather than assembled. Why it holds: one runtime, one state model; loops, approvals, and memory are core objects, not plugins. Web, CLI, HTTP/SSE, UDS, and native tools let people and agents operate that same system. Compozy Network adds peer discovery, capability exchange, delegation, and receipts as one subsystem of the OS.

The runtime UI's job is to make agent work legible and controllable at a glance: what is running, what needs you, what finished, and what it produced. Depth — events, tools, memory, network traffic — stays one step away for whoever wants it, and no one is asked to decode runtime internals to understand their own work. Success looks like: a person supervises several concurrent agents, understands the state of each in seconds, and acts on it (resume, approve, inspect, route) without ever being shown a control or metric the runtime does not actually support.

## Brand Personality

People-first, plain-spoken, calm-confident. (COPY.md §5 is the authority; this is the design-facing distillation.)

- **Calm, not cute.** Plain, not vague. Confident, not inflated.
- Everyday words carry the message; the mechanism stays one step away as proof, never as an entry fee.
- Prefer nouns and outcomes over adjectives; lead with what happened, then how, then the evidence.
- The product is usually the subject — "CompozyOS keeps...", "Agents continue..." — not "we" or "you" as a sales hook.
- No emoji, no exclamation marks, no fake urgency, no fabricated stats or maturity claims.
- Emotional goal: the steadiness of a well-run workplace. The person should feel informed and in control — never marketed to, and never required to be an engineer to understand what is happening.

## Anti-references

This must NOT look like:

- **Operator cockpits.** CompozyOS's own early direction, now retired: control-room density, walls of metrics, colored badges on every row, mono ids in primary positions, panels competing for attention. Expert texture is not a virtue; it is a failure of hierarchy.
- **Generic SaaS dashboards.** Hero-metric templates (big number + gradient accent), identical icon-heading-text card grids, decorative glassmorphism. (Glass is banned as content decoration; the tokenized OS-shell chrome glass — menubar, dock, rail, shell popovers, window frames — is sanctioned per DESIGN.md §5, never on window content.)
- **Chat skins that hide the work.** Approachable never means opaque: sessions are durable, inspectable objects with real state, not an ephemeral bubble stream. Friendly and truthful, not friendly instead of truthful.
- **Hype copy.** `AI-powered`, `revolutionary`, `next-generation`, `supercharge`, `unleash`, `seamless`, `10x`, `cutting-edge` — banned per COPY.md §6.
- **Plausible-but-untrue UI.** Controls, metrics, or states the daemon does not actually support. When a mockup conflicts with daemon truth, daemon wins.
- **Decorative depth.** Freehand drop shadows, sketchy/hand-drawn SVG, stripe backgrounds, gradient text, side-stripe borders, over-rounded cards (32px+), eyebrow-on-every-section scaffolding. Depth comes only from the exported `--shadow-*` / hairline tokens.

**Guardrail — friendly never means hiding the machine.** Plain language changes what a surface says first, never what it lets you see. Sessions stay durable, inspectable objects with real state; the canonical term, the event trail, and the raw detail stay reachable one step deeper. A surface that reads plainly by dropping the truth has failed both rules, not passed one.

## Design Principles

1. **Truthful UI over plausible UI.** Render only what the runtime supports. Daemon state is the source of truth; never invent controls or metrics to fill a layout.
2. **Approachable first, deep on demand.** Design the default read of every screen for someone meeting agent work for the first time: few elements, plain words, obvious next action. Reveal mechanism progressively — inspection surfaces carry the detail; the default view never pays for it.
3. **Extensible and agent-manageable by default.** Anything a human can do in the UI, an agent can do through CLI/HTTP/UDS over the same state. A UI-only capability is an incomplete feature.
4. **Show shipped behavior, not aspiration.** Every visible claim, control, and label maps to merged runtime mechanisms — commands, protocol objects, events, artifacts.
5. **Calm confidence through restraint.** Clarity over decoration. Hierarchy from type scale, weight, spacing, and the neutral ramp — color only where it carries live state. No hacks, no theater.
6. **Plain language is a feature.** Language is part of access, not a courtesy laid over it. Primary surfaces use everyday words; runtime jargon appears only where precision earns it, defined nearby or one step deeper. A person should never have to learn the runtime's vocabulary to understand their own work.

## Plain-language surface register

How deep the vocabulary may go, by surface. Depth is a ceiling, not a target — a surface allowed full vocabulary still reads plainly where plain words are true.

| Surface                                                       | Allowed vocabulary depth                                                                           |
| ------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| First-run, onboarding, empty states, approvals, notifications | No runtime nouns. Everyday words carry the primary line; the canonical term stays one step deeper. |
| Session and task default views                                | Runtime nouns allowed, defined on first use (plain label + one clause of gloss).                   |
| Inspector, events, settings advanced                          | Full runtime vocabulary. This is where precision earns its cost.                                   |
| CLI, API, config, reference docs                              | Canonical vocabulary only. No aliases, no glosses standing in for the real name.                   |

Surface aliases (the UI label that differs from the canonical noun) are governed by `COPY.md` §6 and reserved in `docs/_memory/glossary.md`. An alias is a label, never a rename.

## Accessibility & Inclusion

Target: **WCAG 2.2 AA**, measured against the warm-dark surface ramp.

- Body text ≥ 4.5:1, large text (≥18px or bold ≥14px) ≥ 3:1; placeholder text held to 4.5:1, not the muted-gray default. DESIGN.md's contrast tokens are authoritative.
- Visible, tokenized focus states and complete keyboard paths for every interactive surface.
- `prefers-reduced-motion: reduce` alternative for every animation (crossfade or instant).
- The signal palette (accent = action, success, danger, warning, info) is never the sole carrier of state — pair color with text, icon, or shape so color-blind and high-contrast users read the same meaning.
- Language is part of access — see Design Principle 6 and the Plain-language surface register above for the vocabulary ceiling each surface carries.
