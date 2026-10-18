import { mergeProps } from "@base-ui/react/merge-props";
import { useRender } from "@base-ui/react/use-render";

import { cn } from "../lib/utils";

export type SurfaceSize = "default" | "compact" | "flush";
export type SurfaceVariant = "card" | "sunken";

export interface SurfaceProps extends useRender.ComponentProps<"div"> {
  /**
   * `default` = px-5 py-4 card padding; `compact` = px-4 py-3 for dense tiles;
   * `flush` = no padding, for containers that pad their own head/body/foot.
   */
  size?: SurfaceSize;
  /**
   * `card` (default) lifts content one step above the pane: `bg-card` plus
   * `shadow-card`, whose first layer is the `line` hairline. `sunken` is the
   * recessed inset for secondary lists (tool rows, command previews): no
   * border and no shadow on `--sunken`.
   */
  variant?: SurfaceVariant;
}

const SIZE_CLASS: Record<SurfaceSize, string | undefined> = {
  default: "px-5 py-4",
  compact: "px-4 py-3",
  flush: undefined,
};

/**
 * The canonical tile at `rounded-lg`, one of the two surface levels. Card-like
 * surfaces (Panel, StatusCard, RunCard, DescriptionCard, Metric) compose this
 * instead of re-copying the surface tuple; `render` swaps the element (Panel is
 * a `section`). Content layout (flex, gap) stays with the composing consumer.
 */
function Surface({
  size = "default",
  variant = "card",
  className,
  render,
  ...props
}: SurfaceProps) {
  return useRender({
    defaultTagName: "div",
    render,
    props: mergeProps<"div">(
      {
        className: cn(
          "rounded-lg",
          variant === "sunken" ? "bg-sunken" : "bg-card shadow-card",
          SIZE_CLASS[size],
          className
        ),
      },
      { "data-slot": "surface", "data-size": size, "data-variant": variant } as Record<
        string,
        unknown
      >,
      props
    ),
    state: { slot: "surface", size, variant },
  });
}

export { Surface };
