import { Fragment } from "react";

import { cn } from "@/lib/utils";

import { highlightSegments, type CharRange } from "../lib/byte-ranges";

/** Text with its matched ranges in `<mark>` (brand tint, foreground kept). */
export function HighlightedText({
  text,
  ranges,
  className,
}: {
  text: string;
  ranges: readonly CharRange[];
  className?: string;
}) {
  return (
    <span className={className}>
      {highlightSegments(text, ranges).map((segment, index) =>
        segment.match ? (
          <mark
            key={index}
            data-slot="match"
            className={cn("rounded-[2px] bg-brand-tint text-inherit")}
          >
            {segment.text}
          </mark>
        ) : (
          <Fragment key={index}>{segment.text}</Fragment>
        )
      )}
    </span>
  );
}
