import { versionChipHeight } from "@/components/version-chip";

/**
 * Leading for the feature stack. The features are independent items, not one sentence, so they get
 * enough air to read as a list — the band reserves exactly this much per line.
 */
export const FEATURE_LINE_HEIGHT = 1.45;

interface FormatMetrics {
  gutter: number;
  lockupHeight: number;
  versionFontSize: number;
  featureFontSize: number;
  /** Lockup to version chip. */
  headGap: number;
  /** Version chip to the feature band. */
  bandGap: number;
  /** Where the block sits in the free vertical space: 0.5 is dead centre, lower rides higher. */
  centerBias: number;
}

/**
 * Sizes per aspect, following video-layout.md: supporting text stays at or above the 44px-per-1080px
 * floor once scaled, and nothing lands inside the safe area.
 */
const LANDSCAPE: FormatMetrics = {
  gutter: 140,
  lockupHeight: 118,
  versionFontSize: 30,
  featureFontSize: 72,
  headGap: 40,
  bandGap: 76,
  centerBias: 0.5,
};

const SQUARE: FormatMetrics = {
  gutter: 96,
  lockupHeight: 92,
  versionFontSize: 22,
  featureFontSize: 52,
  headGap: 30,
  bandGap: 56,
  centerBias: 0.5,
};

/**
 * Portrait gets larger type — 9:16 is watched small — and rides above centre, because feed players
 * park their own chrome along the bottom of the frame.
 */
const PORTRAIT: FormatMetrics = {
  gutter: 80,
  lockupHeight: 110,
  versionFontSize: 26,
  featureFontSize: 60,
  headGap: 34,
  bandGap: 64,
  centerBias: 0.42,
};

export interface ReleaseLayout extends FormatMetrics {
  /** Top of the lockup once the scene has settled. */
  headTop: number;
  /** How far the lockup-plus-chip group travels up to open the feature band. */
  groupLift: number;
  featureBandTop: number;
  featureBandHeight: number;
}

export function resolveLayout(width: number, height: number, featureCount: number): ReleaseLayout {
  const aspect = width / height;
  const metrics = aspect > 1.2 ? LANDSCAPE : aspect < 0.8 ? PORTRAIT : SQUARE;

  const headHeight =
    metrics.lockupHeight + metrics.headGap + versionChipHeight(metrics.versionFontSize);
  const featureBandHeight = featureCount * metrics.featureFontSize * FEATURE_LINE_HEIGHT;
  const blockHeight = headHeight + metrics.bandGap + featureBandHeight;

  const headTop = (height - blockHeight) * metrics.centerBias;

  return {
    ...metrics,
    headTop,
    // The group opens on its own, balanced the same way, and ends at headTop; the delta is the travel.
    groupLift: headTop - (height - headHeight) * metrics.centerBias,
    featureBandTop: headTop + headHeight + metrics.bandGap,
    featureBandHeight,
  };
}
