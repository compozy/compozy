import type { Attachment } from "@assistant-ui/react";

import { isImageAttachmentMime } from "@/systems/session/lib/attachment-kinds";

import { sessionAttachmentTileState } from "../session-attachment-tile-model";

export function sessionComposerSendBlocker({
  attachments,
  promptImage,
}: {
  attachments: readonly Attachment[];
  promptImage: boolean;
}): string | null {
  const uploading = attachments.find(
    attachment => sessionAttachmentTileState(attachment) === "uploading"
  );
  if (uploading) return `Saving ${uploading.name}`;

  const rejected = attachments.some(
    attachment => sessionAttachmentTileState(attachment) === "rejected"
  );
  if (rejected) return "Remove files that are not supported";

  const persistFailed = attachments.find(attachment => {
    if (sessionAttachmentTileState(attachment) !== "error") return false;
    return attachment.status.type === "incomplete" && attachment.status.message === "Couldn't save";
  });
  if (persistFailed) return `Couldn't save ${persistFailed.name}`;

  const oversized = attachments.some(attachment => {
    if (sessionAttachmentTileState(attachment) !== "error") return false;
    return (
      attachment.status.type === "incomplete" &&
      (attachment.status.message?.includes("over 10 MB") ?? false)
    );
  });
  if (oversized) return "A file is over 10 MB";

  const hasImages = attachments.some(
    attachment => attachment.type === "image" || isImageAttachmentMime(attachment.contentType)
  );
  if (hasImages && !promptImage) return "This model does not accept images";

  const notReady = attachments.find(
    attachment => sessionAttachmentTileState(attachment) !== "ready"
  );
  if (notReady) return `Saving ${notReady.name}`;

  return null;
}
