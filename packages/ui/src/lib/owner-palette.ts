/**
 * Owner-avatar palette resolution against the tokenised CSS variables declared in
 * `packages/ui/src/tokens.css` (`--color-avatar-{agent,human,system}-*-{bg,fg}`).
 *
 * the palette is a glance-aid resolved via CSS variables;
 * `<OwnerAvatar>` and downstream surfaces consume the `var(...)` strings so retunes
 * propagate from tokens.css.
 */
export type OwnerKind = "agent" | "human" | "system";

export interface OwnerColors {
  /** CSS-variable expression for the avatar background. */
  bg: string;
  /** CSS-variable expression for the avatar foreground (text + glyph). */
  fg: string;
}

export const AGENT_SLOT_COUNT = 4;
export const HUMAN_SLOT_COUNT = 3;
export const SYSTEM_SLOT_COUNT = 1;

const FNV_OFFSET_BASIS = 0x811c9dc5;
const FNV_PRIME = 0x01000193;

function fnv1aHash(value: string): number {
  let hash = FNV_OFFSET_BASIS;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, FNV_PRIME);
  }
  return hash >>> 0;
}

function slotCount(kind: OwnerKind): number {
  switch (kind) {
    case "agent":
      return AGENT_SLOT_COUNT;
    case "human":
      return HUMAN_SLOT_COUNT;
    case "system":
      return SYSTEM_SLOT_COUNT;
  }
}

/** Deterministic slot index for the (kind, ownerId) pair. Stable across runs. */
export function seed(kind: OwnerKind, ownerId: string): number {
  return fnv1aHash(ownerId) % slotCount(kind);
}

/**
 * Resolves the (bg, fg) CSS-variable expressions for an owner identity. Returns literal
 * `var(--avatar-...)` strings so the consumer can drop them straight into a `style`
 * object.
 */
export function colorsFor(kind: OwnerKind, ownerId: string): OwnerColors {
  if (kind === "system") {
    return {
      bg: "var(--color-avatar-system-bg)",
      fg: "var(--color-avatar-system-fg)",
    };
  }
  const index = seed(kind, ownerId);
  return {
    bg: `var(--color-avatar-${kind}-${index}-bg)`,
    fg: `var(--color-avatar-${kind}-${index}-fg)`,
  };
}
