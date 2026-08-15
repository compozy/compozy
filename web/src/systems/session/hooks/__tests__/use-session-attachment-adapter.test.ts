import { beforeEach, describe, expect, it, vi } from "vitest";

const attachmentApiMocks = vi.hoisted(() => ({
  deleteSessionAttachment: vi.fn(),
  uploadSessionAttachment: vi.fn(),
}));

vi.mock("../../adapters/session-attachment-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/session-attachment-api")>()),
  deleteSessionAttachment: attachmentApiMocks.deleteSessionAttachment,
  uploadSessionAttachment: attachmentApiMocks.uploadSessionAttachment,
}));

import type { SessionAttachment } from "../../types";
import { SessionAttachmentAdapter } from "../use-session-attachment-adapter";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, reject, resolve };
}

function pngFile(): File {
  return new File([Uint8Array.of(0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a)], "race.png", {
    type: "image/png",
  });
}

function uploadedAttachment(): SessionAttachment {
  return {
    bytes: 8,
    created_at: "2026-08-15T00:00:00Z",
    height: 10,
    id: `att_${"a".repeat(64)}`,
    kind: "image",
    mime_type: "image/png",
    name: "race.png",
    sha256: "a".repeat(64),
    width: 10,
  };
}

describe("SessionAttachmentAdapter upload removal", () => {
  beforeEach(() => {
    attachmentApiMocks.deleteSessionAttachment.mockReset();
    attachmentApiMocks.uploadSessionAttachment.mockReset();
  });

  it("Should delete bytes when removal wins an in-flight upload", async () => {
    const upload = deferred<SessionAttachment>();
    attachmentApiMocks.uploadSessionAttachment.mockReturnValue(upload.promise);
    attachmentApiMocks.deleteSessionAttachment.mockResolvedValue(undefined);
    const adapter = new SessionAttachmentAdapter({ workspaceId: "ws-a", sessionId: "sess-a" });
    const iterator = adapter.add({ file: pngFile() });
    const running = await iterator.next();
    if (running.done) throw new Error("attachment upload ended before yielding its pending state");

    const completion = iterator.next();
    await vi.waitFor(() =>
      expect(attachmentApiMocks.uploadSessionAttachment).toHaveBeenCalledOnce()
    );
    const removal = adapter.remove(running.value);
    upload.resolve(uploadedAttachment());

    await expect(removal).resolves.toBeUndefined();
    await expect(completion).resolves.toEqual({ done: true, value: undefined });
    expect(attachmentApiMocks.deleteSessionAttachment).toHaveBeenCalledWith(
      "ws-a",
      "sess-a",
      `att_${"a".repeat(64)}`
    );
  });

  it("Should retain the durable ref when in-flight cleanup fails so removal can retry", async () => {
    const upload = deferred<SessionAttachment>();
    attachmentApiMocks.uploadSessionAttachment.mockReturnValue(upload.promise);
    attachmentApiMocks.deleteSessionAttachment
      .mockRejectedValueOnce(new Error("delete unavailable"))
      .mockResolvedValueOnce(undefined);
    const adapter = new SessionAttachmentAdapter({ workspaceId: "ws-a", sessionId: "sess-a" });
    const iterator = adapter.add({ file: pngFile() });
    const running = await iterator.next();
    if (running.done) throw new Error("attachment upload ended before yielding its pending state");

    const completion = iterator.next();
    await vi.waitFor(() =>
      expect(attachmentApiMocks.uploadSessionAttachment).toHaveBeenCalledOnce()
    );
    const removal = adapter.remove(running.value);
    upload.resolve(uploadedAttachment());

    await expect(removal).rejects.toThrow("delete unavailable");
    await expect(completion).resolves.toEqual({ done: true, value: undefined });
    await expect(adapter.remove(running.value)).resolves.toBeUndefined();
    expect(attachmentApiMocks.deleteSessionAttachment).toHaveBeenCalledTimes(2);
  });
});
