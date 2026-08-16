import type { ComponentProps, Ref } from "react";

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

function assignRef<T>(ref: Ref<T> | undefined, value: T | null): void {
  if (typeof ref === "function") {
    ref(value);
    return;
  }
  if (ref) ref.current = value;
}

export function SessionAttachmentGallery({
  items,
  className,
  ref: externalRef,
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
      ref={node => {
        railRef.current = node;
        assignRef(externalRef, node);
      }}
      data-testid="user-message-attachment-gallery"
      data-overflow={overflow}
      className={cn(
        "relative isolate mb-1 min-w-0 self-end overflow-hidden",
        "[--att-fade:64px]",
        overflow === "none" ? "w-max max-w-full" : "w-full",
        className
      )}
      {...props}
    >
      <span
        aria-hidden="true"
        className={cn(
          "att-rail__edge att-rail__edge--start pointer-events-none absolute",
          "inset-y-0 start-0 z-2 w-16",
          "bg-linear-to-r from-canvas via-canvas/70 to-transparent",
          "rtl:bg-linear-to-l",
          "opacity-0 transition-opacity duration-slow ease-out",
          showStart && "opacity-100"
        )}
      />
      <ul
        ref={trackRef}
        aria-label="Attachments"
        className={cn(
          "att-rail__track m-0 flex list-none flex-nowrap",
          "items-start justify-end gap-1.5",
          "overflow-x-auto overflow-y-hidden px-0 py-0.5",
          "overscroll-x-contain [scroll-padding-inline:var(--att-fade)]",
          "[scrollbar-width:none] [-ms-overflow-style:none]",
          "[touch-action:pan-x] [&::-webkit-scrollbar]:hidden"
        )}
      >
        {items.map((item, occurrence) => (
          <li key={`${item.id}:${occurrence}`} data-attachment-rail-item className="shrink-0">
            {item.kind === "image" ? (
              <SessionAttachmentFrame
                href={item.href}
                src={item.src}
                filename={item.filename}
                title={imageTitle(item)}
                width={item.width}
                height={item.height}
              />
            ) : (
              <SessionAttachmentFileCard
                href={item.href}
                filename={item.filename}
                extension={item.extension}
                sizeLabel={item.sizeLabel}
              />
            )}
          </li>
        ))}
      </ul>
      <span
        aria-hidden="true"
        className={cn(
          "att-rail__edge att-rail__edge--end pointer-events-none absolute",
          "inset-y-0 end-0 z-2 w-16",
          "bg-linear-to-l from-canvas via-canvas/70 to-transparent",
          "rtl:bg-linear-to-r",
          "opacity-0 transition-opacity duration-slow ease-out",
          showEnd && "opacity-100"
        )}
      />
    </div>
  );
}
