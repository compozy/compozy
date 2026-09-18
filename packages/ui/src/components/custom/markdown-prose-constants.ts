export const INLINE_CODE_CLASS =
  "rounded-xs bg-surface-glaze px-1.5 py-px font-mono text-inline-code text-fg-strong group-data-[compact=true]/md:px-1 group-data-[compact=true]/md:text-form-input";

// Font-size utility per prose tier; the components and the token contract test share this map.
export const PROSE_TYPE = {
  body: "text-card-title",
  h1: "text-prose-h1",
  h2: "text-prose-h2",
  h3: "text-prose-h3",
  h4: "text-card-title",
  label: "text-small-body",
} as const;

// Dense-surface heading tier, selected by `data-compact` on the `group/md` root.
export const PROSE_TYPE_COMPACT = {
  h1: "group-data-[compact=true]/md:text-item-title",
  h2: "group-data-[compact=true]/md:text-card-title",
  h3: "group-data-[compact=true]/md:text-small-body",
  h4: "group-data-[compact=true]/md:text-small-body",
} as const;

// Link ink: the underline is the non-color cue, so it shares the text color and its contrast.
export const PROSE_LINK = {
  text: "text-accent-strong",
  underline: "decoration-accent-strong",
} as const;
