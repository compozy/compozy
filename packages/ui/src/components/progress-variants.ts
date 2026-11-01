import { cva } from "class-variance-authority";

const progressIndicatorVariants = cva("h-full transition-[width] duration-slow ease-out", {
  variants: {
    tone: {
      brand: "bg-brand",
      success: "bg-success",
      warning: "bg-warning",
      destructive: "bg-destructive",
      info: "bg-info",
      neutral: "bg-neutral",
    },
  },
  defaultVariants: {
    tone: "brand",
  },
});

export { progressIndicatorVariants };
