import type { Attachment } from "@assistant-ui/react";

import {
  ATTACHMENT_ALLOWED_REASON,
  attachmentsFromPromptMessageParts,
  formatAttachmentSize,
} from "@/systems/session/lib/attachment-kinds";

export type SessionAttachmentTileState = "ready" | "uploading" | "error" | "rejected";

export interface SessionAttachmentTileModel {
  id: string;
  name: string;
  mimeType?: string;
  bytes?: number;
  file?: File;
  height?: number;
  width?: number;
  state: SessionAttachmentTileState;
  sizeLabel: string;
  retryable: boolean;
}

export function sessionAttachmentTileState(attachment: Attachment): SessionAttachmentTileState {
  if (attachment.status.type === "running") return "uploading";
  if (attachment.status.type === "complete") return "ready";
  if (attachment.status.type === "requires-action") return "ready";
  if (attachment.status.type === "incomplete") {
    if (attachment.status.message === ATTACHMENT_ALLOWED_REASON) return "rejected";
    return "error";
  }
  return "ready";
}

export function sessionAttachmentTileModel(attachment: Attachment): SessionAttachmentTileModel {
  const state = sessionAttachmentTileState(attachment);
  const [uploaded] = attachmentsFromPromptMessageParts(attachment.content);
  const bytes = attachment.file?.size;
  let sizeLabel = bytes === undefined ? "" : formatAttachmentSize(bytes);
  if (state === "uploading") sizeLabel = "Saving…";
  if (state === "rejected") sizeLabel = ATTACHMENT_ALLOWED_REASON;
  if (state === "error") {
    sizeLabel =
      attachment.status.type === "incomplete"
        ? (attachment.status.message ?? "Couldn't save")
        : sizeLabel;
  }
  return {
    id: attachment.id,
    name: attachment.name,
    mimeType: attachment.contentType,
    bytes,
    file: attachment.file,
    ...(uploaded?.width && uploaded?.height
      ? { height: uploaded.height, width: uploaded.width }
      : {}),
    state,
    sizeLabel,
    retryable: state === "error" && sizeLabel === "Couldn't save",
  };
}
