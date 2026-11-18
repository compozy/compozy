const PREVIEWABLE_IMAGE_TYPES = new Set([
  "image/png",
  "image/jpeg",
  "image/gif",
  "image/webp",
  "image/svg+xml",
]);
const BASE64 = /^[A-Za-z0-9+/]*={0,2}$/;

/**
 * A `data:` URL for an image preview, or `null` when the type is not
 * previewable or the payload is not base64. SVG is only ever shown through
 * `<img>`: an image context runs no scripts and loads no external resources,
 * so markup inside the file is never inlined into the document.
 */
export function imageDataUrl(mimeType: string, base64: string): string | null {
  const type = mimeType.trim().toLowerCase().split(";")[0] ?? "";
  if (!PREVIEWABLE_IMAGE_TYPES.has(type)) return null;
  const payload = base64.replace(/\s+/g, "");
  if (!BASE64.test(payload)) return null;
  return `data:${type};base64,${payload}`;
}
