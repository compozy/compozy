// Identity palette for `MessageAvatar`. The 8 slots map onto the canonical
// `--color-avatar-{agent,human,system}-*-{bg,fg}` tokens declared in
// `packages/ui/src/tokens.css` (the same tokens consumed by `<OwnerAvatar>`
// via `colorsFor()` from `@agh/ui`). Network avatars do not have a known
// kind at every callsite, so we keep the seed-indexed table here; retunes
// still flow through tokens.css.
export const NETWORK_IDENTITY_PALETTE: ReadonlyArray<readonly [string, string]> = [
  ["var(--color-avatar-agent-0-bg)", "var(--color-avatar-agent-0-fg)"],
  ["var(--color-avatar-agent-1-bg)", "var(--color-avatar-agent-1-fg)"],
  ["var(--color-avatar-agent-2-bg)", "var(--color-avatar-agent-2-fg)"],
  ["var(--color-avatar-agent-3-bg)", "var(--color-avatar-agent-3-fg)"],
  ["var(--color-avatar-human-0-bg)", "var(--color-avatar-human-0-fg)"],
  ["var(--color-avatar-human-1-bg)", "var(--color-avatar-human-1-fg)"],
  ["var(--color-avatar-human-2-bg)", "var(--color-avatar-human-2-fg)"],
  ["var(--color-avatar-system-bg)", "var(--color-avatar-system-fg)"],
];

export function pickIdentityPaletteIndex(seed: string): number {
  let hash = 0;
  for (let index = 0; index < seed.length; index += 1) {
    hash = (hash * 31 + seed.charCodeAt(index)) | 0;
  }
  return Math.abs(hash) % NETWORK_IDENTITY_PALETTE.length;
}

export function pickIdentityPaletteColors(seed: string): readonly [string, string] {
  const palette = NETWORK_IDENTITY_PALETTE[pickIdentityPaletteIndex(seed)];
  if (!palette) {
    throw new Error("network identity palette is empty");
  }
  return palette;
}

export function getIdentityInitial(value: string | null | undefined): string {
  const trimmed = value?.trim() ?? "";
  if (trimmed.length === 0) {
    return "?";
  }
  return trimmed.charAt(0).toUpperCase();
}
