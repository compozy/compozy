import { cva } from "class-variance-authority";

const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center rounded-pill border border-transparent bg-clip-padding font-sans text-body font-medium whitespace-nowrap transition-[color,background-color,border-color,box-shadow,opacity,transform] duration-fast ease-out outline-none select-none focus-visible:outline-none focus-visible:shadow-focus-ring active:not-aria-[haspopup]:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-danger [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground hover:bg-primary-hover aria-expanded:bg-primary-hover [a]:hover:bg-primary-hover",
        primary:
          "bg-primary text-primary-foreground hover:bg-primary-hover aria-expanded:bg-primary-hover [a]:hover:bg-primary-hover",
        outline: "border-line bg-transparent text-fg hover:bg-surface-2 aria-expanded:bg-surface-2",
        secondary:
          "bg-surface-2 text-fg hover:bg-selected hover:shadow-card aria-expanded:bg-selected",
        ghost: "text-fg hover:bg-surface-2 aria-expanded:bg-surface-2 aria-expanded:text-fg",
        destructive: "bg-danger-tint text-danger hover:bg-danger-tint hover:opacity-90",
        "destructive-solid":
          "bg-danger text-accent-ink hover:bg-[color-mix(in_srgb,var(--color-danger)_88%,black)]",
        success: "bg-success-tint text-success hover:opacity-90",
        link: "text-fg-2 underline-offset-4 hover:text-fg hover:underline",
        neutral:
          "bg-surface-2 text-fg hover:bg-selected hover:shadow-card aria-expanded:bg-selected",
      },
      size: {
        default:
          "h-button-default gap-1.5 px-2.5 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
        xs: "h-button-xs gap-1 px-2 text-eyebrow has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-button-sm gap-1 px-2.5 text-eyebrow has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        lg: "h-button-lg gap-1.5 px-3 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
        cta: "h-button-cta gap-2 px-5 has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
        "cta-lg":
          "h-button-cta-lg gap-2 px-5 has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
        icon: "size-button-icon-default",
        "icon-xs": "size-button-icon-xs [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "size-button-icon-sm",
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
