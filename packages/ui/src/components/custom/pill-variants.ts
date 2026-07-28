import { cva } from "class-variance-authority";

const pillVariants = cva(
  "inline-flex w-fit shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-xs transition-colors duration-base ease-out focus-visible:outline-none focus-visible:shadow-focus-ring disabled:cursor-not-allowed disabled:opacity-50 [&>svg]:pointer-events-none [&>svg]:size-3",
  {
    variants: {
      tone: {
        neutral: "bg-neutral-tint text-neutral-ink",
        accent: "bg-accent-tint text-accent",
        success: "bg-success-tint text-success",
        warning: "bg-warning-tint text-warning",
        danger: "bg-danger-tint text-danger",
        info: "bg-info-tint text-info",
      },
      size: {
        xs: "h-pill-xs px-1.5 leading-none",
        sm: "h-pill-sm px-2 leading-none",
        md: "h-pill-md px-2.5 leading-none",
      },
      mono: {
        true: "font-mono",
        false: "font-sans",
      },
      solid: { true: "", false: "" },
      active: { true: "", false: "" },
    },
    compoundVariants: [
      { mono: true, size: "xs", className: "text-mono-id font-semibold tracking-mono-id" },
      { mono: true, size: "sm", className: "text-mono-id font-semibold tracking-mono-id" },
      { mono: true, size: "md", className: "text-mono-id font-semibold tracking-mono-id" },
      { mono: false, size: "xs", className: "text-eyebrow font-medium tracking-eyebrow" },
      { mono: false, size: "sm", className: "text-eyebrow font-medium tracking-eyebrow" },
      { mono: false, size: "md", className: "text-eyebrow font-medium tracking-eyebrow" },
      { solid: true, tone: "neutral", className: "bg-muted text-canvas" },
      { solid: true, tone: "accent", className: "bg-accent text-accent-ink" },
      { solid: true, tone: "success", className: "bg-success text-canvas" },
      { solid: true, tone: "warning", className: "bg-warning text-canvas" },
      { solid: true, tone: "danger", className: "bg-danger text-canvas" },
      { solid: true, tone: "info", className: "bg-info text-canvas" },
      {
        active: true,
        className: "bg-elevated text-fg-strong",
      },
    ],
    defaultVariants: {
      tone: "neutral",
      size: "sm",
      mono: false,
      solid: false,
    },
  }
);

export { pillVariants };
