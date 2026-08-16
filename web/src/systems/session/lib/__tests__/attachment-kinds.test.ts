import { describe, expect, it } from "vitest";

import {
  ATTACHMENT_ALLOWED_REASON,
  attachmentExtensionMark,
  attachmentsFromPromptMessageParts,
  formatAttachmentSize,
  SESSION_ATTACHMENT_DATA_TYPE,
  SESSION_ATTACHMENT_PART_NAME,
  sniffAttachmentBytes,
  sniffAttachmentFile,
} from "../attachment-kinds";

function bytes(...values: number[]): Uint8Array {
  return Uint8Array.from(values);
}

describe("attachment-kinds", () => {
  it("Should sniff PNG, JPEG, WebP, and PDF magic bytes", () => {
    expect(sniffAttachmentBytes(bytes(0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a))).toEqual({
      ok: true,
      kind: "image",
      mimeType: "image/png",
    });
    expect(sniffAttachmentBytes(bytes(0xff, 0xd8, 0xff, 0xe0))).toEqual({
      ok: true,
      kind: "image",
      mimeType: "image/jpeg",
    });
    const webp = new Uint8Array(12);
    webp.set([0x52, 0x49, 0x46, 0x46], 0);
    webp.set([0x57, 0x45, 0x42, 0x50], 8);
    expect(sniffAttachmentBytes(webp)).toEqual({
      ok: true,
      kind: "image",
      mimeType: "image/webp",
    });
    expect(sniffAttachmentBytes(bytes(0x25, 0x50, 0x44, 0x46, 0x2d))).toEqual({
      ok: true,
      kind: "file",
      mimeType: "application/pdf",
      mark: "PDF",
    });
  });

  it("Should reject ZIP and SVG signatures as unsupported", () => {
    expect(sniffAttachmentBytes(bytes(0x50, 0x4b, 0x03, 0x04))).toEqual({
      ok: false,
      reason: "unsupported",
    });
    expect(sniffAttachmentBytes(bytes(0x3c, 0x73, 0x76, 0x67))).toEqual({
      ok: false,
      reason: "unsupported",
    });
  });

  it("Should accept markdown and text by mime or extension", async () => {
    const markdown = await sniffAttachmentFile(
      new File(["# notes"], "notes.md", { type: "text/markdown" })
    );
    expect(markdown).toEqual({ ok: true, kind: "file", mimeType: "text/markdown", mark: "MD" });
    const plain = await sniffAttachmentFile(new File(["log"], "error-log.txt", { type: "" }));
    expect(plain).toEqual({ ok: true, kind: "file", mimeType: "text/plain", mark: "TXT" });
    const markdownAlias = await sniffAttachmentFile(
      new File(["# release notes"], "release.mdown", { type: "" })
    );
    expect(markdownAlias).toEqual({
      ok: true,
      kind: "file",
      mimeType: "text/markdown",
      mark: "MD",
    });
    const utf8Log = await sniffAttachmentFile(
      new File(["service ready ✓"], "service.log", { type: "" })
    );
    expect(utf8Log).toEqual({ ok: true, kind: "file", mimeType: "text/plain", mark: "TXT" });
  });

  it("Should classify a file regardless of its browser-reported size", async () => {
    const oversized = new File([new Uint8Array(8)], "huge.png", { type: "image/png" });
    Object.defineProperty(oversized, "size", { value: 100 * 1024 * 1024 });
    expect(await sniffAttachmentFile(oversized)).toEqual({
      ok: false,
      reason: "unsupported",
    });
  });

  it("Should format sizes with the mono tile grammar", () => {
    expect(formatAttachmentSize(240 * 1024)).toBe("240 KB");
    expect(formatAttachmentSize(18.2 * 1024 * 1024)).toBe("18.2 MB");
    expect(attachmentExtensionMark("flatten-spec.pdf", "application/pdf")).toBe("PDF");
    expect(ATTACHMENT_ALLOWED_REASON).toBe("PNG, JPEG, WebP, PDF, Markdown, or text");
  });

  it("Should collect prompt attachment refs from data parts", () => {
    const ref = {
      bytes: 32,
      height: 10,
      id: "att_1",
      kind: "image",
      mime_type: "image/png",
      name: "shot.png",
      sha256: "b".repeat(64),
      width: 10,
    };
    expect(
      attachmentsFromPromptMessageParts([
        { type: "text", data: "ignore" },
        { type: SESSION_ATTACHMENT_DATA_TYPE, data: ref },
        { type: "data", name: SESSION_ATTACHMENT_PART_NAME, data: ref },
      ])
    ).toEqual([ref, ref]);
  });
});
