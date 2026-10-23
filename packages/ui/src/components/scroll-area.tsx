import { ScrollArea as ScrollAreaPrimitive } from "@base-ui/react/scroll-area";
import { useReducedMotionConfig } from "motion/react";

import { cn } from "../lib/utils";

interface ScrollAreaProps extends ScrollAreaPrimitive.Root.Props {
  viewportClassName?: string;
}

/**
 * Overlay scroll region. The bar floats over the viewport and shows only while
 * the pointer is over the area, while it scrolls, or while the viewport holds
 * keyboard focus. Vertical overflow reserves an inline-end gutter (it follows
 * the bar in RTL); horizontal overflow reserves a bottom gutter only while a
 * horizontal bar is mounted. Either way the bar never covers content.
 */
function ScrollArea({ className, viewportClassName, children, ...props }: ScrollAreaProps) {
  return (
    <ScrollAreaPrimitive.Root
      data-slot="scroll-area"
      className={cn("group/scroll-area relative", className)}
      {...props}
    >
      <ScrollAreaPrimitive.Viewport
        data-slot="scroll-area-viewport"
        className={cn(
          "size-full rounded-[inherit] transition-[color,box-shadow] outline-none focus-visible:outline-none focus-visible:shadow-focus-ring",
          "data-has-overflow-y:pe-3",
          // A consumer-added horizontal bar mounts inside the viewport.
          "has-[>[data-slot=scroll-area-scrollbar][data-orientation=horizontal]]:data-has-overflow-x:pb-3",
          viewportClassName
        )}
      >
        {children}
      </ScrollAreaPrimitive.Viewport>
      <ScrollBar />
      <ScrollAreaPrimitive.Corner />
    </ScrollAreaPrimitive.Root>
  );
}

function ScrollBar({
  className,
  orientation = "vertical",
  ...props
}: ScrollAreaPrimitive.Scrollbar.Props) {
  const reduced = useReducedMotionConfig();
  return (
    <ScrollAreaPrimitive.Scrollbar
      data-slot="scroll-area-scrollbar"
      data-orientation={orientation}
      orientation={orientation}
      className={cn(
        "flex touch-none p-0.5 opacity-0 select-none data-horizontal:h-2.5 data-horizontal:flex-col data-vertical:h-full data-vertical:w-2.5",
        reduced ? "transition-none" : "transition-opacity delay-300 duration-base",
        "data-hovering:opacity-100 data-hovering:delay-0 data-scrolling:opacity-100 data-scrolling:delay-0",
        "group-has-[>[data-slot=scroll-area-viewport]:focus-visible]/scroll-area:opacity-100 group-has-[>[data-slot=scroll-area-viewport]:focus-visible]/scroll-area:delay-0",
        className
      )}
      {...props}
    >
      <ScrollAreaPrimitive.Thumb
        data-slot="scroll-area-thumb"
        className={cn(
          "relative flex-1 rounded-full bg-line hover:bg-line-strong",
          reduced ? "transition-none" : "transition-colors"
        )}
      />
    </ScrollAreaPrimitive.Scrollbar>
  );
}

export { ScrollArea, ScrollBar };
