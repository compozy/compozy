import { describe, expect, it } from "vitest";

import { classifyNavigation, safeExternalURL } from "../navigation-policy";

// Invariant: product navigation stays on the daemon origin; only parsed HTTP(S) URLs and the exact
// Accessibility settings deep link leave through the operating system.
describe("navigation policy", () => {
  it.each([
    ["http://127.0.0.1:2123/sessions/abc", "allow"],
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
