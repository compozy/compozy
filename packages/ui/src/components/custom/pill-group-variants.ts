import { cva } from "class-variance-authority";

const pillGroupSegmentVariants = cva(
  "inline-flex cursor-pointer items-center justify-center gap-1.5 whitespace-nowrap rounded-pill font-medium leading-none transition-colors duration-base ease-out focus-visible:outline-none focus-visible:shadow-focus-ring disabled:cursor-not-allowed disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-3.5",
  {
    variants: {
      active: {
        true: "bg-surface-2 text-fg",
        false: "bg-transparent text-muted hover:bg-surface-2 hover:text-fg",
      },
      size: {
        sm: "min-h-(--height-pill-group-segment-sm) px-(--space-pill-group-segment-sm-x) text-eyebrow",
        md: "min-h-(--height-pill-group-segment-md) px-(--space-pill-group-segment-md-x) text-meta",
      },
    },
    defaultVariants: {
      active: false,
      size: "md",
    },
  }
);

export { pillGroupSegmentVariants };
