import type { Attachment } from "@assistant-ui/react";

import { isImageAttachmentMime } from "@/systems/session/lib/attachment-kinds";
import type { SessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";

import { sessionAttachmentTileState } from "../session-attachment-tile-model";

export function sessionComposerSendBlocker({
  attachments,
  promptEmbeddedContextCapability,
  promptImageCapability,
}: {
  attachments: readonly Attachment[];
  promptEmbeddedContextCapability: SessionPromptCapability;
  promptImageCapability: SessionPromptCapability;
}): string | null {
  const uploading = attachments.find(
    attachment => sessionAttachmentTileState(attachment) === "uploading"
  );
  if (uploading) return `Saving ${uploading.name}`;

  const rejected = attachments.some(
    attachment => sessionAttachmentTileState(attachment) === "rejected"
  );
  if (rejected) return "Remove files that are not supported";

  const failed = attachments.find(attachment => sessionAttachmentTileState(attachment) === "error");
  if (failed?.status.type === "incomplete") {
    return failed.status.message ?? `Couldn't save ${failed.name}`;
  }

  const hasImages = attachments.some(
    attachment => attachment.type === "image" || isImageAttachmentMime(attachment.contentType)
  );
  if (hasImages && promptImageCapability === "unsupported") {
    return "This agent does not accept images";
  }

  const hasPDF = attachments.some(attachment => attachment.contentType === "application/pdf");
  if (hasPDF && promptEmbeddedContextCapability === "unsupported") {
    return "This agent does not accept PDF files";
  }

  const notReady = attachments.find(
    attachment => sessionAttachmentTileState(attachment) !== "ready"
  );
  if (notReady) return `Saving ${notReady.name}`;

  return null;
}
