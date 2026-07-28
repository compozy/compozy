import { Separator as SeparatorPrimitive } from "@base-ui/react/separator";
import type * as React from "react";

import { cn } from "../lib/utils";

export interface SeparatorProps extends Omit<SeparatorPrimitive.Props, "style"> {
  label?: React.ReactNode;
  labelClassName?: string;
  lineClassName?: string;
  style?: React.CSSProperties;
  tone?: "default" | "accent";
}

function Separator({
  className,
  orientation = "horizontal",
  label,
  labelClassName,
  lineClassName,
  style,
  tone = "default",
  ...props
}: SeparatorProps) {
  if (label) {
    return (
      <div
        data-slot="separator"
        data-orientation={orientation}
        data-tone={tone}
        role="separator"
        aria-orientation={orientation}
        className={cn(
          "flex shrink-0 items-center gap-3 data-vertical:flex-col data-vertical:self-stretch",
          className
        )}
        style={style}
        {...props}
      >
        <SeparatorPrimitive
          aria-hidden="true"
          orientation={orientation}
          className={cn(
            "shrink-0 data-horizontal:h-px data-horizontal:flex-1 data-vertical:h-full data-vertical:w-px",
            tone === "accent" ? "bg-accent" : "bg-line",
            lineClassName
          )}
        />
        <span
          data-slot="separator-label"
          className={cn(
            "eyebrow shrink-0",
            tone === "accent" ? "text-accent" : "text-subtle",
            labelClassName
          )}
        >
          {label}
        </span>
        <SeparatorPrimitive
          aria-hidden="true"
          orientation={orientation}
          className={cn(
            "shrink-0 data-horizontal:h-px data-horizontal:flex-1 data-vertical:h-full data-vertical:w-px",
            tone === "accent" ? "bg-accent" : "bg-line",
            lineClassName
          )}
        />
      </div>
    );
  }

  return (
    <SeparatorPrimitive
      data-slot="separator"
      orientation={orientation}
      className={cn(
        "shrink-0 bg-line data-horizontal:h-px data-horizontal:w-full data-vertical:w-px data-vertical:self-stretch",
        className
      )}
      {...props}
    />
  );
}

export { Separator };
