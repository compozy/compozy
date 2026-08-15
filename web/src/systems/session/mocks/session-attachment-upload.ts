const encoder = new TextEncoder();
const decoder = new TextDecoder();

export interface MockSessionAttachmentUpload {
  bytes: Uint8Array;
  mimeType: string;
  name: string;
}

function findBytes(haystack: Uint8Array, needle: Uint8Array, start: number): number {
  if (needle.byteLength === 0) return start;
  const lastStart = haystack.byteLength - needle.byteLength;
  for (let offset = start; offset <= lastStart; offset += 1) {
    let matches = true;
    for (let index = 0; index < needle.byteLength; index += 1) {
      if (haystack[offset + index] !== needle[index]) {
        matches = false;
        break;
      }
    }
    if (matches) return offset;
  }
  return -1;
}

function multipartBoundary(contentType: string | null): string | null {
  const match = contentType?.match(/boundary=(?:"([^"]+)"|([^;\s]+))/i);
  return match?.[1] ?? match?.[2] ?? null;
}

function quotedDispositionValue(headers: string, key: string): string | null {
  const disposition = headers.match(/^content-disposition:\s*([^\r\n]+)$/im)?.[1];
  if (!disposition) return null;
  const match = disposition.match(new RegExp(`(?:^|;)\\s*${key}="([^"]*)"`, "i"));
  return match?.[1] ?? null;
}

export async function uploadedAttachmentFromRequest(
  request: Request
): Promise<MockSessionAttachmentUpload | null> {
  const boundary = multipartBoundary(request.headers.get("content-type"));
  if (!boundary) return null;

  const body = new Uint8Array(await request.arrayBuffer());
  const delimiter = encoder.encode(`--${boundary}`);
  const headerSeparator = encoder.encode("\r\n\r\n");
  const nextDelimiter = encoder.encode(`\r\n--${boundary}`);
  let cursor = 0;

  while (cursor < body.byteLength) {
    const delimiterStart = findBytes(body, delimiter, cursor);
    if (delimiterStart < 0) return null;
    const headersStart = delimiterStart + delimiter.byteLength + 2;
    const headersEnd = findBytes(body, headerSeparator, headersStart);
    if (headersEnd < 0) return null;

    const headers = decoder.decode(body.subarray(headersStart, headersEnd));
    const contentStart = headersEnd + headerSeparator.byteLength;
    const contentEnd = findBytes(body, nextDelimiter, contentStart);
    if (contentEnd < 0) return null;

    if (quotedDispositionValue(headers, "name") === "file") {
      const name = quotedDispositionValue(headers, "filename");
      const mimeType = headers.match(/^content-type:\s*([^\r\n]+)$/im)?.[1]?.trim();
      if (!name || !mimeType) return null;
      return {
        bytes: body.slice(contentStart, contentEnd),
        mimeType,
        name,
      };
    }

    cursor = contentEnd + 2;
  }

  return null;
}
