type SwatchKind = "color" | "radius" | "duration" | "easing" | "tracking";

/**
 * A showcased token. There is deliberately no value here: the showcase reads each
 * token's computed value from the document, so it always shows what tokens.css /
 * tokens-light.css ship for the active theme instead of a copy that drifts.
 */
export interface TokenSwatch {
  token: string;
  role?: string;
  kind: SwatchKind;
}

export interface TokenGroup {
  id: string;
  label: string;
  caption: string;
  swatches: TokenSwatch[];
}

function colors(entries: Array<[token: string, role: string]>): TokenSwatch[] {
  return entries.map(([token, role]) => ({ token, role, kind: "color" }));
}

export const TOKEN_GROUPS: TokenGroup[] = [
  {
    id: "backgrounds",
    label: "Surface ramp",
    caption:
      "Neutral, chroma-free greys in both themes: chrome → desk → pane surface, with sunken insets and a hover step.",
    swatches: colors([
      ["--color-rail", "Chrome: rail + topbar"],
      ["--color-desk", "Desk behind panes"],
      ["--color-canvas", "Pane / card surface"],
      ["--color-canvas-soft", "Menus, dialogs, cards"],
      ["--color-canvas-tint", "Tinted panel"],
      ["--color-sunken", "Tab strip, inset lists"],
      ["--color-surface-2", "Hover, secondary pill"],
      ["--color-selected", "Selected row / rail item"],
      ["--color-code-bg", "Inline code wash"],
      ["--color-elevated", "Raised control fill"],
      ["--color-well", "Identity well (mint over surface)"],
      ["--color-disabled", "Disabled fill"],
    ]),
  },
  {
    id: "hairlines",
    label: "Hairlines",
    caption: "Opaque greys, one ladder per theme. Soft → strong → focus.",
    swatches: colors([
      ["--color-line", "Generic 1 px hairline, seams"],
      ["--color-line-soft", "Group bottoms, popover ring"],
      ["--color-line-strong", "Queued ring, strong divider"],
      ["--color-line-focus", "Input focus border"],
    ]),
  },
  {
    id: "text",
    label: "Text",
    caption: "Neutral text ladder. Every step that carries copy clears WCAG AA in both themes.",
    swatches: colors([
      ["--color-fg-strong", "Titles, active labels"],
      ["--color-fg", "Body"],
      ["--color-fg-3", "Assistant prose"],
      ["--color-fg-2", "Links, legend copy"],
      ["--color-muted", "Secondary copy"],
      ["--color-subtle", "Placeholders, meta"],
      ["--color-faint", "Mono ids, separators"],
    ]),
  },
  {
    id: "primary",
    label: "Primary action",
    caption: "The single primary is inverted: near-white on dark, near-black on light.",
    swatches: colors([
      ["--color-primary", "Primary pill fill"],
      ["--color-primary-hover", "Primary hover"],
      ["--color-primary-foreground", "Text on primary"],
    ]),
  },
  {
    id: "accent",
    label: "Accent",
    caption: "Compozy orange marks highlights and needs-you (attn), never the default action.",
    swatches: colors([
      ["--color-accent", "Highlight / needs you"],
      ["--color-accent-hover", "Accent pressed"],
      ["--color-accent-strong", "Accent text, prose links"],
      ["--color-accent-ink", "Text on accent fill"],
      ["--color-accent-tint", "Chip / pill tint"],
      ["--color-accent-tint-strong", "Bar fill"],
      ["--color-accent-dim", "Accent ring"],
      ["--color-accent-glow", "Pulse keyframe base"],
    ]),
  },
  {
    id: "signal",
    label: "Signal palette",
    caption:
      "Mint marks in-progress and done; amber only real warnings. Signal is state, not taxonomy.",
    swatches: colors([
      ["--color-success", "In progress / done"],
      ["--color-warning", "Real warnings"],
      ["--color-danger", "Error / destructive"],
      ["--color-info", "Informational"],
      ["--color-neutral", "Idle / cancelled"],
    ]),
  },
  {
    id: "tints",
    label: "Signal tints",
    caption: "Background tints for chips, pills, and kind dots.",
    swatches: colors([
      ["--color-success-tint", "Success chip bg"],
      ["--color-warning-tint", "Warning chip bg"],
      ["--color-danger-tint", "Danger chip bg"],
      ["--color-info-tint", "Info chip bg"],
      ["--color-neutral-tint", "Neutral chip bg"],
    ]),
  },
  {
    id: "overlays",
    label: "Overlays",
    caption: "Modal scrim and ghost hover — white-alpha on dark, ink-alpha on light.",
    swatches: [
      ...colors([
        ["--color-overlay-scrim", "Modal / dialog backdrop"],
        ["--color-overlay-ghost-hover", "Ghost hover"],
      ]),
      { token: "--overlay-blur", role: "Dialog / sheet backdrop blur only", kind: "radius" },
    ],
  },
  {
    id: "glaze",
    label: "Surface glaze ladder",
    caption:
      "Translucent ink over the ramp that flips per theme. Inline white or black alpha literals are forbidden.",
    swatches: colors([
      ["--color-row-hover", "List / nav hover (aliased as --hover)"],
      ["--color-row-selected", "List / nav selected baseline"],
      ["--color-surface-glaze", "RadioCard / panel head selected"],
      ["--color-bar-fill", "Priority / progress / usage bars"],
      ["--color-input-fill", "Composer / textarea / search input"],
      ["--color-btn-default-fill", "Neutral Button default fill"],
      ["--color-btn-default-hover", "Neutral Button hover fill"],
      ["--color-badge-fill", "PillGroup count badge bg"],
    ]),
  },
  {
    id: "avatars",
    label: "Owner avatar palette",
    caption:
      "Owner palette resolved via owner-palette.ts colorsFor(); light inks are deepened to read on white.",
    swatches: colors([
      ["--color-avatar-agent-0-bg", "Agent slot 0 — bg"],
      ["--color-avatar-agent-0-fg", "Agent slot 0 — fg"],
      ["--color-avatar-agent-1-bg", "Agent slot 1 — bg"],
      ["--color-avatar-agent-1-fg", "Agent slot 1 — fg"],
      ["--color-avatar-agent-2-bg", "Agent slot 2 — bg"],
      ["--color-avatar-agent-2-fg", "Agent slot 2 — fg"],
      ["--color-avatar-agent-3-bg", "Agent slot 3 — bg"],
      ["--color-avatar-agent-3-fg", "Agent slot 3 — fg"],
      ["--color-avatar-human-0-bg", "Human slot 0 — bg"],
      ["--color-avatar-human-0-fg", "Human slot 0 — fg"],
      ["--color-avatar-human-1-bg", "Human slot 1 — bg"],
      ["--color-avatar-human-1-fg", "Human slot 1 — fg"],
      ["--color-avatar-human-2-bg", "Human slot 2 — bg"],
      ["--color-avatar-human-2-fg", "Human slot 2 — fg"],
    ]),
  },
  {
    id: "radii",
    label: "Radii",
    caption: "Actions are pills; cards and panels take lg; the identity well takes icon-well.",
    swatches: [
      { token: "--radius-xs", role: "Tightest chip", kind: "radius" },
      { token: "--radius-sm", role: "Compact chip, kbd", kind: "radius" },
      { token: "--radius", role: "Default", kind: "radius" },
      { token: "--radius-md", role: "Inputs, menu rows", kind: "radius" },
      { token: "--radius-lg", role: "Cards / panels", kind: "radius" },
      { token: "--radius-xl", role: "Sheet / hero card", kind: "radius" },
      { token: "--radius-icon-well", role: "Identity well", kind: "radius" },
      { token: "--radius-pill", role: "Buttons, pills, search", kind: "radius" },
    ],
  },
  {
    id: "motion",
    label: "Motion",
    caption: "Fast / base / slow tiers; reduced motion zeroes everything.",
    swatches: [
      { token: "--duration-fast", role: "Hover feedback", kind: "duration" },
      { token: "--duration-base", role: "Default", kind: "duration" },
      { token: "--duration-slow", role: "Panel / modal", kind: "duration" },
      { token: "--ease-out", role: "Default easing", kind: "easing" },
    ],
  },
  {
    id: "tracking",
    label: "Tracking",
    caption: "Body tracking and the mono tracking used by badges and protocol strings.",
    swatches: [
      { token: "--tracking-body", role: "Body baseline", kind: "tracking" },
      { token: "--tracking-mono", role: "Mono eyebrow tracking", kind: "tracking" },
    ],
  },
];
