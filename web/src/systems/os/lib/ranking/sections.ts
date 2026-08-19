import { rankCandidates } from "./rank";
import type {
  RankedCandidate,
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

function emptyQuerySections<T extends RankingCandidate>(
  candidates: readonly T[],
  snapshot: RankingSnapshot
): readonly RankingSection<T>[] {
  const byId = new Map(candidates.map(candidate => [candidate.id, candidate]));
  const consumed = new Set<string>();
  const pinned: RankedCandidate<T>[] = [];
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
  const curated: RankedCandidate<T>[] = [];
  for (const candidate of candidates) {
    if (candidate.curated !== false && !consumed.has(candidate.stableKey)) {
      curated.push(emptyRanked(candidate));
    }
  }
  const sections: Array<RankingSection<T> | null> = [
    makeSection("Pinned", pinned, snapshot.weights.entity_section_visible_cap),
    makeSection("Recents", recent, snapshot.weights.entity_section_visible_cap),
  ];
  for (const group of snapshot.weights.group_order) {
    const section = makeSection(
      group,
      contextual.get(group) ?? [],
      snapshot.weights.entity_section_visible_cap
    );
    if (section !== null) sections.push(section);
    contextual.delete(group);
  }
  for (const group of [...contextual.keys()].sort((left, right) => left.localeCompare(right))) {
    sections.push(
      makeSection(group, contextual.get(group) ?? [], snapshot.weights.entity_section_visible_cap)
    );
  }
  sections.push(makeSection("Curated", curated, snapshot.weights.entity_section_visible_cap));
  return sections.filter((section): section is RankingSection<T> => section !== null);
}

export function assembleRankingSections<T extends RankingCandidate>(
  query: string,
  candidates: readonly T[],
  snapshot: RankingSnapshot
): readonly RankingSection<T>[] {
  if (query.trim() === "") return emptyQuerySections(candidates, snapshot);
  const ranked = rankCandidates(query, candidates, snapshot).filter(
    candidate => candidate.score >= floorFor(candidate.candidate.subtype, snapshot.weights)
  );
  const grouped = new Map<string, RankedCandidate<T>[]>();
  for (const candidate of ranked) {
    const group = candidate.candidate.group.trim() || "Commands";
    const existing = grouped.get(group);
    if (existing === undefined) grouped.set(group, [candidate]);
    else existing.push(candidate);
  }
  const sections: RankingSection<T>[] = [];
  for (const group of snapshot.weights.group_order) {
    const section = makeSection(
      group,
      grouped.get(group) ?? [],
      snapshot.weights.entity_section_visible_cap
    );
    if (section !== null) sections.push(section);
    grouped.delete(group);
  }
  for (const group of [...grouped.keys()].sort((left, right) => left.localeCompare(right))) {
    const section = makeSection(
      group,
      grouped.get(group) ?? [],
      snapshot.weights.entity_section_visible_cap
    );
    if (section !== null) sections.push(section);
  }
  return sections;
}
