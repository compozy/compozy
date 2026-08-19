import { matchRankingCandidate } from "./match";
import { normalizeRankingText } from "./normalize";
import type { RankedCandidate, RankingCandidate, RankingSnapshot, RankingWeights } from "./types";

const DAY_MS = 24 * 60 * 60 * 1_000;

export function decayFrecency(
  weight: number,
  lastUsedAt: number,
  now: number,
  halfLifeDays: number
): number {
  if (weight <= 0 || halfLifeDays <= 0 || now <= lastUsedAt) return Math.max(0, weight);
  return weight * Math.pow(0.5, (now - lastUsedAt) / (halfLifeDays * DAY_MS));
}

export function isPrunableSignal(
  weight: number,
  lastUsedAt: number,
  now: number,
  weights: RankingWeights
): boolean {
  return now - lastUsedAt >= weights.prune_after_days * DAY_MS && weight < weights.prune_threshold;
}

function groupRank(group: string, weights: RankingWeights): number {
  const rank = weights.group_order.indexOf(group);
  return rank === -1 ? weights.group_order.length : rank;
}

function frecencyScore(commandId: string, snapshot: RankingSnapshot): number {
  const usage = snapshot.usage.find(signal => signal.command_id === commandId);
  if (usage === undefined || usage.weight <= 0) return 0;
  return Math.min(
    snapshot.weights.frecency_cap,
    snapshot.weights.frecency_scale * Math.log1p(usage.weight)
  );
}

function queryLearningScore(query: string, commandId: string, snapshot: RankingSnapshot): number {
  const normalized = normalizeRankingText(query).text;
  if (normalized === "") return 0;
  let weight = 0;
  for (const hit of snapshot.query_hits) {
    if (hit.command_id !== commandId) continue;
    const learned = normalizeRankingText(hit.query).text;
    if (learned.startsWith(normalized) || normalized.startsWith(learned)) weight += hit.weight;
  }
  if (weight <= 0) return 0;
  return snapshot.weights.query_learning_cap * (weight / (weight + 1));
}

function scoreBucket(score: number, deadband: number): number {
  return deadband <= 0 ? score : Math.floor(score / deadband);
}

export function compareRankedCandidates<T extends RankingCandidate>(
  left: RankedCandidate<T>,
  right: RankedCandidate<T>,
  weights: RankingWeights
): number {
  const group =
    groupRank(left.candidate.group, weights) - groupRank(right.candidate.group, weights);
  if (group !== 0) return group;
  if ((left.candidate.available ?? true) !== (right.candidate.available ?? true)) {
    return left.candidate.available === false ? 1 : -1;
  }
  const bucket =
    scoreBucket(right.score, weights.deadband) - scoreBucket(left.score, weights.deadband);
  if (bucket !== 0) return bucket;
  if (left.candidate.label.length !== right.candidate.label.length) {
    return left.candidate.label.length - right.candidate.label.length;
  }
  const label = left.candidate.label.localeCompare(right.candidate.label);
  return label === 0 ? left.candidate.stableKey.localeCompare(right.candidate.stableKey) : label;
}

export function rankCandidates<T extends RankingCandidate>(
  query: string,
  candidates: readonly T[],
  snapshot: RankingSnapshot
): readonly RankedCandidate<T>[] {
  if (normalizeRankingText(query).text.length > snapshot.weights.max_query_length) return [];
  const ranked: RankedCandidate<T>[] = [];
  const seen = new Set<string>();
  for (const candidate of candidates) {
    if (seen.has(candidate.stableKey)) continue;
    seen.add(candidate.stableKey);
    const match = matchRankingCandidate(query, candidate, snapshot.weights);
    if (match === null) continue;
    const frecency = frecencyScore(candidate.id, snapshot);
    const learned = queryLearningScore(query, candidate.id, snapshot);
    const context = candidate.contextual === true ? snapshot.weights.context_boost : 0;
    ranked.push({
      candidate,
      matchKind: match.kind,
      matchScore: match.score,
      frecencyScore: frecency,
      queryLearningScore: learned,
      contextScore: context,
      score: match.score + frecency + learned + context,
    });
  }
  return ranked.sort((left, right) => compareRankedCandidates(left, right, snapshot.weights));
}
