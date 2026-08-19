import { normalizeRankingText } from "./normalize";
import type { RankedCandidate, RankingCandidate, RankingWeights } from "./types";

export function ghostCompletion(
  query: string,
  ranked: readonly RankedCandidate<RankingCandidate>[],
  weights: RankingWeights
): string | null {
  const top = ranked[0];
  const runnerUp = ranked[1];
  const normalizedQuery = normalizeRankingText(query).text;
  if (top === undefined || normalizedQuery === "" || top.score < weights.ghost_min_score)
    return null;
  if (runnerUp !== undefined && top.score - runnerUp.score <= weights.deadband) return null;
  const normalizedLabel = normalizeRankingText(top.candidate.label).text;
  if (!normalizedLabel.startsWith(normalizedQuery) || normalizedLabel === normalizedQuery)
    return null;
  return top.candidate.label.slice(query.length);
}

export function acceptGhostCompletion(
  query: string,
  tail: string | null,
  selectionStart: number | null,
  selectionEnd: number | null
): string | null {
  if (tail === null || selectionStart !== query.length || selectionEnd !== query.length)
    return null;
  return query + tail;
}
