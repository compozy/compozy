// Suite: attachment-url
// Invariant: only `compozy://session-attachments/<id>` resolves to the
// workspace-scoped bytes route; `data:` and http(s) URLs stay null.
// Owning layer: unit (systems/session/lib)
// No existing suite owned this helper.

import { describe, expect, it } from "vitest";

import { sessionAttachmentBytesURL, sessionAttachmentIdFromURI } from "../attachment-url";

describe("sessionAttachmentIdFromURI", () => {
  it("Should extract the attachment id from a session-attachment URI", () => {
    expect(sessionAttachmentIdFromURI("compozy://session-attachments/att-image")).toBe("att-image");
  });

  it("Should reject data, http, and empty ids", () => {
    expect(sessionAttachmentIdFromURI("data:image/png;base64,abc")).toBeNull();
    expect(sessionAttachmentIdFromURI("https://cdn.example/diagram.png")).toBeNull();
    expect(sessionAttachmentIdFromURI("compozy://session-attachments/")).toBeNull();
    expect(sessionAttachmentIdFromURI("compozy://tool-artifacts/art_abc")).toBeNull();
  });
});

describe("sessionAttachmentBytesURL", () => {
  it("Should resolve a session-attachment URI onto the bytes route", () => {
    const uri = "compozy://session-attachments/att-image";
    expect(sessionAttachmentBytesURL("ws_product_studio", "sess_frontend_launch_qa", uri)).toBe(
      "/api/workspaces/ws_product_studio/sessions/sess_frontend_launch_qa/attachments/att-image/bytes"
    );
  });

  it("Should encode workspace, session, and attachment ids", () => {
    expect(sessionAttachmentBytesURL("ws/a", "sess b", "compozy://session-attachments/att c")).toBe(
      "/api/workspaces/ws%2Fa/sessions/sess%20b/attachments/att%20c/bytes"
    );
  });

  it("Should return null for data URLs, external URLs, and empty scope", () => {
    expect(sessionAttachmentBytesURL("ws", "sess", "data:image/png;base64,abc")).toBeNull();
    expect(sessionAttachmentBytesURL("ws", "sess", "https://cdn.example/diagram.png")).toBeNull();
    expect(
      sessionAttachmentBytesURL(" ", "sess", "compozy://session-attachments/att-image")
    ).toBeNull();
    expect(
      sessionAttachmentBytesURL("ws", "", "compozy://session-attachments/att-image")
    ).toBeNull();
  });
});
