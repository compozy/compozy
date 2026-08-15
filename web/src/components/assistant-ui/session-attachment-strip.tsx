import { useAui, useAuiState } from "@assistant-ui/react";
import type { Attachment } from "@assistant-ui/react";
import { TriangleAlert } from "lucide-react";

import { cn } from "@/lib/utils";
import { isImageAttachmentMime } from "@/systems/session/lib/attachment-kinds";
import type { SessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";
import { Button } from "@compozy/ui";

import { useAttachmentRail } from "./hooks/use-attachment-rail";
import { SessionAttachmentTile } from "./session-attachment-tile";
import { sessionAttachmentTileModel } from "./session-attachment-tile-model";

export function SessionAttachmentCapabilityGate({
  message,
  onRemove,
}: {
  message: string;
  onRemove: () => void;
}) {
  return (
    <div
      role="status"
      data-testid="composer-attachment-gate"
      className="flex min-h-7 w-full flex-wrap items-center gap-2 text-small-body text-warning"
    >
      <TriangleAlert className="size-3.5 shrink-0 text-warning" />
      <span>
        <b className="font-medium text-warning">{message}</b>
      </span>
      <Button type="button" variant="ghost" size="sm" onClick={onRemove} className="ms-auto">
        Remove unsupported files
      </Button>
    </div>
  );
}

function isImageTile(attachment: Attachment): boolean {
  return attachment.type === "image" || isImageAttachmentMime(attachment.contentType);
}

export function SessionAttachmentStrip({
  promptEmbeddedContextCapability,
  promptImageCapability,
}: {
  promptEmbeddedContextCapability: SessionPromptCapability;
  promptImageCapability: SessionPromptCapability;
}) {
  const aui = useAui();
  const attachments = useAuiState(state => state.composer.attachments);
  const { overflow, railRef, trackRef } = useAttachmentRail(attachments.length);
  const imageAttachments = attachments.filter(isImageTile);
  const pdfAttachments = attachments.filter(
    attachment => attachment.contentType === "application/pdf"
  );
  const blockedAttachments = [
    ...(promptImageCapability === "unsupported" ? imageAttachments : []),
    ...(promptEmbeddedContextCapability === "unsupported" ? pdfAttachments : []),
  ];
  const blockedKinds = [
    ...(promptImageCapability === "unsupported" && imageAttachments.length > 0 ? ["images"] : []),
    ...(promptEmbeddedContextCapability === "unsupported" && pdfAttachments.length > 0
      ? ["PDF files"]
      : []),
  ];
  const gateMessage =
    blockedKinds.length > 0 ? `This agent does not accept ${blockedKinds.join(" or ")}.` : null;

  if (attachments.length === 0) return null;

  const removeBlockedAttachments = () => {
    for (const attachment of blockedAttachments) {
      void aui.composer.attachment({ id: attachment.id }).remove();
    }
  };

  return (
    <div data-testid="composer-attachment-strip" className="flex min-w-0 flex-col gap-1.5">
      {gateMessage ? (
        <SessionAttachmentCapabilityGate
          message={gateMessage}
          onRemove={removeBlockedAttachments}
        />
      ) : null}
      <div
        ref={railRef}
        data-overflow={overflow}
        className="relative min-w-0 isolate [--att-fade:56px]"
      >
        <span
          aria-hidden="true"
          className={cn(
            "pointer-events-none absolute inset-y-0 start-0 z-2 w-14",
            "opacity-0 transition-opacity duration-slow ease-out",
            "bg-linear-to-r from-elevated via-elevated/70 to-transparent",
            "rtl:bg-linear-to-l",
            overflow.includes("start") ? "opacity-100" : null
          )}
        />
        <ul
          ref={trackRef}
          aria-label="Attachments"
          className={cn(
            "m-0 flex list-none flex-nowrap items-start gap-2 p-0 py-0.5",
            "overflow-x-auto overflow-y-hidden [scrollbar-width:none]",
            "[&::-webkit-scrollbar]:hidden"
          )}
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
            "pointer-events-none absolute inset-y-0 end-0 z-2 w-14",
            "opacity-0 transition-opacity duration-slow ease-out",
            "bg-linear-to-l from-elevated via-elevated/70 to-transparent",
            "rtl:bg-linear-to-r",
            overflow.includes("end") ? "opacity-100" : null
          )}
        />
      </div>
    </div>
  );
}
