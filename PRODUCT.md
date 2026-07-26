# Product

## Register

product

> Default register is `product` — the runtime UI in `web/` + `packages/ui`, where design SERVES the product. AGH also ships a brand surface (the agh.network marketing + Fumadocs site in `packages/site`, where design IS the product). When a task targets that site, override the register to `brand` for that task. PRODUCT.md keeps `product` as the standing default.

## Users

**Primary — People running agent work.** Anyone who delegates real work to AI agents — founders, product people, writers, analysts, and developers — and needs that work to be durable, visible, and steerable rather than trapped in throwaway terminal tabs. They run ACP-compatible agent CLIs (Claude Code, OpenClaw, Hermes) through AGH without needing to know the protocol underneath. Their context: a personal machine running a background daemon, often several agent sessions at once, where the job is to start work, see what agents are doing, step in when something needs them, and trust what finished. Design for the least technical person in this group; never require terminal literacy to understand state.

**Secondary — Agent/runtime developers.** Engineers extending AGH against daemon contracts: extensions, hooks, skills, capabilities, bridges, and SDKs. They need the UI to expose the same structured surfaces the daemon exposes, not a UI-only shortcut.

**Also first-class — Agents themselves.** Agents operate AGH through structured surfaces (CLI `-o json`, HTTP/SSE, UDS, tool registry). The UI is one view over state that agents can equally drive; it is never the only path to a capability.

## Product Purpose

AGH is a local-first agent operating system. One Go daemon hosts durable, inspectable agent sessions; one shared surface (CLI, HTTP/SSE, UDS, and this web UI) serves humans and agents over the same daemon-owned state; and `agh-network/v0` lets sessions discover peers, delegate work, exchange capabilities, and close the loop with receipts.

The runtime UI's job is to make agent work legible and controllable at a glance: what is running, what needs you, what finished, and what it produced. Depth — events, tools, memory, network traffic — stays one step away for whoever wants it, and no one is asked to decode runtime internals to understand their own work. Success looks like: a person supervises several concurrent agents, understands the state of each in seconds, and acts on it (resume, approve, inspect, route) without ever being shown a control or metric the runtime does not actually support.

## Brand Personality

People-first, plain-spoken, calm-confident. (COPY.md §5 is the authority; this is the design-facing distillation.)

- **Calm, not cute.** Plain, not vague. Confident, not inflated.
- Everyday words carry the message; the mechanism stays one step away as proof, never as an entry fee.
- Prefer nouns and outcomes over adjectives; lead with what happened, then how, then the evidence.
- The product is usually the subject — "AGH keeps...", "Agents continue..." — not "we" or "you" as a sales hook.
- No emoji, no exclamation marks, no fake urgency, no fabricated stats or maturity claims.
- Emotional goal: the steadiness of a well-run workplace. The person should feel informed and in control — never marketed to, and never required to be an engineer to understand what is happening.

## Anti-references

This must NOT look like:

- **Operator cockpits.** AGH's own early direction, now retired: control-room density, walls of metrics, colored badges on every row, mono ids in primary positions, panels competing for attention. Expert texture is not a virtue; it is a failure of hierarchy.
- **Generic SaaS dashboards.** Hero-metric templates (big number + gradient accent), identical icon-heading-text card grids, decorative glassmorphism. (Glass is banned as content decoration; the tokenized OS-shell chrome glass — menubar, dock, rail, shell popovers, window frames — is sanctioned per DESIGN.md §5, never on window content.)
- **Chat skins that hide the work.** Approachable never means opaque: sessions are durable, inspectable objects with real state, not an ephemeral bubble stream. Friendly and truthful, not friendly instead of truthful.
- **Hype copy.** `AI-powered`, `revolutionary`, `next-generation`, `supercharge`, `unleash`, `seamless`, `10x`, `cutting-edge` — banned per COPY.md §6.
- **Plausible-but-untrue UI.** Controls, metrics, or states the daemon does not actually support. When a mockup conflicts with daemon truth, daemon wins.
- **Decorative depth.** Freehand drop shadows, sketchy/hand-drawn SVG, stripe backgrounds, gradient text, side-stripe borders, over-rounded cards (32px+), eyebrow-on-every-section scaffolding. Depth comes only from the exported `--shadow-*` / hairline tokens.

## Design Principles

1. **Truthful UI over plausible UI.** Render only what the runtime supports. Daemon state is the source of truth; never invent controls or metrics to fill a layout.
2. **Approachable first, deep on demand.** Design the default read of every screen for someone meeting agent work for the first time: few elements, plain words, obvious next action. Reveal mechanism progressively — inspection surfaces carry the detail; the default view never pays for it.
3. **Extensible and agent-manageable by default.** Anything a human can do in the UI, an agent can do through CLI/HTTP/UDS over the same state. A UI-only capability is an incomplete feature.
4. **Show shipped behavior, not aspiration.** Every visible claim, control, and label maps to merged runtime mechanisms — commands, protocol objects, events, artifacts.
5. **Calm confidence through restraint.** Clarity over decoration. Hierarchy from type scale, weight, spacing, and the neutral ramp — color only where it carries live state. No hacks, no theater.

## Accessibility & Inclusion

Target: **WCAG 2.2 AA**, measured against the warm-dark surface ramp.

- Body text ≥ 4.5:1, large text (≥18px or bold ≥14px) ≥ 3:1; placeholder text held to 4.5:1, not the muted-gray default. DESIGN.md's contrast tokens are authoritative.
- Visible, tokenized focus states and complete keyboard paths for every interactive surface.
- `prefers-reduced-motion: reduce` alternative for every animation (crossfade or instant).
- The signal palette (accent = action, success, danger, warning, info) is never the sole carrier of state — pair color with text, icon, or shape so color-blind and high-contrast users read the same meaning.
- Language is part of access: primary surfaces use everyday words, and runtime jargon appears only where precision earns it — defined nearby or one step deeper.
