import type {
  Attachment,
  AttachmentAdapter,
  CompleteAttachment,
  PendingAttachment,
} from "@assistant-ui/react";
import { useState } from "react";

import {
  deleteSessionAttachment,
  SessionAttachmentApiError,
  uploadSessionAttachment,
} from "../adapters/session-attachment-api";
import { ATTACHMENT_ALLOWED_REASON, SESSION_ATTACHMENT_PART_NAME } from "../lib/attachment-kinds";
import { consumeSubmittedComposerAttachment } from "../lib/session-composer-attachment-ownership";
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
    if (error.statusCode === 413) return "File exceeds the server upload limit";
    if (error.statusCode === 415) return ATTACHMENT_ALLOWED_REASON;
  }
  return "Couldn't save";
}

function registerFileAliases(
  files: Map<string, File>,
  pendingID: string,
  uploadedID: string,
  file: File
): void {
  files.set(pendingID, file);
  files.set(uploadedID, file);
}

export class SessionAttachmentAdapter implements AttachmentAdapter {
  /**
   * Wildcard lets the daemon apply its configured admission policy consistently
   * to picker, drop, and paste input.
   */
  public accept = SESSION_ATTACHMENT_ADAPTER_ACCEPT;

  private readonly uploads = new Map<string, SessionAttachment>();
  private readonly sentFiles = new Map<string, File>();
  private readonly uploadedFiles = new Map<string, File>();
  private readonly inFlightUploads = new Map<string, Promise<SessionAttachment>>();
  private readonly removedWhileUploading = new Set<string>();
  private readonly workspaceId: string;
  private readonly sessionId: string;

  constructor({ workspaceId, sessionId }: SessionAttachmentAdapterOptions) {
    this.workspaceId = workspaceId;
    this.sessionId = sessionId;
  }

  public async *add({ file }: { file: File }): AsyncGenerator<PendingAttachment, void> {
    const id = pendingId();
    const base: Omit<PendingAttachment, "status"> = {
      id,
      type: file.type.startsWith("image/") ? "image" : "file",
      name: file.name,
      contentType: file.type,
      file,
    };

    yield {
      ...base,
      status: { type: "running", reason: "uploading", progress: 0 },
    };

    try {
      const upload = uploadSessionAttachment(this.workspaceId, this.sessionId, file);
      this.inFlightUploads.set(id, upload);
      const uploaded = await upload;
      if (this.removedWhileUploading.has(id)) return;
      this.uploads.set(id, uploaded);
      registerFileAliases(this.uploadedFiles, id, uploaded.id, file);
      yield {
        ...base,
        contentType: uploaded.mime_type,
        status: { type: "requires-action", reason: "composer-send" },
        content: [
          {
            type: "data",
            name: SESSION_ATTACHMENT_PART_NAME,
            data: uploaded,
          },
        ],
      };
    } catch (error) {
      if (this.removedWhileUploading.has(id)) return;
      yield {
        ...base,
        status: {
          type: "incomplete",
          reason: "error",
          message: persistFailureMessage(error),
        },
      };
    } finally {
      this.inFlightUploads.delete(id);
    }
  }

  public async remove(attachment: Attachment): Promise<void> {
    if (consumeSubmittedComposerAttachment(attachment)) {
      const uploaded = this.uploads.get(attachment.id);
      this.uploads.delete(attachment.id);
      if (uploaded) this.forgetUploadedFile(uploaded.id);
      return;
    }
    const pending = this.inFlightUploads.get(attachment.id);
    if (pending) {
      this.removedWhileUploading.add(attachment.id);
      try {
        const uploaded = await pending;
        try {
          await deleteSessionAttachment(this.workspaceId, this.sessionId, uploaded.id);
          this.forgetUploadedFile(uploaded.id);
        } catch (error) {
          this.uploads.set(attachment.id, uploaded);
          throw error;
        }
      } catch (error) {
        if (this.uploads.has(attachment.id)) throw error;
        return;
      } finally {
        this.removedWhileUploading.delete(attachment.id);
      }
      return;
    }
    const uploaded = this.uploads.get(attachment.id);
    if (!uploaded) return;
    await deleteSessionAttachment(this.workspaceId, this.sessionId, uploaded.id);
    this.uploads.delete(attachment.id);
    this.forgetUploadedFile(uploaded.id);
  }

  public async send(attachment: PendingAttachment): Promise<CompleteAttachment> {
    const uploaded = this.uploads.get(attachment.id);
    if (!uploaded) {
      throw new Error(`Attachment "${attachment.name}" is not ready to send`);
    }
    this.uploads.delete(attachment.id);
    if (attachment.file) {
      registerFileAliases(this.sentFiles, attachment.id, uploaded.id, attachment.file);
    }
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

  public acknowledgeSentFiles(): void {
    this.sentFiles.clear();
    this.uploadedFiles.clear();
  }

  public recoverSentFiles(): File[] {
    const files = new Set([...this.sentFiles.values(), ...this.uploadedFiles.values()]);
    for (const file of files) {
      this.forgetFile(file);
    }
    return [...files];
  }

  private forgetUploadedFile(uploadedID: string): void {
    const file = this.uploadedFiles.get(uploadedID);
    if (file) this.forgetFile(file);
  }

  private forgetFile(file: File): void {
    for (const [id, candidate] of this.sentFiles) {
      if (candidate === file) this.sentFiles.delete(id);
    }
    for (const [id, candidate] of this.uploadedFiles) {
      if (candidate === file) this.uploadedFiles.delete(id);
    }
  }
}

export function useSessionAttachmentAdapter(
  workspaceId: string,
  sessionId: string
): SessionAttachmentAdapter {
  const [adapter] = useState(() => new SessionAttachmentAdapter({ workspaceId, sessionId }));
  return adapter;
}
