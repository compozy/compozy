import { cva } from "class-variance-authority";

/**
 * Restrained operator alerts: semantic tint + hairline border, with the signal
 * repeated on the leading icon. Titles stay fg-strong; descriptions stay muted.
 * Matches web/CLAUDE.md — signal palette is information, never solid semantic banners.
 */
const alertVariants = cva(
  [
    "group/alert relative grid w-full gap-x-2.5 gap-y-2 rounded-lg border border-transparent bg-surface-2",
    "px-3.5 py-3 text-left text-small-body text-fg",
    "has-data-[slot=alert-action]:relative has-data-[slot=alert-action]:pr-18",
    "has-[>svg]:grid-cols-[auto_1fr]",
    "*:data-[slot=alert-title]:text-fg",
    "*:data-[slot=alert-description]:text-muted",
    "*:[svg]:row-span-full *:[svg]:translate-y-0.5 *:[svg:not([class*='size-'])]:size-3.5",
  ].join(" "),
  {
    variants: {
      variant: {
        default: "[&>svg]:text-subtle",
        neutral: "[&>svg]:text-subtle",
        danger: "[&>svg]:text-danger",
        warning: "bg-warning-tint [&>svg]:text-warning",
        success: "bg-success-tint [&>svg]:text-success",
        info: "bg-info-tint [&>svg]:text-info",
        accent: "bg-accent-tint [&>svg]:text-accent",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
);

export { alertVariants };
