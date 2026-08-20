// Suite: command-palette ranking.
// Invariant: one pure scorer turns the daemon's versioned weights and workspace
// signals into a deterministic total order, section grammar, and ghost tail.
// Owning layer: web/src/systems/os/lib/ranking. Canonical suite: this file.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  acceptGhostCompletion,
  assembleRankingResults,
  compareRankedCandidates,
  decayFrecency,
  ghostCompletion,
  isPrunableSignal,
  matchRankingCandidate,
  rankCandidates,
  type RankingCandidate,
  type RankingSnapshot,
  type RankingWeights,
} from "..";

const weights = JSON.parse(
  readFileSync(
    resolve(process.cwd(), "../internal/cmdpalette/testdata/ranking_weights_v1.json"),
    "utf8"
  )
) as RankingWeights;

function snapshot(overrides: Partial<RankingSnapshot> = {}): RankingSnapshot {
  return {
    weights,
    usage: [],
    query_hits: [],
    pins: [],
    revision: "ps_fixture",
    ...overrides,
  };
}

function candidate(
  id: string,
  label: string,
  overrides: Partial<RankingCandidate> = {}
): RankingCandidate {
  return {
    stableKey: id,
    id,
    label,
    group: "Commands",
    available: true,
    subtype: "command",
    ...overrides,
  };
}

