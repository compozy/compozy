import { normalizeRankingText, subsequenceMatch, type NormalizedText } from "./normalize";
import type { MatchScore, RankingCandidate, RankingWeights } from "./types";

function interpolate(minimum: number, maximum: number, ratio: number): number {
  return minimum + (maximum - minimum) * ratio;
}

function scoreNormalizedField(
  term: string,
  field: NormalizedText,
  weights: RankingWeights
): MatchScore | null {
  if (term === "" || field.text === "") return null;
  if (field.text === term) return { kind: "exact", score: weights.match_exact };
  if (field.text.startsWith(term)) return { kind: "prefix", score: weights.match_prefix };
  if (field.tokens.some(token => token.startsWith(term))) {
    return { kind: "token-prefix", score: weights.match_token_prefix };
  }
  if (field.compact.startsWith(term)) {
    return { kind: "compact-prefix", score: weights.match_compact_prefix };
  }
  if (field.text.includes(term)) return { kind: "contains", score: weights.match_contains };
  const subsequence = subsequenceMatch(term, field.text);
  if (subsequence === null) return null;
  if (subsequence.boundaryMatches > 1) {
    return {
      kind: "word-boundary",
      score: interpolate(
        weights.match_word_boundary_min,
        weights.match_word_boundary_max,
        subsequence.density
      ),
    };
  }
  return {
    kind: "subsequence",
    score: interpolate(
      weights.match_subsequence_min,
      weights.match_subsequence_max,
      subsequence.density
    ),
  };
}

function betterMatch(left: MatchScore | null, right: MatchScore | null): MatchScore | null {
  if (left === null) return right;
  if (right === null) return left;
  return left.score >= right.score ? left : right;
}

function secondary(match: MatchScore | null, weights: RankingWeights): MatchScore | null {
  if (match === null) return null;
  return {
    kind: match.kind,
    score: Math.min(weights.secondary_field_cap, match.score * weights.secondary_field_multiplier),
  };
}

export function matchRankingCandidate(
  query: string,
  candidate: RankingCandidate,
  weights: RankingWeights
): MatchScore | null {
  const normalizedQuery = normalizeRankingText(query);
  if (normalizedQuery.text === "") return { kind: "none", score: 0 };
  const aliases = (candidate.aliases ?? []).map(normalizeRankingText);
  if (aliases.some(alias => alias.text === normalizedQuery.text)) {
    return { kind: "alias-exact", score: weights.match_alias_exact };
  }
  const label = normalizeRankingText(candidate.label);
  if (label.text === normalizedQuery.text) return { kind: "exact", score: weights.match_exact };

  const primaryFields = [label, ...aliases];
  const secondaryFields = [
    normalizeRankingText(candidate.id),
    ...(candidate.keywords ?? []).map(normalizeRankingText),
    normalizeRankingText(candidate.description ?? ""),
  ];
  const termMatches: MatchScore[] = [];
  for (const token of normalizedQuery.tokens) {
    let best: MatchScore | null = null;
    for (const field of primaryFields) {
      best = betterMatch(best, scoreNormalizedField(token, field, weights));
    }
    for (const field of secondaryFields) {
      best = betterMatch(best, secondary(scoreNormalizedField(token, field, weights), weights));
    }
    if (best === null) return null;
    termMatches.push(best);
  }
  const score = termMatches.reduce((total, match) => total + match.score, 0) / termMatches.length;
  const bestTermMatch = termMatches.reduce((best, match) =>
    match.score > best.score ? match : best
  );
  return { kind: bestTermMatch.kind, score };
}
