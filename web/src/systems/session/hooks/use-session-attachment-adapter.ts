import { useState } from "react";
import type {
  Attachment,
  AttachmentAdapter,
  CompleteAttachment,
  PendingAttachment,
} from "@assistant-ui/react";

import {
  deleteSessionAttachment,
  SessionAttachmentApiError,
  uploadSessionAttachment,
} from "../adapters/session-attachment-api";
import {
  ATTACHMENT_ALLOWED_REASON,
  ATTACHMENT_MAX_FILE_BYTES,
  attachmentOversizeMessage,
  SESSION_ATTACHMENT_PART_NAME,
  sniffAttachmentFile,
} from "../lib/attachment-kinds";
import type { SessionAttachment } from "../types";

export const SESSION_ATTACHMENT_ADAPTER_ACCEPT = "*";

interface SessionAttachmentAdapterOptions {
  sessionId: string;
  workspaceId: string;
}

function pendingId(): string {
  const crypto = globalThis.crypto;
  if (typeof crypto?.randomUUID === "function") {
    return crypto.randomUUID();
  }
  if (typeof crypto?.getRandomValues !== "function") {
    return `pending-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
  }
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6]! & 0x0f) | 0x40;
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;
  const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function persistFailureMessage(error: unknown): string {
  if (error instanceof SessionAttachmentApiError) {
    if (error.statusCode === 413) return attachmentOversizeMessage(ATTACHMENT_MAX_FILE_BYTES + 1);
    if (error.statusCode === 415) return ATTACHMENT_ALLOWED_REASON;
  }
  return "Couldn't save";
}

export class SessionAttachmentAdapter implements AttachmentAdapter {
  /**
   * Wildcard so drop/paste of unsupported types still enter `add()` and become
   * in-place rejected tiles. The paperclip picker uses ATTACHMENT_PICKER_ACCEPT.
   */
  public accept = SESSION_ATTACHMENT_ADAPTER_ACCEPT;

  private readonly uploads = new Map<string, SessionAttachment>();
  private readonly workspaceId: string;
  private readonly sessionId: string;

  constructor({ workspaceId, sessionId }: SessionAttachmentAdapterOptions) {
    this.workspaceId = workspaceId;
    this.sessionId = sessionId;
  }

  public async *add({ file }: { file: File }): AsyncGenerator<PendingAttachment, void> {
    const id = pendingId();
    const sniff = await sniffAttachmentFile(file);
    const type = sniff.ok && sniff.kind === "image" ? "image" : "file";
    const base: Omit<PendingAttachment, "status"> = {
      id,
      type,
      name: file.name,
      contentType: sniff.ok ? sniff.mimeType : file.type,
      file,
    };

    if (!sniff.ok && sniff.reason === "too-large") {
      yield {
        ...base,
        status: {
          type: "incomplete",
          reason: "error",
          message: attachmentOversizeMessage(file.size),
        },
      };
      return;
    }
    if (!sniff.ok) {
      yield {
        ...base,
        status: { type: "incomplete", reason: "error", message: ATTACHMENT_ALLOWED_REASON },
      };
      return;
    }

    yield {
      ...base,
      contentType: sniff.mimeType,
      status: { type: "running", reason: "uploading", progress: 0 },
    };

    try {
      const uploaded = await uploadSessionAttachment(this.workspaceId, this.sessionId, file);
      this.uploads.set(id, uploaded);
      yield {
        ...base,
        contentType: uploaded.mime_type,
        status: { type: "requires-action", reason: "composer-send" },
      };
    } catch (error) {
      yield {
        ...base,
        status: {
          type: "incomplete",
          reason: "error",
          message: persistFailureMessage(error),
        },
      };
    }
  }

  public async remove(attachment: Attachment): Promise<void> {
    const uploaded = this.uploads.get(attachment.id);
    this.uploads.delete(attachment.id);
    if (!uploaded) return;
    if (attachment.status.type === "complete") return;
    await deleteSessionAttachment(this.workspaceId, this.sessionId, uploaded.id);
  }

  public async send(attachment: PendingAttachment): Promise<CompleteAttachment> {
    const uploaded = this.uploads.get(attachment.id);
    if (!uploaded) {
      throw new Error(`Attachment "${attachment.name}" is not ready to send`);
    }
    this.uploads.delete(attachment.id);
    return {
      ...attachment,
      id: uploaded.id,
      contentType: uploaded.mime_type,
      status: { type: "complete" },
      content: [
        {
          type: "data",
          name: SESSION_ATTACHMENT_PART_NAME,
          data: {
            bytes: uploaded.bytes,
            height: uploaded.height,
            id: uploaded.id,
            kind: uploaded.kind,
            mime_type: uploaded.mime_type,
            name: uploaded.name,
            sha256: uploaded.sha256,
            width: uploaded.width,
          },
        },
      ],
    };
  }
}

export function useSessionAttachmentAdapter(
  workspaceId: string,
  sessionId: string
): SessionAttachmentAdapter {
  const [scope, setScope] = useState({ workspaceId, sessionId });
  const [adapter, setAdapter] = useState(
    () => new SessionAttachmentAdapter({ workspaceId, sessionId })
  );
  if (scope.workspaceId !== workspaceId || scope.sessionId !== sessionId) {
    const next = new SessionAttachmentAdapter({ workspaceId, sessionId });
    setScope({ workspaceId, sessionId });
    setAdapter(next);
    return next;
  }
  return adapter;
}
