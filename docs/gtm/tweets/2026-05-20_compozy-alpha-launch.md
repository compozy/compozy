# Compozy Alpha — Launch Thread

- **Date:** 2026-05-20
- **Author:** Pedro Nauck (@pedronauck)
- **Voice:** Pedro X/Twitter (English, ≤280 chars per tweet)
- **Visual assets:** `/Users/pedronauck/Dev/compozy/_graphic/compozy/video_launch/final/video_frame_0{1..7}.png`

## 1/7 — Launch

> **Image:** `video_frame_01.png` (hero — "An open workplace for AI agents · Agent Network Protocol · AgentOS Runtime")

```
just shipped Compozy 🚀

two products in one Go binary. a complete agent runtime with sessions, memory, autonomy, tools, automation, bridges, extensions. and an open network protocol so agents actually coordinate work across machines

going to walk through both
```

## 2/7 — Network

> **Image:** `video_frame_03.png` (Built-in network — agents talking each other · DISCOVER → DELEGATE → RECEIPT)

```
compozy-network/v0 is the layer where agents on different machines actually delegate work

greet, whois, say with a work_id, receipt, trace. one peer accepts responsibility, the other streams progress

MCP connects agents to tools. Compozy Network connects agents to agents.
```

## 3/7 — Runtime

> **Image:** `video_frame_02.png` (AgentOS Runtime — Your agents under control · session list with RUNNING / IDLE / WAITING / COMPLETED)

```
the runtime ships as a single Go daemon that runs 20+ agent CLIs as durable sessions that survive crashes and closed terminals

event ledger at ~/.compozy/sessions/<id>/events.db. resume or replay through CLI, HTTP, UDS, or the web UI. one SQLite under all of it
```

## 4/7 — Memory

> **Image:** `video_frame_04.png` (Advanced Memory — layers that compound)

```
memory in Compozy is typed markdown across global and workspace scopes. user, feedback, project, reference

compozy memory write/search/decisions/dream. consolidation only fires when the signal warrants it

memory you can read and version, not just vector-search
```

## 5/7 — Bridges

> **Image:** `video_frame_05.png` (Bridges — from anywhere into a session · Slack / Discord / Telegram)

```
bridges route slack, discord, telegram and more straight into a session. extension-owned adapters verify, normalize, and write to the same session events

compozy bridge create/routes/test-delivery

your agent now has a slack inbox
```

## 6/7 — Extensibility

> **Image:** `video_frame_06.png` (Every layer can be extensible — modular blocks)

```
every layer in compozy is extensible. agents, skills, hooks, tools, MCP, bridges, capabilities, even subprocess services. all packaged as extensions with explicit precedence rules

compozy extension install/enable, compozy skill create. every layer is code you own, swap, or share
```

## 7/7 — CTA

> **Image:** `video_frame_07.png` (compozy.com — logo + URL)

```
this is alpha. everything in this thread actually ships today

curl -fsSL https://compozy.com/install.sh | sh
compozy daemon start

compozy.com
```

---

## Voice gate notes

All seven tweets pass Pedro voice gate (`x-pedro-voice` skill) with score below the blocking threshold of 3.

- **Em-dashes:** 0 across the thread (em-dash overuse is the most common AI-tell).
- **AI vocabulary:** 0 (no `additionally`, `delve`, `intricate`, `landscape`, `leverage`, `robust`, `seamless`).
- **Hedging:** 0 (no `perhaps`, `arguably`, `might`, `could`, `somewhat`).
- **Banned openers:** 0 (no `let's`, `imagine`, `in today's`, `here's the thing`, `the truth is`).
- **Approved opener:** tweet 1 uses `just shipped` from the approvelist (`^just\s+(spent|finished|shipped|deployed|debugged)\b`).
- **Banned marketing phrases:** 0 (no `AI-powered`, `revolutionary`, `next-generation`, `supercharge`, `unleash`, `seamless`, `10x`).
- **No `we` / `our`:** Compozy and the runtime/network are the subjects throughout.
- **Emojis:** one 🚀 in tweet 1, in expected launch context per the semantic emoji table.
- **Positive markers:** `actually` (tweets 1, 2, 7), `finally` not used in current draft, `ships` / `shipped` / `spins up` / `runs` as Pedro action verbs.

### Borderline patterns retained intentionally

- `every layer is code you own, swap, or share` (tweet 6) — three-verb closer. Concrete action verbs, not flowery adjective parallelism. Acceptable Pedro pattern.
- `verify, normalize, and write` (tweet 5) — three-verb technical enumeration. Same reasoning.
- `memory you can read and version, not just vector-search` (tweet 4) — uses `not just` contrast. Mild negative-parallelism but single contrast, not the formulaic `not just X, it's Y` AI pattern.

## Source briefs

Each topical tweet was grounded in a parallel Explore subagent brief that read the actual code, docs, and launch blog. The launch blog at `packages/site/content/blog/posts/introducing-compozy-the-first-agent-network-protocol.mdx` is the canonical source for the positioning, and `COPY.md` (repo root) governs the verbal grammar.

- **Runtime brief:** session durability, event ledger path, daemon singleton, CLI/HTTP/UDS/Web UI parity.
- **Network brief:** six message kinds (greet/whois/say/capability/receipt/trace), work_id lifecycle, commit-first in-process delivery, the MCP kicker line.
- **Memory brief:** typed indexes (user/feedback/project/reference), global vs workspace scopes, dream consolidation gating, file-backed markdown.
- **Bridges brief:** Slack/Discord/Telegram/GitHub/Linear, extension-owned adapters, durable route records, `compozy bridge create/routes/test-delivery`.
- **Extensibility brief:** 16 hook event families, five-layer skill precedence, `extension.toml`, `AGENT.md`, marketplace provenance.

## Suggested ordering for posting

1. Schedule the entire thread in one drafting session (X composer) so each tweet's image attaches reliably.
2. Confirm `compozy.com/install.sh` is reachable before tweet 7 ships.
3. Pin tweet 1 to Pedro's profile after the thread posts.
4. Quote-tweet from the @compozy account (if active) within the first hour to amplify.
