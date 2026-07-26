import { cva } from "class-variance-authority";

const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center rounded-md border border-transparent bg-clip-padding font-sans text-form-label font-medium tracking-eyebrow whitespace-nowrap transition-[color,background-color,border-color,box-shadow,opacity,transform] duration-fast ease-out outline-none select-none focus-visible:outline-none focus-visible:shadow-focus-ring active:not-aria-[haspopup]:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-danger [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default:
          "bg-accent text-accent-ink shadow-highlight hover:bg-accent-hover [a]:hover:bg-accent-hover",
        primary:
          "bg-accent text-accent-ink shadow-highlight hover:bg-accent-hover [a]:hover:bg-accent-hover",
        outline:
          "border-line bg-transparent text-fg hover:bg-btn-default-hover aria-expanded:bg-btn-default-hover",
        secondary:
          "bg-canvas-tint border-line text-fg hover:bg-btn-default-hover aria-expanded:bg-btn-default-hover",
        ghost:
          "text-fg hover:bg-btn-default-hover aria-expanded:bg-btn-default-hover aria-expanded:text-fg",
        destructive: "bg-danger-tint text-danger hover:bg-danger-tint hover:opacity-90",
        success: "bg-success-tint text-success hover:opacity-90",
        link: "text-accent underline-offset-4 hover:underline",
        neutral: "bg-btn-default-fill text-fg-strong hover:bg-btn-default-hover",
      },
      size: {
        default:
          "h-button-default gap-1.5 px-2.5 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
        xs: "h-button-xs gap-1 rounded-md px-2 text-eyebrow has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-button-sm gap-1 rounded-md px-2.5 text-eyebrow has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        lg: "h-button-lg gap-1.5 px-3 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
        cta: "h-button-cta gap-2 px-5 has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
        "cta-lg":
          "h-button-cta-lg gap-2 rounded-md px-5 text-small-body has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
        icon: "size-button-icon-default",
        "icon-xs": "size-button-icon-xs rounded-md [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "size-button-icon-sm rounded-md",
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
