import { describe, expect, it } from "vitest";

import {
  apiErrorDetails,
  isRemoteRejectionCode,
  remoteRejectionCopy,
  remoteRejectionMessage,
} from "../remote-error-copy";

describe("remote-error-copy", () => {
  it("Should map every remote rejection code to plain-English copy", () => {
    const codes = [
      "remote_scope_denied",
      "remote_entitlement_unavailable",
      "plan_required",
      "device_revoked",
      "device_pending_approval",
      "device_not_approved",
      "route_not_remote",
      "remote_bridge_not_transitive",
      "remote_bridge_browser_denied",
      "claim_token_not_remote",
      "tool_not_forwardable",
      "tool_forward_denied",
      "remote_caller_override_forbidden",
      "computer_not_owned",
    ];

    for (const code of codes) {
      expect(isRemoteRejectionCode(code)).toBe(true);
      const copy = remoteRejectionCopy(code);
      expect(copy).not.toBeNull();
      expect(copy!.title.length).toBeGreaterThan(0);
      expect(copy!.detail.length).toBeGreaterThan(0);
    }
    expect(isRemoteRejectionCode("unknown_error")).toBe(false);
    expect(remoteRejectionCopy("unknown_error")).toBeNull();
  });

  it("Should name the denied scope in remote_scope_denied copy", () => {
    const terminalDenied = remoteRejectionCopy("remote_scope_denied", {
      scope: "terminal:operate",
    });
    expect(terminalDenied).toEqual({
      title: "Terminals aren't allowed from this device",
      detail:
        "Ask an admin device to enable terminal:operate in Settings → Computers → Permissions.",
    });

    const filesDenied = remoteRejectionMessage("remote_scope_denied", "fallback", {
      scope: "files:read",
    });
    expect(filesDenied).toContain("Reading files isn't allowed from this device");
    expect(filesDenied).toContain("files:read");
  });

  it("Should extract string details safely from error payloads", () => {
    expect(apiErrorDetails({ details: { scope: "terminal:operate", count: 2 } })).toEqual({
      scope: "terminal:operate",
    });
    expect(apiErrorDetails({ details: {} })).toBeUndefined();
    expect(apiErrorDetails(null)).toBeUndefined();
  });
});
