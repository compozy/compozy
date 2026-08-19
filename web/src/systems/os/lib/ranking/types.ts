import type { CmdPaletteRankSignals } from "../cmd-palette-types";

export type RankingWeights = CmdPaletteRankSignals["weights"];
export type RankingSnapshot = CmdPaletteRankSignals;

export type RankingSubtype = "command" | "default" | "path" | "tab";

export interface RankingCandidate {
  readonly stableKey: string;
  readonly id: string;
  readonly label: string;
  readonly group: string;
  readonly aliases?: readonly string[];
  readonly keywords?: readonly string[];
  readonly description?: string;
  readonly contextual?: boolean;
  readonly available?: boolean;
  readonly curated?: boolean;
  readonly subtype?: RankingSubtype;
}

export type MatchKind =
  | "alias-exact"
  | "exact"
  | "prefix"
  | "token-prefix"
  | "compact-prefix"
  | "word-boundary"
  | "contains"
  | "subsequence"
  | "none";

export interface MatchScore {
  readonly kind: MatchKind;
  readonly score: number;
}

export interface RankedCandidate<T extends RankingCandidate = RankingCandidate> {
  readonly candidate: T;
  readonly matchKind: MatchKind;
  readonly matchScore: number;
  readonly frecencyScore: number;
  readonly queryLearningScore: number;
  readonly contextScore: number;
  readonly score: number;
}

export interface RankingSection<T extends RankingCandidate = RankingCandidate> {
  readonly title: string;
  readonly candidates: readonly RankedCandidate<T>[];
  readonly total: number;
}
