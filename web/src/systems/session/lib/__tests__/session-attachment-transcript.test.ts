// Suite: session-attachment-transcript
// Invariant: Go-shaped file parts survive normalizeTranscriptMessages +
// toThreadPart, and the view-model resolves them to the bytes route (never
// data: or external URLs). Size/dimensions come from metadata.attachments.
// Owning layer: unit (systems/session/lib)
// No existing suite owned repository/schema file-part preservation.

import { describe, expect, it } from "vitest";

import { normalizeTranscriptMessages } from "../message-schemas";
import {
  userMessageAttachmentItems,
  userMessageHasAttachments,
  userMessageHasText,
} from "../session-attachment-items";
import { toThreadMessageLikes } from "../session-thread-repository";
import type { SessionMessage } from "../../types";

const WORKSPACE_ID = "ws_product_studio";
const SESSION_ID = "sess_frontend_launch_qa";

function goUserTurnWithAttachments(): SessionMessage {
  return {
    id: "user-attach-001",
    role: "user",
    parts: [
      {
        type: "file",
        mediaType: "image/png",
        url: "compozy://session-attachments/att-image",
        filename: "diagram.png",
      },
      {
        type: "file",
        mediaType: "application/pdf",
        url: "compozy://session-attachments/att-pdf",
        filename: "notes.pdf",
      },
      { type: "text", text: "Flatten both.", state: "done" },
    ],
    metadata: {
      attachments: [
        {
          id: "att-image",
          name: "diagram.png",
          mime_type: "image/png",
          bytes: 2048,
          sha256: "a".repeat(64),
          kind: "image",
          width: 1280,
          height: 720,
        },
        {
          id: "att-pdf",
          name: "notes.pdf",
          mime_type: "application/pdf",
          bytes: 4096,
          sha256: "b".repeat(64),
          kind: "file",
        },
      ],
    },
  } as SessionMessage;
}

describe("session attachment transcript projection", () => {
  it("Should keep file parts through normalize and toThreadPart", async () => {
    const normalized = await normalizeTranscriptMessages([goUserTurnWithAttachments()]);
    expect(normalized).toHaveLength(1);
    const parts = normalized[0]?.parts ?? [];
    expect(parts.filter(part => part.type === "file")).toHaveLength(2);
    expect(parts.some(part => part.type === "text")).toBe(true);

    const likes = toThreadMessageLikes(normalized);
    const content = likes[0]?.content;
    if (!Array.isArray(content)) {
      throw new Error("expected projected content parts");
    }
    const fileParts = content.filter(part => part.type === "file");
    expect(fileParts).toEqual([
      {
        type: "file",
        data: "compozy://session-attachments/att-image",
        mimeType: "image/png",
        filename: "diagram.png",
      },
      {
        type: "file",
        data: "compozy://session-attachments/att-pdf",
        mimeType: "application/pdf",
        filename: "notes.pdf",
      },
    ]);
  });

  it("Should resolve file parts to the bytes route and join metadata size", async () => {
    const normalized = await normalizeTranscriptMessages([goUserTurnWithAttachments()]);
    const likes = toThreadMessageLikes(normalized);
    const message = likes[0];
    if (!message) {
      throw new Error("expected a projected user message");
    }

    expect(userMessageHasText(message.content)).toBe(true);
    expect(userMessageHasAttachments(message.content)).toBe(true);

    const items = userMessageAttachmentItems(
      message.content,
      message.metadata,
      WORKSPACE_ID,
      SESSION_ID
    );
    expect(items).toEqual([
      {
        kind: "image",
        id: "att-image",
        href: `/api/workspaces/${WORKSPACE_ID}/sessions/${SESSION_ID}/attachments/att-image/bytes`,
        src: `/api/workspaces/${WORKSPACE_ID}/sessions/${SESSION_ID}/attachments/att-image/bytes`,
        filename: "diagram.png",
        width: 1280,
        height: 720,
      },
      {
        kind: "file",
        id: "att-pdf",
        href: `/api/workspaces/${WORKSPACE_ID}/sessions/${SESSION_ID}/attachments/att-pdf/bytes`,
        filename: "notes.pdf",
        extension: "PDF",
        sizeLabel: "4 KB",
      },
    ]);
    expect(items.every(item => !item.href.startsWith("data:"))).toBe(true);
  });

  it("Should drop data and external URLs instead of rendering them", () => {
    const items = userMessageAttachmentItems(
      [
        {
          type: "file",
          mimeType: "image/png",
          data: "data:image/png;base64,abc",
          filename: "leak.png",
        },
        {
          type: "file",
          mimeType: "image/png",
          data: "https://cdn.example/leak.png",
          filename: "remote.png",
        },
      ],
      {},
      WORKSPACE_ID,
      SESSION_ID
    );
    expect(items).toEqual([]);
  });

  it("Should treat image-only turns as attachments without text", async () => {
    const imageOnly = {
      id: "user-image-only",
      role: "user",
      parts: [
        {
          type: "file",
          mediaType: "image/webp",
          url: "compozy://session-attachments/att-shot",
          filename: "shot.webp",
        },
      ],
    } as SessionMessage;
    const normalized = await normalizeTranscriptMessages([imageOnly]);
    const likes = toThreadMessageLikes(normalized);
    expect(userMessageHasText(likes[0]?.content)).toBe(false);
    expect(userMessageHasAttachments(likes[0]?.content)).toBe(true);
  });
});
