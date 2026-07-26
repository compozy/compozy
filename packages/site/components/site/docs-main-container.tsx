"use client";

import { cn } from "@agh/ui";
import type { ComponentProps } from "react";

export function DocsMainContainer({
  className,
  id = "main-content",
  ...props
}: ComponentProps<"article">) {
  return (
    <main
      id={id}
      {...props}
      className={cn(
        "flex flex-col [grid-area:main] gap-4 px-4 py-6 md:px-6 md:pt-8 xl:px-8 xl:pt-14 *:max-w-225",
        className
      )}
    />
  );
}
