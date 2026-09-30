import { cva } from "class-variance-authority";

export const filtersContainerVariants = cva("flex flex-wrap items-center", {
  variants: {
    variant: {
      solid: "gap-2",
      default: "",
    },
    size: {
      sm: "gap-1.5",
      default: "gap-2.5",
      lg: "gap-3.5",
    },
  },
  defaultVariants: {
    variant: "default",
    size: "default",
  },
});

type FilterChipSize = "sm" | "default" | "lg";

/**
 * Chip segments are `neutral` pill segments that paint under their transparent
 * border box (`bg-clip-border`), so a chip reads as one continuous pill with no
 * canvas showing between segments. `default` chips sit at the PillGroup md
 * geometry (26 px, 12.5 px); `sm` and `lg` keep the button ladder.
 */
export const FILTER_CHIP_SEGMENT_CLASS: Record<FilterChipSize, string> = {
  sm: "bg-clip-border",
  default: "h-(--height-pill-group-segment-md)! bg-clip-border text-meta",
  lg: "bg-clip-border",
};

export const FILTER_CHIP_ICON_CLASS: Record<FilterChipSize, string> = {
  sm: "bg-clip-border",
  default: "size-(--height-pill-group-segment-md)! bg-clip-border",
  lg: "bg-clip-border",
};
