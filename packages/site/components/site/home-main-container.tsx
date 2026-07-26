"use client";

import { cn } from "@agh/ui";
import type { ComponentProps } from "react";

export function HomeMainContainer({
  className,
  id = "main-content",
  ...props
}: ComponentProps<"main">) {
  return (
    <main
      id={id}
      {...props}
      className={cn(
        "site-home flex flex-1 flex-col bg-canvas [--fd-layout-width:1400px]",
        className
      )}
    />
  );
}
