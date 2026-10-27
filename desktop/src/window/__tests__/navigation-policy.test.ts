import type { Session } from "electron";
import { describe, expect, it, vi } from "vitest";

import { classifyNavigation, safeExternalURL } from "../navigation-policy";
import { applyDefaultDenyPermissions } from "../security";

vi.mock("electron", () => ({ shell: { openExternal: vi.fn() } }));

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

// Invariant: both Electron permission paths allow only product main-frame notifications,
// clipboard writes, and installed-font reads. Unknown permissions, foreign origins, and subframes
// remain default-denied.
describe("product window permissions", () => {
  it.each([
    ["notifications", "http://127.0.0.1:2123/", true, true],
    ["clipboard-sanitized-write", "http://127.0.0.1:2123/sessions", true, true],
    ["local-fonts", "http://127.0.0.1:2123/settings/appearance", true, true],
    ["local-fonts", "https://example.com/", true, false],
    ["notifications", "https://example.com/", true, false],
    ["notifications", "http://127.0.0.1:2124/", true, false],
    ["notifications", "http://127.0.0.1:2123/", false, false],
    ["clipboard-sanitized-write", "http://127.0.0.1:2123/", false, false],
    ["notifications", "invalid", true, false],
    ["notifications", "", true, false],
    ["media", "http://127.0.0.1:2123/", true, false],
    ["geolocation", "http://127.0.0.1:2123/", true, false],
  ])("Should handle %s from %s (main frame: %s)", (permission, url, isMainFrame, allowed) => {
    const check = vi.fn();
    const request = vi.fn();
    const session = {
      setPermissionCheckHandler: check,
      setPermissionRequestHandler: request,
    } as unknown as Session;
    for (const productOrigin of ["http://127.0.0.1:2123", undefined]) {
      applyDefaultDenyPermissions(session, productOrigin);
      const expected = Boolean(productOrigin) && allowed;
      expect(check.mock.lastCall?.[0](null, permission, url, { isMainFrame })).toBe(expected);
      const callback = vi.fn();
      request.mock.lastCall?.[0](null, permission, callback, {
        requestingUrl: url,
        isMainFrame,
      });
      expect(callback).toHaveBeenCalledExactlyOnceWith(expected);
    }
  });
});