describe("command palette ranking", () => {
  it("matches word-boundary subsequences inside the documented band [UT-020]", () => {
    const match = matchRankingCandidate("nwt", candidate("window.tab.new", "New tab"), weights);
    expect(match?.kind).toBe("word-boundary");
    expect(match?.score).toBeGreaterThanOrEqual(weights.match_word_boundary_min);
    expect(match?.score).toBeLessThanOrEqual(weights.match_word_boundary_max);
  });

  it("folds diacritics and rejects candidates when any term drops [UT-021, UT-022]", () => {
    expect(
      matchRankingCandidate("sessao", candidate("session.open", "Sessão"), weights)
    ).not.toBeNull();
    expect(
      matchRankingCandidate("new missing", candidate("window.tab.new", "New tab"), weights)
    ).toBeNull();
  });

  it("keeps exact aliases above exact titles and applies context boost [UT-023, UT-024]", () => {
    const aliases = candidate("github", "Open source", { aliases: ["gh"] });
    const title = candidate("gh-command", "gh");
    expect(rankCandidates("gh", [title, aliases], snapshot()).map(row => row.candidate.id)).toEqual(
      ["github", "gh-command"]
    );

    const contextual = candidate("contextual", "Open", { contextual: true });
    const global = candidate("global", "Open");
    expect(rankCandidates("open", [global, contextual], snapshot())[0]?.candidate.id).toBe(
      "contextual"
    );
  });

  it("defines a transitive antisymmetric order stable across input shuffles [UT-025]", () => {
    const candidates = Array.from({ length: 18 }, (_, index) =>
      candidate(`command-${index}`, `Command ${String.fromCharCode(65 + index)}`, {
        group: index % 3 === 0 ? "Apps" : index % 3 === 1 ? "Commands" : "Settings",
        contextual: index % 4 === 0,
      })
    );
    const signals = snapshot({
      usage: candidates.map((entry, index) => ({
        command_id: entry.id,
        weight: index / 3,
        last_used_at: index,
      })),
    });
    const baseline = rankCandidates("command", candidates, signals);
    const reversed = rankCandidates("command", [...candidates].reverse(), signals);
    expect(reversed.map(row => row.candidate.id)).toEqual(baseline.map(row => row.candidate.id));
    for (const left of baseline) {
      for (const right of baseline) {
        const forward = Math.sign(compareRankedCandidates(left, right, weights));
        const backward = Math.sign(compareRankedCandidates(right, left, weights));
        if (left.candidate.stableKey === right.candidate.stableKey) expect(forward).toBe(0);
        else expect(forward).toBe(-backward);
      }
    }
    for (const left of baseline) {
      for (const middle of baseline) {
        for (const right of baseline) {
          if (
            compareRankedCandidates(left, middle, weights) <= 0 &&
            compareRankedCandidates(middle, right, weights) <= 0
          ) {
            expect(compareRankedCandidates(left, right, weights)).toBeLessThanOrEqual(0);
          }
        }
      }
    }
  });

  it("treats regex metacharacters as literal input [UT-026]", () => {
    const ranked = rankCandidates("(*\\", [candidate("literal", "Run (*\\ safely")], snapshot());
    expect(ranked).toHaveLength(1);
  });

  it("decays frecency exactly and flags only old weak signals [UT-027, UT-028]", () => {
    const now = Date.UTC(2026, 7, 19, 12);
    expect(
      decayFrecency(
        10,
        now - weights.frecency_half_life_days * 86_400_000,
        now,
        weights.frecency_half_life_days
      )
    ).toBe(5);
    expect(
      isPrunableSignal(
        weights.prune_threshold / 2,
        now - weights.prune_after_days * 86_400_000,
        now,
        weights
      )
    ).toBe(true);
    const sameText = [candidate("used", "Open"), candidate("unused", "Open")];
    expect(
      rankCandidates(
        "open",
        sameText,
        snapshot({ usage: [{ command_id: "used", weight: 10, last_used_at: now }] })
      )[0]?.candidate.id
    ).toBe("used");
  });

  it("lets the strongest learned query association dominate deterministically [UT-029]", () => {
    const rows = [candidate("x", "GH tool"), candidate("y", "GH tool")];
    const ranked = rankCandidates(
      "gh",
      rows,
      snapshot({
        query_hits: [
          { query: "gh", command_id: "x", weight: 1 },
          { query: "gh", command_id: "y", weight: 3 },
        ],
      })
    );
    expect(ranked.map(row => row.candidate.id)).toEqual(["y", "x"]);
  });

  it("assembles empty and typed sections with caps and promotion floors [UT-030]", () => {
    const rows = [
      candidate("pinned", "Pinned command"),
      candidate("recent", "Recent command"),
      candidate("view-context", "Context view", { group: "Views", contextual: true }),
      candidate("view-curated", "Curated view", { group: "Views" }),
      candidate("app", "Curated app", { group: "Apps" }),
      candidate("tab", "Tab target", { group: "Tabs", subtype: "tab" }),
    ];
    const signals = snapshot({
      pins: ["pinned"],
      usage: [{ command_id: "recent", weight: 1, last_used_at: 10 }],
      weights: { ...weights, entity_section_visible_cap: 1 },
    });
    const emptySections = assembleRankingResults("", rows, signals).sections;
    expect(emptySections.map(section => section.title)).toEqual([
      "Pinned",
      "Recents",
      "Views",
      "Tabs",
      "Apps",
    ]);
    expect(emptySections.find(section => section.title === "Views")).toMatchObject({
      total: 2,
      candidates: [{ candidate: { id: "view-context" } }],
    });
    expect(assembleRankingResults("target", rows, signals).sections).toEqual([]);
    expect(assembleRankingResults("target", rows, signals).fallback).toBe(true);
    expect(
      assembleRankingResults("tab target", rows, signals).sections.map(section => section.title)
    ).toEqual(["Tabs"]);
  });

  it("assembles the agent fallback at the served weak-match boundary [UT-140]", () => {
    const row = candidate("ask", "Ask");
    const boundaryWeights: RankingWeights = {
      ...weights,
      fallback_weak_match_threshold: 120,
      match_exact: 120,
      promotion_command_floor: 0,
    };
    const boundary = assembleRankingResults("Ask", [row], snapshot({ weights: boundaryWeights }));
    expect(boundary.sections[0]?.candidates[0]?.candidate.id).toBe("ask");
    expect(boundary.fallback).toBe(true);

    const below = assembleRankingResults(
      "Ask",
      [row],
      snapshot({ weights: { ...boundaryWeights, match_exact: 119 } })
    );
    expect(below.sections).toEqual([]);
    expect(below.fallback).toBe(true);

    expect(assembleRankingResults("missing", [row], snapshot()).fallback).toBe(true);
    expect(assembleRankingResults("", [row], snapshot()).fallback).toBe(false);
  });

  it("treats a match below the promotion floor as fallback", () => {
    const result = assembleRankingResults("xp", [candidate("x", "xylophone")], snapshot());
    expect(result.sections).toEqual([]);
    expect(result.fallback).toBe(true);
  });

  it("returns and accepts an unambiguous casing-preserving ghost only at end of input [UT-118, UT-119]", () => {
    const ranked = rankCandidates("Ne", [candidate("new", "New tab")], snapshot());
    const tail = ghostCompletion("Ne", ranked, weights);
    expect(tail).toBe("w tab");
    expect(acceptGhostCompletion("Ne", tail, 2, 2)).toBe("New tab");
    expect(acceptGhostCompletion("Ne", tail, 1, 1)).toBeNull();
    expect(
      ghostCompletion(
        "Ne",
        rankCandidates("Ne", [candidate("new", "New tab"), candidate("news", "News")], snapshot()),
        weights
      )
    ).toBeNull();
  });

  it("refuses a diacritic-equivalent ghost that would corrupt the raw query", () => {
    const ranked = rankCandidates("Né", [candidate("new", "New tab")], snapshot());
    expect(ghostCompletion("Né", ranked, weights)).toBeNull();
  });
});
