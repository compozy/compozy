import { describe, expect, it } from "vitest";

import { classifyNavigation, safeExternalURL } from "../navigation-policy";

// Invariant: product navigation stays on the daemon origin; only parsed HTTP(S) URLs and the exact
// Accessibility settings deep link leave through the operating system. Same-origin API/bytes
// documents cannot replace the SPA; this suite owns the desktop navigation boundary.
describe("navigation policy", () => {
  it.each([
    ["http://127.0.0.1:2123/sessions/abc", "allow"],
    ["http://127.0.0.1:2123/", "allow"],
    ["http://127.0.0.1:2123/api", "deny"],
    ["http://127.0.0.1:2123/%61pi/sessions", "deny"],
    ["http://127.0.0.1:2123/api%2Fworkspaces/ws/attachments/att/bytes", "deny"],
    ["http://127.0.0.1:2123/%invalid", "deny"],
    ["http://127.0.0.1:2123/api/sessions", "deny"],
    ["http://127.0.0.1:2123/api/workspaces/ws/sessions/sess/attachments/att/bytes", "deny"],
    [
      "http://127.0.0.1:2123/api/workspaces/ws/sessions/sess/attachments/att/bytes?download=1",
      "deny",
    ],
    ["http://127.0.0.1:2123/api/../sessions/abc", "allow"],
    ["http://127.0.0.1:2123/sessions/abc?return=/api/sessions", "allow"],
    ["https://compozy.com/docs", "external"],
    ["http://localhost:2123/", "external"],
    ["x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility", "external"],
    ["file:///etc/passwd", "deny"],
    ["mailto:test@example.com", "deny"],
    ["javascript:alert(1)", "deny"],
  ])("Should classify %s as %s", (target, expected) => {
    expect(classifyNavigation(target, "http://127.0.0.1:2123")).toBe(expected);
  });

  it("Should reject malformed and non-web external URLs", () => {
    expect(safeExternalURL("not a url")).toBeNull();
    expect(safeExternalURL("file:///tmp/a")).toBeNull();
    expect(safeExternalURL("x-apple.systempreferences:com.apple.preference.security")).toBeNull();
  });
});
