import { rankCandidates } from "./rank";
import type {
  RankedCandidate,
  RankingAssembly,
  RankingCandidate,
  RankingSection,
  RankingSnapshot,
  RankingSubtype,
  RankingWeights,
} from "./types";

function floorFor(subtype: RankingSubtype | undefined, weights: RankingWeights): number {
  switch (subtype) {
    case "command":
      return weights.promotion_command_floor;
    case "path":
      return weights.promotion_path_floor;
    case "tab":
      return weights.promotion_tab_floor;
    default:
      return weights.promotion_default_floor;
  }
}

function emptyRanked<T extends RankingCandidate>(candidate: T): RankedCandidate<T> {
  return {
    candidate,
    matchKind: "none",
    matchScore: 0,
    frecencyScore: 0,
    queryLearningScore: 0,
    contextScore: 0,
    score: 0,
  };
}

function makeSection<T extends RankingCandidate>(
  title: string,
  candidates: readonly RankedCandidate<T>[],
  cap: number
): RankingSection<T> | null {
  if (candidates.length === 0) return null;
  return { title, candidates: candidates.slice(0, cap), total: candidates.length };
}

function orderSections<T extends RankingCandidate>(
  grouped: ReadonlyMap<string, readonly RankedCandidate<T>[]>,
  weights: RankingWeights
): RankingSection<T>[] {
  const sections: RankingSection<T>[] = [];
  const remaining = new Set(grouped.keys());
  for (const group of weights.group_order) {
    const section = makeSection(
      group,
      grouped.get(group) ?? [],
      weights.entity_section_visible_cap
    );
    if (section !== null) sections.push(section);
    remaining.delete(group);
  }
  for (const group of [...remaining].sort((left, right) => left.localeCompare(right))) {
    const section = makeSection(
      group,
      grouped.get(group) ?? [],
      weights.entity_section_visible_cap
    );
    if (section !== null) sections.push(section);
  }
  return sections;
}

function emptyQuerySections<T extends RankingCandidate>(
  candidates: readonly T[],
  snapshot: RankingSnapshot
): readonly RankingSection<T>[] {
  const byId = new Map(candidates.map(candidate => [candidate.id, candidate]));
  const consumed = new Set<string>();
  const pinned: RankedCandidate<T>[] = [];
  // snapshot.pins is daemon-sorted by ascending PinnedAt, with command id as
  // the tie-break, and must be consumed in that order.
  for (const commandId of snapshot.pins) {
    const candidate = byId.get(commandId);
    if (candidate === undefined || consumed.has(candidate.stableKey)) continue;
    consumed.add(candidate.stableKey);
    pinned.push(emptyRanked(candidate));
  }
  const recent: RankedCandidate<T>[] = [];
  const usage = [...snapshot.usage].sort((left, right) =>
    right.last_used_at === left.last_used_at
      ? left.command_id.localeCompare(right.command_id)
      : right.last_used_at - left.last_used_at
  );
  for (const signal of usage) {
    const candidate = byId.get(signal.command_id);
    if (candidate === undefined || consumed.has(candidate.stableKey)) continue;
    consumed.add(candidate.stableKey);
    recent.push(emptyRanked(candidate));
  }
  const contextual = new Map<string, RankedCandidate<T>[]>();
  for (const candidate of candidates) {
    if (candidate.contextual !== true || consumed.has(candidate.stableKey)) continue;
    consumed.add(candidate.stableKey);
    const group = candidate.group.trim() || "Commands";
    const groupCandidates = contextual.get(group);
    if (groupCandidates === undefined) contextual.set(group, [emptyRanked(candidate)]);
    else groupCandidates.push(emptyRanked(candidate));
  }
  const curated = new Map<string, RankedCandidate<T>[]>();
  for (const candidate of candidates) {
    if (candidate.curated !== false && !consumed.has(candidate.stableKey)) {
      const group = candidate.group.trim() || "Commands";
      const groupCandidates = curated.get(group);
      if (groupCandidates === undefined) curated.set(group, [emptyRanked(candidate)]);
      else groupCandidates.push(emptyRanked(candidate));
    }
  }
  const grouped = new Map<string, RankedCandidate<T>[]>();
  for (const group of new Set([...contextual.keys(), ...curated.keys()])) {
    grouped.set(group, [...(contextual.get(group) ?? []), ...(curated.get(group) ?? [])]);
  }
  return [
    makeSection("Pinned", pinned, snapshot.weights.entity_section_visible_cap),
    makeSection("Recents", recent, snapshot.weights.entity_section_visible_cap),
    ...orderSections(grouped, snapshot.weights),
  ].filter((section): section is RankingSection<T> => section !== null);
}

export function assembleRankingResults<T extends RankingCandidate>(
  query: string,
  candidates: readonly T[],
  snapshot: RankingSnapshot
): RankingAssembly<T> {
  if (query.trim() === "") {
    return { sections: emptyQuerySections(candidates, snapshot), fallback: false };
  }
  const ranked = rankCandidates(query, candidates, snapshot);
  const topScore = ranked[0]?.score;
  if (topScore === undefined || topScore < snapshot.weights.fallback_weak_match_threshold) {
    return { sections: [], fallback: true };
  }
  const promoted = ranked.filter(
    candidate => candidate.score >= floorFor(candidate.candidate.subtype, snapshot.weights)
  );
  if (promoted.length === 0) return { sections: [], fallback: true };
  const grouped = new Map<string, RankedCandidate<T>[]>();
  for (const candidate of promoted) {
    const group = candidate.candidate.group.trim() || "Commands";
    const existing = grouped.get(group);
    if (existing === undefined) grouped.set(group, [candidate]);
    else existing.push(candidate);
  }
  return {
    sections: orderSections(grouped, snapshot.weights),
    fallback: topScore === snapshot.weights.fallback_weak_match_threshold,
  };
}
