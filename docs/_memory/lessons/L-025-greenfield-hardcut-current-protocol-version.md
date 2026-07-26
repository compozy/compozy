# L-025 — On greenfield, hard-cut the current protocol version — don't bump to a new one

**Class:** Project posture / RFC discipline
**Date discovered:** 2026-05-13 (workspace-isolation hard-cut follow-up; PR #145 on branch `workspace-fix`)
**Evidence sources:** Commits `76afabb8`, `6fb41e8d`, `de247cc4` on branch `workspace-fix`; ledger
`.codex/ledger/2026-05-12-MEMORY-workspace-isolation-hard-cut.md` lines 27, 30–31, 143–151; RFC
audit across `docs/rfcs/`, `internal/network/`, `packages/site/content/protocol/`.

## Context

Commit `76afabb8 feat: hard cut workspace isolation` performed a greenfield, zero-legacy rewrite of
the AGH Network wire and routing grammar so every envelope carries `workspace_id` and every
conversation identity is workspace-qualified. The runtime
contract _changed_, but no public user existed on the previous shape.

In the follow-up commit `6fb41e8d docs: add workspace-qualified network v2 RFC`, the agent
interpreted the wire change as a major protocol version bump and authored
`docs/rfcs/006_agh-network-v2.md` plus rebranded code, docs, copy, blog post, landing page, slides,
and tests to `agh-network/v2`. The mistake was caught immediately. Commit `de247cc4 fix: keep
network protocol at v0` reverted the version identifier across 92 files, deleted RFC 006, and
restored `agh-network/v0` as the current contract — while keeping the workspace-isolation hard cut
intact. RFC 004 remained the future `v1` auth/proofs/trust profile.

A four-surface audit (RFCs, Go implementation, docs site, institutional memory) on
`2026-05-13` confirmed the walk-back was clean. The remaining gap was a lesson capturing _why_
the v2 path was wrong.

## Root cause

Protocol version identifiers were treated as **feature labels** ("this version has workspace
isolation") instead of **wire-compat tokens** ("peers speaking version X interoperate"). On a
greenfield branch with zero published peers, no consumer needed an interop boundary. The wire
shape simply changed, and the current version (`v0`) needed to describe the new shape.

Versioning a not-yet-shipped behavior introduces two artifacts that must be maintained forever: a
"historical v0" RFC that no one ever ran, and a "v2" RFC that pretends to supersede something that
never existed. Worse, it consumed the next version slot reserved for genuine wire-compat work
(`v1` future trust/auth/proofs in RFC 004) and shoved that work to `v3`, breaking the linear
RFC narrative across `003`, `004`, `005`.

## Rule

> Protocol version identifiers are wire-compat tokens, not feature labels. On greenfield, when the
> current protocol has no published peers, changes to its wire shape hard-cut the **current**
> version in place; do not invent a new version identifier. Reserve new version slots for genuine
> interoperability boundaries with already-shipped peers, or for clearly future protocols that the
> runtime does not yet implement.

This is the protocol-grammar corollary of [L-006](L-006-greenfield-delete-not-adapt.md): greenfield
zero-legacy means _delete_, not _adapt_. Here it means _redefine the current version_, not
_invent a new one_.

## Operationalization

- Before adding a new version identifier to any RFC, ask: **does a published peer speak the old
  version?** If no, hard-cut the current version's RFC, code constants, routing fields, envelope
  schema, tests, docs, copy, and landing/slides in one change.
- The hard cut sweeps every surface in one commit. Renames of protocol version strings touch:
  - RFC files under `docs/rfcs/` (current contract + any historical archive marker).
  - Go constants under `internal/network/` (for example, `ProtocolV0`).
  - Generated artifacts: `openapi/agh.json`, `web/src/generated/agh-openapi.d.ts`, runtime/API
    reference pages.
  - Site content: `packages/site/content/protocol/**`, `packages/site/content/runtime/core/network/**`,
    blog posts, landing components, `packages/slides/slides/agh-network/`.
  - Copy: `COPY.md`, `docs/_memory/glossary.md`.
  - Tests: any fixture pinning the protocol literal, including hard-cut truth tests.
- Future versions (`v1`, `v2`, ...) describe work the runtime does **not yet implement** and which
  introduces a real interop boundary (auth, proofs, trust profiles, new required fields, removed
  kinds). They are RFCs, not runtime claims.
- Truth tests should encode the rule: the hard-cut test suite asserts both
  "current runtime contract uses `agh-network/v0`" and a regex blacklist for forbidden alternative
  version strings, so a future agent cannot silently rebrand.

## Anti-pattern

- Bumping `v0 → v2` because the wire grammar changed, even though no peer ran the old grammar.
- Adding an RFC that supersedes a never-shipped protocol shape.
- Consuming the next version slot reserved for future trust/auth work to label current behavior.
- Marking the previous RFC "historical/superseded" when in fact the runtime moved straight from
  unshipped-A to unshipped-B.
- Treating version identifiers as marketing or release labels ("v2 is the workspace-aware version")
  instead of as interop tokens with peers.

## Source

- Commit `76afabb8 feat: hard cut workspace isolation` — the greenfield wire rewrite.
- Commit `6fb41e8d docs: add workspace-qualified network v2 RFC` — the erroneous v2 bump (later
  reverted).
- Commit `de247cc4 fix: keep network protocol at v0` — the 92-file walk-back that restored v0 and
  deleted `docs/rfcs/006_agh-network-v2.md`.
- `.codex/ledger/2026-05-12-MEMORY-workspace-isolation-hard-cut.md` lines 27, 30–31, 143–151 —
  the canonical forensic record of the version-bump mistake and correction.
- `docs/rfcs/003_agh-network-v0.md` — the current contract carrying the workspace-qualified hard
  cut.
- `docs/rfcs/004_agh-network-v1.md` — future auth/proofs/trust profile, depends on v0; the slot
  the erroneous v2 was poaching.
- `internal/network/envelope.go` (`ProtocolV0`) — the single-source-of-truth protocol constant; delivery
  is commit-first and in-process under ADR-010.
- `packages/site/lib/__tests__/protocol-rfc-hard-cut.test.ts` — the truth test that now asserts
  `agh-network/v0` is current and forbids `agh-network/v2` / `ProtocolV2` / `006_agh-network-v2`.
- [L-006](L-006-greenfield-delete-not-adapt.md) — the broader posture this lesson specializes to
  protocol versioning.
