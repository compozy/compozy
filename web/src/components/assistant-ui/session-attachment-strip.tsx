import { useAui, useAuiState } from "@assistant-ui/react";
import type { Attachment } from "@assistant-ui/react";
import { TriangleAlert } from "lucide-react";

import { cn } from "@/lib/utils";
import { isImageAttachmentMime } from "@/systems/session/lib/attachment-kinds";
import { Button } from "@compozy/ui";

import { useAttachmentRail } from "./hooks/use-attachment-rail";
import { SessionAttachmentTile } from "./session-attachment-tile";
import { sessionAttachmentTileModel } from "./session-attachment-tile-model";

export function SessionAttachmentCapabilityGate({
  onRemoveImages,
}: {
  onRemoveImages: () => void;
}) {
  return (
    <div
      role="status"
      data-testid="composer-attachment-gate"
      className="flex min-h-7 w-full flex-wrap items-center gap-2 text-[12px] text-warning"
    >
      <TriangleAlert className="size-[13px] shrink-0 text-warning" />
      <span>
        <b className="font-medium text-warning">This model does not accept images.</b> Remove them
        or pick another model.
      </span>
      <Button type="button" variant="ghost" size="sm" onClick={onRemoveImages} className="ml-auto">
        Remove
      </Button>
    </div>
  );
}

function isImageTile(attachment: Attachment): boolean {
  return attachment.type === "image" || isImageAttachmentMime(attachment.contentType);
}

export function SessionAttachmentStrip({ promptImage }: { promptImage: boolean }) {
  const aui = useAui();
  const attachments = useAuiState(state => state.composer.attachments);
  const { overflow, railRef, trackRef } = useAttachmentRail(attachments.length);
  const imageAttachments = attachments.filter(isImageTile);
  const showGate = !promptImage && imageAttachments.length > 0;

  if (attachments.length === 0) return null;

  const removeImages = () => {
    for (const attachment of imageAttachments) {
      void aui.composer.attachment({ id: attachment.id }).remove();
    }
  };

  return (
    <div data-testid="composer-attachment-strip" className="flex min-w-0 flex-col gap-1.5">
      {showGate ? <SessionAttachmentCapabilityGate onRemoveImages={removeImages} /> : null}
      <div
        ref={railRef}
        data-overflow={overflow}
        className="relative min-w-0 isolate [--att-fade:56px] [--att-fade-bg:var(--elevated)]"
      >
        <span
          aria-hidden="true"
          className={cn(
            "pointer-events-none absolute inset-y-0 left-0 z-2 w-[var(--att-fade)] opacity-0 transition-opacity duration-slow ease-out",
            "bg-[linear-gradient(90deg,rgb(from_var(--att-fade-bg)_r_g_b_/_1)_0%,rgb(from_var(--att-fade-bg)_r_g_b_/_0.7)_32%,rgb(from_var(--att-fade-bg)_r_g_b_/_0)_100%)]",
            overflow.includes("start") ? "opacity-100" : null
          )}
        />
        <ul
          ref={trackRef}
          aria-label="Attachments"
          className="m-0 flex list-none flex-nowrap items-start gap-2 overflow-x-auto overflow-y-hidden p-0 py-0.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        >
          {attachments.map(attachment => {
            const model = sessionAttachmentTileModel(attachment);
            return (
              <SessionAttachmentTile
                key={attachment.id}
                model={model}
                onRemove={() => {
                  void aui.composer.attachment({ id: attachment.id }).remove();
                }}
                onRetry={
                  model.retryable
                    ? () => {
                        const file = attachment.file;
                        void aui.composer.attachment({ id: attachment.id }).remove();
                        if (file) void aui.composer.addAttachment(file);
                      }
                    : undefined
                }
              />
            );
          })}
        </ul>
        <span
          aria-hidden="true"
          className={cn(
            "pointer-events-none absolute inset-y-0 right-0 z-2 w-[var(--att-fade)] opacity-0 transition-opacity duration-slow ease-out",
            "bg-[linear-gradient(270deg,rgb(from_var(--att-fade-bg)_r_g_b_/_1)_0%,rgb(from_var(--att-fade-bg)_r_g_b_/_0.7)_32%,rgb(from_var(--att-fade-bg)_r_g_b_/_0)_100%)]",
            overflow.includes("end") ? "opacity-100" : null
          )}
        />
      </div>
    </div>
  );
}
