"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export interface DockTitleProps extends React.ComponentProps<"span"> {
  /**
   * Keep the author's line breaks and wrap long words — for agent-written
   * prose such as a clarification question. Single-line titles (permission
   * subjects) leave it off.
   */
  multiline?: boolean;
}

function DockTitle({ children, className, multiline = false, ...props }: DockTitleProps) {
  return (
    <span
      data-slot="dock-title"
      data-multiline={multiline || undefined}
      className={cn(
        "text-item-title font-medium text-fg",
        multiline && "min-w-0 break-words whitespace-pre-line",
        className
      )}
      {...props}
    >
      {children}
    </span>
  );
}

export { DockTitle };
