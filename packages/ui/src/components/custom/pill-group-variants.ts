import { cva } from "class-variance-authority";

const pillGroupSegmentVariants = cva(
  "inline-flex cursor-pointer items-center justify-center gap-1.5 whitespace-nowrap rounded-xs text-form-label font-medium leading-none tracking-eyebrow transition-colors duration-base ease-out focus-visible:outline-none focus-visible:shadow-focus-ring disabled:cursor-not-allowed disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-3.5",
  {
    variants: {
      active: {
        true: "bg-elevated text-fg-strong shadow-highlight",
        false: "bg-transparent text-subtle hover:text-muted",
      },
      size: {
        sm: "min-h-(--height-pill-group-segment-sm) px-(--space-pill-group-segment-sm-x)",
        md: "min-h-(--height-pill-group-segment-md) px-(--space-pill-group-segment-md-x)",
      },
    },
    defaultVariants: {
      active: false,
      size: "md",
    },
  }
);

export { pillGroupSegmentVariants };
