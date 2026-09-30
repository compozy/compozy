import { cva } from "class-variance-authority";

// Pressed = the hover plate held down (`surface-2` + full ink), never a ring
// or outline. Geometry mirrors the button ladder: default 30 / 13 / 16 glyph ·
// sm 26 / 12.5 / 14 · lg 34 / 13.5 / 16.
const toggleVariants = cva(
  "group/toggle inline-flex items-center justify-center gap-1 rounded-pill font-medium whitespace-nowrap transition-[color,background-color,border-color,opacity] duration-fast ease-out outline-none hover:bg-surface-2 hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-danger aria-pressed:bg-surface-2 aria-pressed:text-fg data-[state=on]:bg-surface-2 data-[state=on]:text-fg [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: "bg-transparent text-muted",
        outline: "border border-line bg-transparent text-fg hover:bg-surface-2",
      },
      size: {
        default:
          "h-button-default min-w-(--height-button-default) px-2.5 text-small-body has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
        sm: "h-button-sm min-w-(--height-button-sm) px-2 text-meta has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3.5",
        lg: "h-button-lg min-w-(--height-button-lg) px-3 text-body has-data-[icon=inline-end]:pr-2.5 has-data-[icon=inline-start]:pl-2.5",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

export { toggleVariants };
