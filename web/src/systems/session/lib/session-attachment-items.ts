import {
  attachmentExtensionMark,
  formatAttachmentSize,
  isImageAttachmentMime,
} from "./attachment-kinds";
import { sessionAttachmentBytesURL, sessionAttachmentIdFromURI } from "./attachment-url";

export interface SessionAttachmentImageItem {
  kind: "image";
  id: string;
  href: string;
  src: string;
  filename: string;
  width?: number;
  height?: number;
}

export interface SessionAttachmentFileItem {
  kind: "file";
  id: string;
  href: string;
  filename: string;
  extension: string;
  sizeLabel?: string;
}

export type SessionAttachmentItem = SessionAttachmentImageItem | SessionAttachmentFileItem;

interface AttachmentRecordMeta {
  id: string;
  bytes?: number;
  width?: number;
  height?: number;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringField(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key];
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function numberField(record: Record<string, unknown>, key: string): number | undefined {
  const value = record[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

export function isImageMediaType(mediaType: string): boolean {
  return isImageAttachmentMime(mediaType);
}

export function formatAttachmentBytes(bytes: number): string {
  return formatAttachmentSize(bytes);
}

export { attachmentExtensionMark };

function messageCustomMetadata(metadata: unknown): Record<string, unknown> | null {
  if (!isRecord(metadata)) {
    return null;
  }
  if (isRecord(metadata.custom)) {
    return metadata.custom;
  }
  return metadata;
}

function attachmentMetaById(metadata: unknown): Map<string, AttachmentRecordMeta> {
  const custom = messageCustomMetadata(metadata);
  const records = custom && Array.isArray(custom.attachments) ? custom.attachments : [];
  const byId = new Map<string, AttachmentRecordMeta>();
  for (const record of records) {
    if (!isRecord(record)) continue;
    const id = stringField(record, "id");
    if (!id) continue;
    byId.set(id, {
      id,
      bytes: numberField(record, "bytes"),
      width: numberField(record, "width"),
      height: numberField(record, "height"),
    });
  }
  return byId;
}

function filePartFields(part: Record<string, unknown>): {
  uri: string;
  mediaType: string;
  filename: string;
} | null {
  const type = stringField(part, "type");
  if (type === "file") {
    const uri = stringField(part, "data") ?? stringField(part, "url");
    const mediaType = stringField(part, "mimeType") ?? stringField(part, "mediaType");
    const filename = stringField(part, "filename") ?? "file";
    if (!uri || !mediaType) return null;
    return { uri, mediaType, filename };
  }
  if (type === "image") {
    const uri = stringField(part, "image") ?? stringField(part, "url") ?? stringField(part, "data");
    if (!uri) return null;
    return { uri, mediaType: "image/png", filename: stringField(part, "filename") ?? "image" };
  }
  return null;
}

export function userMessageHasText(content: unknown): boolean {
  if (!Array.isArray(content)) {
    return false;
  }
  return content.some(part => {
    if (!isRecord(part) || stringField(part, "type") !== "text") {
      return false;
    }
    const text = stringField(part, "text");
    return text !== undefined && text.trim().length > 0;
  });
}

export function userMessageHasAttachments(content: unknown): boolean {
  if (!Array.isArray(content)) {
    return false;
  }
  return content.some(part => {
    if (!isRecord(part)) return false;
    const type = stringField(part, "type");
    return type === "file" || type === "image";
  });
}

export function userMessageAttachmentItems(
  content: unknown,
  metadata: unknown,
  workspaceId: string,
  sessionId: string
): SessionAttachmentItem[] {
  if (!Array.isArray(content)) {
    return [];
  }
  const metaById = attachmentMetaById(metadata);
  const items: SessionAttachmentItem[] = [];
  for (const part of content) {
    if (!isRecord(part)) continue;
    const fields = filePartFields(part);
    if (!fields) continue;
    const href = sessionAttachmentBytesURL(workspaceId, sessionId, fields.uri);
    if (!href) continue;
    const attachmentId = sessionAttachmentIdFromURI(fields.uri) ?? fields.uri;
    const meta = metaById.get(attachmentId);
    if (isImageMediaType(fields.mediaType)) {
      items.push({
        kind: "image",
        id: attachmentId,
        href,
        src: href,
        filename: fields.filename,
        ...(meta?.width ? { width: meta.width } : {}),
        ...(meta?.height ? { height: meta.height } : {}),
      });
      continue;
    }
    items.push({
      kind: "file",
      id: attachmentId,
      href,
      filename: fields.filename,
      extension: attachmentExtensionMark(fields.filename, fields.mediaType),
      ...(meta?.bytes !== undefined ? { sizeLabel: formatAttachmentBytes(meta.bytes) } : {}),
    });
  }
  return items;
}
