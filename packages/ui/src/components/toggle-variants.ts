import { cva } from "class-variance-authority";

const toggleVariants = cva(
  "group/toggle inline-flex items-center justify-center gap-1 rounded-md text-form-label font-medium tracking-eyebrow whitespace-nowrap transition-[color,background-color,border-color,box-shadow,opacity] duration-fast ease-out outline-none hover:bg-btn-default-hover hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-danger aria-pressed:bg-elevated aria-pressed:text-fg-strong aria-pressed:shadow-highlight data-[state=on]:bg-elevated data-[state=on]:text-fg-strong data-[state=on]:shadow-highlight [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: "bg-transparent text-muted",
        outline: "border border-line bg-transparent text-fg hover:bg-btn-default-hover",
      },
      size: {
        default:
          "h-button-default min-w-(--height-button-default) px-2.5 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
        sm: "h-button-sm min-w-(--height-button-sm) px-2.5 text-eyebrow has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        lg: "h-button-lg min-w-(--height-button-lg) px-3 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

export { toggleVariants };
