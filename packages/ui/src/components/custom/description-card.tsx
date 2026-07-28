"use client";

import type * as React from "react";

import { cn } from "../../lib/utils";
import { Surface } from "../surface";
import { Markdown } from "./markdown";

export interface DescriptionCardProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** Markdown source — operator-authored or model-streamed. */
  children: string;
  /** Render the card chrome — set `bare` to consume only the prose styles inline. */
  bare?: boolean;
}

/**
 * Operator-authored markdown card. Rounded `bg-canvas-soft` chrome around a
 * `<Markdown />` body so the same prose grammar is shared with every other
 * markdown surface in the runtime (chat messages, tool-call panels, knowledge
 * notes). Pass `bare` to drop the chrome and consume only the prose styles
 * inline.
 */
function DescriptionCard({ children, bare = false, className, ...props }: DescriptionCardProps) {
  if (bare) {
    return (
      <div
        data-slot="description-card"
        data-bare="true"
        className={cn("flex flex-col", className)}
        {...props}
      >
        <Markdown>{children}</Markdown>
      </div>
    );
  }
  return (
    <Surface data-slot="description-card" className={cn("flex flex-col", className)} {...props}>
      <Markdown>{children}</Markdown>
    </Surface>
  );
}

export { DescriptionCard };
