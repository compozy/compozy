type SwatchKind = "color" | "radius" | "duration" | "easing" | "tracking";

export interface TokenSwatch {
  token: string;
  value: string;
  role?: string;
  kind: SwatchKind;
}

export interface TokenGroup {
  id: string;
  label: string;
  caption: string;
  swatches: TokenSwatch[];
}

export const TOKEN_GROUPS: TokenGroup[] = [
  {
    id: "backgrounds",
    label: "Surface ramp",
    caption: "Warm-dark layered backgrounds: rail → canvas → soft → tint → elevated.",
    swatches: [
      { token: "--color-rail", value: "#0c0b0b", role: "Workspace rail bg", kind: "color" },
      { token: "--color-canvas", value: "#131211", role: "Page bg", kind: "color" },
      {
        token: "--color-canvas-soft",
        value: "#1a1918",
        role: "Card / group / sidebar bg",
        kind: "color",
      },
      {
        token: "--color-canvas-tint",
        value: "#1c1b1a",
        role: "Kanban card baseline",
        kind: "color",
      },
      { token: "--color-sidebar", value: "#1a1918", role: "Sidebar panel", kind: "color" },
      {
        token: "--color-elevated",
        value: "#232220",
        role: "Active rows, segment-active",
        kind: "color",
      },
      {
        token: "--color-hover",
        value: "var(--color-row-hover)",
        role: "Generic hover (alias of --row-hover)",
        kind: "color",
      },
      { token: "--color-disabled", value: "#4a4847", role: "Disabled fill", kind: "color" },
    ],
  },
  {
    id: "hairlines",
    label: "Hairlines",
    caption: "Translucent rails derived from white. Soft → strong scales focus + dividers.",
    swatches: [
      {
        token: "--color-line",
        value: "rgba(255, 255, 255, 0.055)",
        role: "Generic 1 px hairline",
        kind: "color",
      },
      {
        token: "--color-line-soft",
        value: "rgba(255, 255, 255, 0.03)",
        role: "Group bottoms, popover ring",
        kind: "color",
      },
      {
        token: "--color-line-strong",
        value: "rgba(255, 255, 255, 0.09)",
        role: "Focus ring, scrollbar thumb hover",
        kind: "color",
      },
    ],
  },
  {
    id: "text",
    label: "Text",
    caption: "Five-step neutral text scale with explicit label/eyebrow roles.",
    swatches: [
      { token: "--color-fg", value: "#ececef", role: "Body", kind: "color" },
      {
        token: "--color-fg-strong",
        value: "#f6f6f8",
        role: "Titles, active labels",
        kind: "color",
      },
      { token: "--color-muted", value: "#9a9a9f", role: "Secondary copy", kind: "color" },
      { token: "--color-subtle", value: "#76767c", role: "Placeholders", kind: "color" },
      { token: "--color-faint", value: "#545458", role: "Mono ids, separators", kind: "color" },
    ],
  },
  {
    id: "accent",
    label: "Accent",
    caption: "Warm orange is the only non-neutral hue. Tints replace solid banners.",
    swatches: [
      { token: "--color-accent", value: "#e8572a", role: "Action / Primary", kind: "color" },
      { token: "--color-accent-hover", value: "#d14e25", role: "Accent pressed", kind: "color" },
      {
        token: "--color-accent-strong",
        value: "#f6874f",
        role: "Highlight accent",
        kind: "color",
      },
      { token: "--color-accent-ink", value: "#17110f", role: "Text on accent fill", kind: "color" },
      {
        token: "--color-accent-tint",
        value: "rgba(232, 87, 42, 0.1)",
        role: "Chip / pill tint",
        kind: "color",
      },
      {
        token: "--color-accent-tint-strong",
        value: "rgba(232, 87, 42, 0.16)",
        role: "Bar fill",
        kind: "color",
      },
      {
        token: "--color-accent-dim",
        value: "rgba(232, 87, 42, 0.24)",
        role: "Legacy focus ring",
        kind: "color",
      },
      {
        token: "--color-accent-glow",
        value: "rgba(232, 87, 42, 0.05)",
        role: "Pulse keyframe base",
        kind: "color",
      },
    ],
  },
  {
    id: "signal",
    label: "Signal palette",
    caption:
      "Desaturated signals. Tint backgrounds at 6–10% alpha; full-color text on tint surfaces.",
    swatches: [
      { token: "--color-success", value: "#5fbf85", role: "Stable / Live", kind: "color" },
      { token: "--color-warning", value: "#d6a647", role: "Caution / Pending", kind: "color" },
      { token: "--color-danger", value: "#e0635a", role: "Error / Destructive", kind: "color" },
      { token: "--color-info", value: "#8e8eb5", role: "Informational", kind: "color" },
      { token: "--color-neutral", value: "#7a7a80", role: "Idle / Cancelled", kind: "color" },
    ],
  },
  {
    id: "tints",
    label: "Signal tints",
    caption: "Background tints (6–10% alpha) for chips, pills, and kind dots.",
    swatches: [
      {
        token: "--color-success-tint",
        value: "rgba(95, 191, 133, 0.08)",
        role: "Success chip bg",
        kind: "color",
      },
      {
        token: "--color-warning-tint",
        value: "rgba(214, 166, 71, 0.08)",
        role: "Warning chip bg",
        kind: "color",
      },
      {
        token: "--color-danger-tint",
        value: "rgba(224, 99, 90, 0.09)",
        role: "Danger chip bg",
        kind: "color",
      },
      {
        token: "--color-info-tint",
        value: "rgba(142, 142, 181, 0.12)",
        role: "Info chip bg / Settings observability",
        kind: "color",
      },
      {
        token: "--color-neutral-tint",
        value: "rgba(150, 150, 155, 0.06)",
        role: "Neutral chip bg (warmed for ramp parity)",
        kind: "color",
      },
    ],
  },
  {
    id: "overlays",
    label: "Overlays",
    caption: "Modal scrim, ghost hover, text selection — all token-driven.",
    swatches: [
      {
        token: "--color-overlay-scrim",
        value: "rgba(0, 0, 0, 0.55)",
        role: "Modal / dialog backdrop",
        kind: "color",
      },
      {
        token: "--overlay-blur",
        value: "3px",
        role: "Dialog / sheet backdrop blur ONLY",
        kind: "radius",
      },
      {
        token: "--color-overlay-ghost-hover",
        value: "rgba(255, 255, 255, 0.06)",
        role: "Ghost hover on dark",
        kind: "color",
      },
    ],
  },
  {
    id: "glaze",
    label: "Surface glaze ladder",
    caption:
      "Translucent white tints layered on the warm ramp. Inline rgba literals are forbidden.",
    swatches: [
      {
        token: "--color-row-hover",
        value: "rgba(255, 255, 255, 0.022)",
        role: "List / nav hover (aliased as --hover)",
        kind: "color",
      },
      {
        token: "--color-row-selected",
        value: "rgba(255, 255, 255, 0.03)",
        role: "List / nav selected baseline",
        kind: "color",
      },
      {
        token: "--color-surface-glaze",
        value: "rgba(255, 255, 255, 0.04)",
        role: "RadioCard / panel head selected",
        kind: "color",
      },
      {
        token: "--color-bar-fill",
        value: "rgba(255, 255, 255, 0.085)",
        role: "Priority / progress / usage bars",
        kind: "color",
      },
      {
        token: "--color-input-fill",
        value: "rgba(255, 255, 255, 0.025)",
        role: "Composer / textarea / search input",
        kind: "color",
      },
      {
        token: "--color-btn-default-fill",
        value: "rgba(255, 255, 255, 0.04)",
        role: "Neutral Button default fill",
        kind: "color",
      },
      {
        token: "--color-btn-default-hover",
        value: "rgba(255, 255, 255, 0.07)",
        role: "Neutral Button hover fill",
        kind: "color",
      },
      {
        token: "--color-badge-fill",
        value: "rgba(255, 255, 255, 0.05)",
        role: "PillGroup count badge bg",
        kind: "color",
      },
    ],
  },
  {
    id: "avatars",
    label: "Owner avatar palette",
    caption:
      "Tokenised owner palette resolved via web/src/lib/owner-palette.ts colorsFor(). Storybook + design ref tools read from the same source.",
    swatches: [
      {
        token: "--color-avatar-agent-0-bg",
        value: "rgba(232, 144, 99, 0.18)",
        role: "Agent slot 0 — bg",
        kind: "color",
      },
      {
        token: "--color-avatar-agent-0-fg",
        value: "#f2b895",
        role: "Agent slot 0 — fg",
        kind: "color",
      },
      {
        token: "--color-avatar-agent-1-bg",
        value: "rgba(168, 178, 220, 0.16)",
        role: "Agent slot 1 — bg",
        kind: "color",
      },
      {
        token: "--color-avatar-agent-1-fg",
        value: "#c5cce7",
        role: "Agent slot 1 — fg",
        kind: "color",
      },
      {
        token: "--color-avatar-agent-2-bg",
        value: "rgba(143, 196, 178, 0.18)",
        role: "Agent slot 2 — bg",
        kind: "color",
      },
      {
        token: "--color-avatar-agent-2-fg",
        value: "#a9d9c7",
        role: "Agent slot 2 — fg",
        kind: "color",
      },
      {
        token: "--color-avatar-agent-3-bg",
        value: "rgba(214, 168, 192, 0.18)",
        role: "Agent slot 3 — bg",
        kind: "color",
      },
      {
        token: "--color-avatar-agent-3-fg",
        value: "#e0bcd0",
        role: "Agent slot 3 — fg",
        kind: "color",
      },
      {
        token: "--color-avatar-human-0-bg",
        value: "rgba(220, 192, 134, 0.2)",
        role: "Human slot 0 — bg",
        kind: "color",
      },
      {
        token: "--color-avatar-human-0-fg",
        value: "#e5cc9a",
        role: "Human slot 0 — fg",
        kind: "color",
      },
      {
        token: "--color-avatar-human-1-bg",
        value: "rgba(195, 178, 156, 0.2)",
        role: "Human slot 1 — bg",
        kind: "color",
      },
      {
        token: "--color-avatar-human-1-fg",
        value: "#d6c5aa",
        role: "Human slot 1 — fg",
        kind: "color",
      },
      {
        token: "--color-avatar-human-2-bg",
        value: "rgba(192, 173, 178, 0.2)",
        role: "Human slot 2 — bg",
        kind: "color",
      },
      {
        token: "--color-avatar-human-2-fg",
        value: "#d2bfc5",
        role: "Human slot 2 — fg",
        kind: "color",
      },
    ],
  },
  {
    id: "layout-grammar",
    label: "Layout grammar",
    caption:
      "Modal width ladder + logo well sizes. Inline arbitrary widths are forbidden in modal / catalog surfaces.",
    swatches: [
      {
        token: "--width-modal-sm",
        value: "560px",
        role: "Confirm / single-field editor",
        kind: "radius",
      },
      {
        token: "--width-modal-md",
        value: "720px",
        role: "Task editor / settings field editor",
        kind: "radius",
      },
      {
        token: "--width-modal-lg",
        value: "880px",
        role: "Bridges wizard / knowledge create dialog",
        kind: "radius",
      },
      {
        token: "--size-catalog-logo",
        value: "1.5rem",
        role: "CatalogCard logoSize='default' (24 px)",
        kind: "radius",
      },
      {
        token: "--size-provider-logo-well",
        value: "2.5rem",
        role: "CatalogCard logoSize='lg' (40 px) / settings provider card",
        kind: "radius",
      },
    ],
  },
  {
    id: "protocol-kinds",
    label: "Protocol Kind Colors",
    caption:
      "Kind-dot colors map onto the new palette: say/whois → neutral, greet/trace → info, direct → accent, receipt → success, capability → warning.",
    swatches: [
      { token: "--color-kind-say", value: "var(--color-neutral)", role: "say", kind: "color" },
      { token: "--color-kind-greet", value: "var(--color-info)", role: "greet", kind: "color" },
      {
        token: "--color-kind-direct",
        value: "var(--color-accent)",
        role: "direct",
        kind: "color",
      },
      {
        token: "--color-kind-receipt",
        value: "var(--color-success)",
        role: "receipt",
        kind: "color",
      },
      {
        token: "--color-kind-capability",
        value: "var(--color-warning)",
        role: "capability",
        kind: "color",
      },
      { token: "--color-kind-trace", value: "var(--color-info)", role: "trace", kind: "color" },
      { token: "--color-kind-whois", value: "var(--color-neutral)", role: "whois", kind: "color" },
    ],
  },
  {
    id: "radii",
    label: "Radii",
    caption: "Ladder: 4 / 5 / 6 / 8 / 10 / 14 / pill.",
    swatches: [
      { token: "--radius-xs", value: "4px", role: "Tightest chip", kind: "radius" },
      { token: "--radius-sm", value: "5px", role: "Kind chip", kind: "radius" },
      { token: "--radius", value: "6px", role: "Default", kind: "radius" },
      { token: "--radius-md", value: "8px", role: "Inputs / buttons", kind: "radius" },
      { token: "--radius-lg", value: "10px", role: "Cards / panels", kind: "radius" },
      { token: "--radius-xl", value: "14px", role: "Sheet / hero card", kind: "radius" },
      { token: "--radius-pill", value: "9999px", role: "Pill / search", kind: "radius" },
    ],
  },
  {
    id: "motion",
    label: "Motion",
    caption: "One fast tier (--dur 140ms) + one slow tier; reduced-motion zeroes everything.",
    swatches: [
      { token: "--duration-base", value: "140ms", role: "Default", kind: "duration" },
      {
        token: "--duration-slow",
        value: "200ms",
        role: "Panel / modal",
        kind: "duration",
      },
      {
        token: "--ease-out",
        value: "cubic-bezier(0.2, 0, 0, 1)",
        role: "Default easing",
        kind: "easing",
      },
    ],
  },
  {
    id: "tracking",
    label: "Tracking",
    caption: "Mono tracking used across eyebrows, badges, and protocol strings.",
    swatches: [
      {
        token: "--tracking-mono",
        value: "0.06em",
        role: "Mono eyebrow tracking",
        kind: "tracking",
      },
    ],
  },
];
