/**
 * Generation 1's blob tones, per theme, as positions in Blobatar's tone set. A pale body vanishes
 * on the light surfaces and the ink body vanishes on the dark ones, so each theme keeps only the
 * swatches that read on it. A later generation adds its own row and leaves this one alone.
 */
const PASTEL = 0.1;
const PALE = 0.28;
const MID = 0.5;
const DEEP = 0.7;
const BRIGHT = 0.85;
const INK = 0.95;

export const AGENT_AVATAR_TONES = {
  1: { dark: [PASTEL, PALE, MID, DEEP, BRIGHT], light: [MID, DEEP, INK] },
} as const;

export type AgentAvatarGeneration = keyof typeof AGENT_AVATAR_TONES;

const LATEST_GENERATION: AgentAvatarGeneration = 1;

/** The row a saved `generation` pins; an unknown number reads as the newest this build has. */
export function agentAvatarTones(generation: number | undefined, theme: "dark" | "light") {
  const row =
    AGENT_AVATAR_TONES[generation as AgentAvatarGeneration] ??
    AGENT_AVATAR_TONES[LATEST_GENERATION];
  return row[theme];
}

function linear(channel: number) {
  return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
}

/** The OKLCh hue of `#rrggbb`, which is how `appearance.color` reaches the body. */
export function agentAvatarHue(color: string) {
  const [r, g, b] = [1, 3, 5].map(at => linear(parseInt(color.slice(at, at + 2), 16) / 255)) as [
    number,
    number,
    number,
  ];
  const l = Math.cbrt(0.4122 * r + 0.5363 * g + 0.0514 * b);
  const m = Math.cbrt(0.2119 * r + 0.6807 * g + 0.1074 * b);
  const s = Math.cbrt(0.0883 * r + 0.2817 * g + 0.63 * b);
  const a = 1.9779 * l - 2.4286 * m + 0.4506 * s;
  const lab = 0.0259 * l + 0.7828 * m - 0.8087 * s;
  return ((Math.atan2(lab, a) * 180) / Math.PI + 360) % 360;
}
