import type { ComponentProps } from "react";

import { useAttachmentRail } from "@/components/assistant-ui/hooks/use-attachment-rail";
import { cn } from "@/lib/utils";

import type { SessionAttachmentItem } from "../lib/session-attachment-items";
import { SessionAttachmentFileCard } from "./session-attachment-file-card";
import { SessionAttachmentFrame } from "./session-attachment-frame";

export interface SessionAttachmentGalleryProps extends ComponentProps<"div"> {
  items: readonly SessionAttachmentItem[];
}

function imageTitle(item: Extract<SessionAttachmentItem, { kind: "image" }>): string {
  if (item.width && item.height) {
    return `${item.filename} · ${item.width}×${item.height}`;
  }
  return item.filename;
}

export function SessionAttachmentGallery({
  items,
  className,
  ...props
}: SessionAttachmentGalleryProps) {
  const { overflow, railRef, trackRef } = useAttachmentRail(items.length);
  if (items.length === 0) {
    return null;
  }

  const showStart = overflow.includes("start");
  const showEnd = overflow.includes("end");

  return (
    <div
      ref={railRef}
      data-testid="user-message-attachment-gallery"
      data-overflow={overflow}
      className={cn(
        "relative isolate mb-1 min-w-0 self-end overflow-hidden [--att-fade:64px] [--att-fade-bg:var(--canvas)]",
        overflow === "none" ? "w-max max-w-full" : "w-full",
        className
      )}
      {...props}
    >
      <span
        aria-hidden="true"
        className={cn(
          "att-rail__edge att-rail__edge--start pointer-events-none absolute inset-y-0 left-0 z-2 w-[var(--att-fade)]",
          "bg-[linear-gradient(90deg,rgb(from_var(--att-fade-bg)_r_g_b_/_1)_0%,rgb(from_var(--att-fade-bg)_r_g_b_/_0.7)_32%,rgb(from_var(--att-fade-bg)_r_g_b_/_0)_100%)]",
          "opacity-0 transition-opacity duration-slow ease-out",
          showStart && "opacity-100"
        )}
      />
      <ul
        ref={node => {
          trackRef.current = node as typeof trackRef.current;
        }}
        aria-label="Attachments"
        className={cn(
          "att-rail__track m-0 flex list-none flex-nowrap items-start justify-end gap-1.5 overflow-x-auto overflow-y-hidden px-0 py-0.5",
          "[scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden",
          "overscroll-x-contain [scroll-padding-inline:var(--att-fade)] [touch-action:pan-x]"
        )}
      >
        {items.map(item => (
          <li key={item.id} className="shrink-0">
            {item.kind === "image" ? (
              <SessionAttachmentFrame
                href={item.href}
                src={item.src}
                filename={item.filename}
                title={imageTitle(item)}
                role="listitem"
              />
            ) : (
              <SessionAttachmentFileCard
                href={item.href}
                filename={item.filename}
                extension={item.extension}
                sizeLabel={item.sizeLabel}
                role="listitem"
              />
            )}
          </li>
        ))}
      </ul>
      <span
        aria-hidden="true"
        className={cn(
          "att-rail__edge att-rail__edge--end pointer-events-none absolute inset-y-0 right-0 z-2 w-[var(--att-fade)]",
          "bg-[linear-gradient(270deg,rgb(from_var(--att-fade-bg)_r_g_b_/_1)_0%,rgb(from_var(--att-fade-bg)_r_g_b_/_0.7)_32%,rgb(from_var(--att-fade-bg)_r_g_b_/_0)_100%)]",
          "opacity-0 transition-opacity duration-slow ease-out",
          showEnd && "opacity-100"
        )}
      />
    </div>
  );
}
