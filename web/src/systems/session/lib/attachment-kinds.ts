/** Client pre-check only — server 413/415 is the authority. */
export const ATTACHMENT_MAX_FILE_BYTES = 10 * 1024 * 1024;
export const ATTACHMENT_MAX_COUNT = 10;

export const ATTACHMENT_ALLOWED_REASON = "PNG, JPEG, WebP, PDF, Markdown, or text";

export const SESSION_ATTACHMENT_PART_NAME = "compozy-attachment";
export const SESSION_ATTACHMENT_DATA_TYPE = `data-${SESSION_ATTACHMENT_PART_NAME}`;

const PNG_SIGNATURE = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a] as const;
const JPEG_SIGNATURE = [0xff, 0xd8, 0xff] as const;
const PDF_SIGNATURE = [0x25, 0x50, 0x44, 0x46] as const; // %PDF
const RIFF_SIGNATURE = [0x52, 0x49, 0x46, 0x46] as const;
const WEBP_MARKER = [0x57, 0x45, 0x42, 0x50] as const;

export type AttachmentKind = "image" | "file";
export type AttachmentFileMark = "PDF" | "MD" | "TXT";
export type AttachmentMIME =
  | "image/png"
  | "image/jpeg"
  | "image/webp"
  | "application/pdf"
  | "text/markdown"
  | "text/plain";

export type AttachmentSniffResult =
  | { ok: true; kind: AttachmentKind; mimeType: AttachmentMIME; mark?: AttachmentFileMark }
  | { ok: false; reason: "unsupported" | "too-large" };

export interface SessionAttachmentRef {
  bytes: number;
  height: number;
  id: string;
  kind: string;
  mime_type: string;
  name: string;
  sha256: string;
  width: number;
}

function bytesStartWith(bytes: Uint8Array, signature: readonly number[]): boolean {
  if (bytes.length < signature.length) return false;
  return signature.every((value, index) => bytes[index] === value);
}

function isWebp(bytes: Uint8Array): boolean {
  if (bytes.length < 12) return false;
  if (!bytesStartWith(bytes, RIFF_SIGNATURE)) return false;
  return (
    bytes[8] === WEBP_MARKER[0] &&
    bytes[9] === WEBP_MARKER[1] &&
    bytes[10] === WEBP_MARKER[2] &&
    bytes[11] === WEBP_MARKER[3]
  );
}

export function sniffAttachmentBytes(bytes: Uint8Array): AttachmentSniffResult {
  if (bytesStartWith(bytes, PNG_SIGNATURE)) {
    return { ok: true, kind: "image", mimeType: "image/png" };
  }
  if (bytesStartWith(bytes, JPEG_SIGNATURE)) {
    return { ok: true, kind: "image", mimeType: "image/jpeg" };
  }
  if (isWebp(bytes)) {
    return { ok: true, kind: "image", mimeType: "image/webp" };
  }
  if (bytesStartWith(bytes, PDF_SIGNATURE)) {
    return { ok: true, kind: "file", mimeType: "application/pdf", mark: "PDF" };
  }
  return { ok: false, reason: "unsupported" };
}

function extensionOf(name: string): string {
  const dot = name.lastIndexOf(".");
  if (dot < 0 || dot === name.length - 1) return "";
  return name.slice(dot + 1).toLowerCase();
}

function sniffTextFile(file: File): AttachmentSniffResult | null {
  const ext = extensionOf(file.name);
  if (file.type === "text/markdown" || ext === "md" || ext === "markdown" || ext === "mdown") {
    return { ok: true, kind: "file", mimeType: "text/markdown", mark: "MD" };
  }
  if (file.type === "text/plain" || ext === "txt") {
    return { ok: true, kind: "file", mimeType: "text/plain", mark: "TXT" };
  }
  return null;
}

export async function sniffAttachmentFile(file: File): Promise<AttachmentSniffResult> {
  if (file.size > ATTACHMENT_MAX_FILE_BYTES) {
    return { ok: false, reason: "too-large" };
  }
  const textHit = sniffTextFile(file);
  if (textHit) return textHit;
  const bytes = new Uint8Array(await file.arrayBuffer());
  const binary = sniffAttachmentBytes(bytes);
  if (binary.ok) return binary;
  try {
    const text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
    const hasUnsupportedControl = Array.from(text).some(character => {
      const code = character.codePointAt(0) ?? 0;
      return code < 0x20 && character !== "\n" && character !== "\r" && character !== "\t";
    });
    if (!hasUnsupportedControl) {
      return { ok: true, kind: "file", mimeType: "text/plain", mark: "TXT" };
    }
  } catch {
    // The daemon remains authoritative for unsupported binary input.
  }
  return { ok: false, reason: "unsupported" };
}

export function attachmentExtensionMark(name: string, mimeType?: string): string {
  if (mimeType === "application/pdf") return "PDF";
  if (mimeType === "text/markdown") return "MD";
  if (mimeType === "text/plain") return "TXT";
  const ext = extensionOf(name);
  if (ext === "pdf") return "PDF";
  if (ext === "md" || ext === "markdown") return "MD";
  if (ext === "txt") return "TXT";
  if (ext.length === 0) return "FILE";
  return ext.slice(0, 4).toUpperCase();
}

export function isImageAttachmentMime(mimeType: string | undefined): boolean {
  return mimeType === "image/png" || mimeType === "image/jpeg" || mimeType === "image/webp";
}

export function formatAttachmentSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  if (kb < 1024) {
    const rounded = kb >= 100 ? Math.round(kb) : Math.round(kb * 10) / 10;
    return `${rounded} KB`;
  }
  const mb = kb / 1024;
  const rounded = mb >= 100 ? Math.round(mb) : Math.round(mb * 10) / 10;
  return `${rounded} MB`;
}

export function attachmentOversizeMessage(bytes: number): string {
  return `${formatAttachmentSize(bytes)} · over 10 MB`;
}

export function isSessionAttachmentRef(value: unknown): value is SessionAttachmentRef {
  if (value == null || typeof value !== "object") return false;
  const record = value as Record<string, unknown>;
  return (
    typeof record.id === "string" &&
    typeof record.name === "string" &&
    typeof record.mime_type === "string" &&
    typeof record.bytes === "number" &&
    typeof record.sha256 === "string" &&
    typeof record.kind === "string" &&
    typeof record.width === "number" &&
    typeof record.height === "number"
  );
}

export function attachmentsFromPromptMessageParts(
  parts: readonly { type?: string; name?: string; data?: unknown }[] | undefined
): SessionAttachmentRef[] {
  if (!parts) return [];
  const attachments: SessionAttachmentRef[] = [];
  for (const part of parts) {
    const isAttachmentPart =
      part.type === SESSION_ATTACHMENT_DATA_TYPE ||
      (part.type === "data" && part.name === SESSION_ATTACHMENT_PART_NAME);
    if (!isAttachmentPart) continue;
    if (!isSessionAttachmentRef(part.data)) continue;
    attachments.push(part.data);
  }
  return attachments;
}
