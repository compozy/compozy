import { cva } from "class-variance-authority";

// Disabled is a deliberate state, not a fade: a faded inverted pill reads as a
// washed-out grey. Filled variants settle on the quiet `surface-2` plate with
// `subtle` ink; bare variants keep no fill and only lose their ink.
const DISABLED_FILLED = "disabled:bg-surface-2 disabled:text-subtle disabled:shadow-none";
const DISABLED_BARE = "disabled:text-subtle";
const DISABLED_OUTLINE = "disabled:border-line-soft disabled:text-subtle";
// A pressed toggle button reads as the hover plate held down — a fill and full
// ink, never a ring or outline.
const PRESSED_PLATE = "aria-pressed:bg-surface-2 aria-pressed:text-fg";

// Geometry (polish contract P4): every size pairs one height with one text
// tier and one glyph size — xs 22 / 12 / 12 · sm 26 / 12.5 / 14 · default
// 30 / 13 / 16 · lg 34 and cta 36 / 13.5 / 16. An explicit `size-*` on the
// glyph wins over the size default.
const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center rounded-pill border border-transparent bg-clip-padding font-sans font-medium whitespace-nowrap transition-[color,background-color,border-color,box-shadow,opacity,transform] duration-fast ease-out outline-none select-none focus-visible:outline-none focus-visible:shadow-focus-ring active:not-aria-[haspopup]:translate-y-px disabled:pointer-events-none aria-invalid:border-danger [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: `bg-primary text-primary-foreground hover:bg-primary-hover aria-expanded:bg-primary-hover [a]:hover:bg-primary-hover ${DISABLED_FILLED}`,
        primary: `bg-primary text-primary-foreground hover:bg-primary-hover aria-expanded:bg-primary-hover [a]:hover:bg-primary-hover ${DISABLED_FILLED}`,
        outline: `border-line bg-transparent text-fg hover:bg-surface-2 aria-expanded:bg-surface-2 ${PRESSED_PLATE} ${DISABLED_OUTLINE}`,
        secondary: `bg-surface-2 text-fg hover:bg-selected aria-expanded:bg-selected aria-pressed:bg-selected ${DISABLED_FILLED}`,
        ghost: `text-fg hover:bg-surface-2 aria-expanded:bg-surface-2 aria-expanded:text-fg ${PRESSED_PLATE} ${DISABLED_BARE}`,
        quiet: `text-muted hover:bg-surface-2 hover:text-fg aria-expanded:bg-surface-2 aria-expanded:text-fg ${PRESSED_PLATE} ${DISABLED_BARE}`,
        destructive: `bg-danger-tint text-danger hover:bg-danger-tint hover:opacity-90 ${DISABLED_FILLED}`,
        "destructive-solid": `bg-danger text-accent-ink hover:bg-[color-mix(in_srgb,var(--color-danger)_88%,black)] ${DISABLED_FILLED}`,
        success: `bg-success-tint text-success hover:opacity-90 ${DISABLED_FILLED}`,
        /** Text link ("Edit ›", "View all ›"): muted ink that steps to `fg` on hover, no plate. */
        link: `text-muted hover:text-fg aria-expanded:text-fg ${DISABLED_BARE}`,
        neutral: `bg-surface-2 text-fg hover:bg-selected aria-expanded:bg-selected aria-pressed:bg-selected ${DISABLED_FILLED}`,
      },
      size: {
        default:
          "h-button-default gap-1.5 px-3 text-small-body has-data-[icon=inline-end]:pr-2.5 has-data-[icon=inline-start]:pl-2.5",
        xs: "h-button-xs gap-1 px-2 text-eyebrow has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-button-sm gap-1 px-2.5 text-meta has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2 [&_svg:not([class*='size-'])]:size-3.5",
        /** Toolbar pill level with PillGroup md segments (26 px, 12.5 px — the `sm` tier). */
        segment:
          "h-(--height-pill-group-segment-md) gap-1 px-2.5 text-meta has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2 [&_svg:not([class*='size-'])]:size-3.5",
        lg: "h-button-lg gap-1.5 px-3.5 text-body has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
        cta: "h-button-cta gap-2 px-4 text-body has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
        "cta-lg":
          "h-button-cta-lg gap-2 px-5 text-body has-data-[icon=inline-end]:pr-3.5 has-data-[icon=inline-start]:pl-3.5",
        icon: "size-button-icon-default",
        "icon-xs": "size-button-icon-xs [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "size-button-icon-sm [&_svg:not([class*='size-'])]:size-3.5",
        "icon-lg": "size-button-icon-lg",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

export { buttonVariants };
