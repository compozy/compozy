import type { SessionInputPayload } from "../types";
import { attachmentExtensionMark, isImageAttachmentMime } from "./attachment-kinds";
import { sessionAttachmentBytesURL } from "./attachment-url";

export type QueuedPromptAttachmentPreview =
  | { kind: "image"; url: string }
  | { kind: "file"; mark: string };

export interface QueuedPromptAttachmentSummary {
  fileCount: number;
  imageCount: number;
  preview?: QueuedPromptAttachmentPreview;
}

export interface QueuedPrompt {
  attachments?: QueuedPromptAttachmentSummary;
  id: string;
  mode?: string;
  status?: string;
  text: string;
}

export function queuedPromptAttachmentSummary(
  attachments: SessionInputPayload["attachments"],
  workspaceId: string,
  sessionId: string
): QueuedPromptAttachmentSummary | undefined {
  if (!attachments || attachments.length === 0) return undefined;

  const first = attachments[0];
  const imageCount = attachments.filter(attachment =>
    isImageAttachmentMime(attachment.mime_type)
  ).length;
  const fileCount = attachments.length - imageCount;
  const imageURL = isImageAttachmentMime(first.mime_type)
    ? sessionAttachmentBytesURL(workspaceId, sessionId, `compozy://session-attachments/${first.id}`)
    : null;

  return {
    fileCount,
    imageCount,
    preview: imageURL
      ? { kind: "image", url: imageURL }
      : { kind: "file", mark: attachmentExtensionMark(first.name, first.mime_type) },
  };
}
