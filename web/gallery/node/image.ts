/**
 * Header-only image inspection for comment attachments: the type comes from the magic bytes,
 * never from the file name or the declared MIME type, and the dimensions from the header.
 */
import { maxAttachmentBytes } from "../shell/comments/model.ts";

export type ImageType = "image/png" | "image/jpeg" | "image/gif" | "image/webp";

export type ImageInfo = { type: ImageType; width: number; height: number; size: number };

export class ImageError extends Error {
  constructor(readonly code: "attachment_too_large" | "unsupported_media_type") {
    super(code);
  }
}

const jpegFrameMarkers = new Set([192, 193, 194, 195, 197, 198, 199, 201, 202, 203, 205, 206, 207]);

function jpegSize(bytes: Uint8Array, data: DataView) {
  let offset = 2;
  while (offset < bytes.length) {
    if (bytes[offset++] !== 0xff) break;
    while (bytes[offset] === 0xff) offset++;
    const marker = bytes[offset++];
    // Start of scan or end of image: no frame header before the pixel data.
    if (marker === undefined || marker === 0xda || marker === 0xd9) break;
    if (marker === 0x01 || (marker >= 0xd0 && marker <= 0xd7)) continue;
    const length = data.getUint16(offset);
    if (length < 2 || offset + length > bytes.length) break;
    if (jpegFrameMarkers.has(marker)) {
      if (length < 7) break;
      return { height: data.getUint16(offset + 3), width: data.getUint16(offset + 5) };
    }
    offset += length;
  }
  return { width: 0, height: 0 };
}

function webpSize(bytes: Uint8Array, data: DataView, chunk: string) {
  const at = (index: number) => bytes[index] ?? 0;
  if (chunk === "VP8X" && bytes.length >= 30) {
    return {
      width: 1 + (at(24) | (at(25) << 8) | (at(26) << 16)),
      height: 1 + (at(27) | (at(28) << 8) | (at(29) << 16)),
    };
  }
  if (chunk === "VP8L" && at(20) === 0x2f) {
    const bits = data.getUint32(21, true);
    return { width: (bits & 0x3fff) + 1, height: ((bits >>> 14) & 0x3fff) + 1 };
  }
  if (chunk === "VP8 " && at(23) === 0x9d && at(24) === 0x01 && at(25) === 0x2a) {
    return { width: data.getUint16(26, true) & 0x3fff, height: data.getUint16(28, true) & 0x3fff };
  }
  return { width: 0, height: 0 };
}

/**
 * A PNG whose chunks fit the buffer from IHDR through IEND with image data in between; a
 * truncated or padded upload is rejected rather than stored as a broken image.
 */
function pngComplete(bytes: Uint8Array, data: DataView) {
  let offset = 8;
  let sawData = false;
  while (offset + 12 <= bytes.length) {
    const length = data.getUint32(offset);
    const type = String.fromCharCode(...bytes.subarray(offset + 4, offset + 8));
    const end = offset + 12 + length;
    if (end > bytes.length) return false;
    if (offset === 8 && type !== "IHDR") return false;
    if (type === "IDAT") sawData = true;
    if (type === "IEND") return sawData && end === bytes.length;
    offset = end;
  }
  return false;
}

function detect(bytes: Uint8Array): { type: ImageType; width: number; height: number } | null {
  const data = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const text = (offset: number, length: number) =>
    String.fromCharCode(...bytes.subarray(offset, offset + length));
  if (bytes[0] === 0x89 && text(1, 7) === "PNG\r\n\u001a\n" && text(12, 4) === "IHDR") {
    if (!pngComplete(bytes, data)) return null;
    return { type: "image/png", width: data.getUint32(16), height: data.getUint32(20) };
  }
  if (text(0, 6) === "GIF87a" || text(0, 6) === "GIF89a") {
    return { type: "image/gif", width: data.getUint16(6, true), height: data.getUint16(8, true) };
  }
  if (bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) {
    return { type: "image/jpeg", ...jpegSize(bytes, data) };
  }
  if (text(0, 4) === "RIFF" && text(8, 4) === "WEBP") {
    return { type: "image/webp", ...webpSize(bytes, data, text(12, 4)) };
  }
  return null;
}

/** Throws `ImageError` unless the bytes are a PNG, JPEG, GIF or WebP of at most 5 MB. */
export function inspectImage(bytes: Uint8Array): ImageInfo {
  if (bytes.length > maxAttachmentBytes) throw new ImageError("attachment_too_large");
  let found: ReturnType<typeof detect>;
  try {
    found = detect(bytes);
  } catch {
    // A truncated header reads past the end of the buffer.
    found = null;
  }
  if (!found || !found.width || !found.height) throw new ImageError("unsupported_media_type");
  return { ...found, size: bytes.length };
}
