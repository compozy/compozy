import { cva } from "class-variance-authority";

const progressIndicatorVariants = cva("h-full transition-[width] duration-slow ease-out", {
  variants: {
    tone: {
      accent: "bg-accent",
      success: "bg-success",
      warning: "bg-warning",
      danger: "bg-danger",
      info: "bg-info",
      neutral: "bg-neutral",
    },
  },
  defaultVariants: {
    tone: "accent",
  },
});

export { progressIndicatorVariants };
