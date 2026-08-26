import { describe, expect, it } from "vitest";

import { projectTerminalBadge } from "../terminal-badge";
import type { TerminalInputRequest } from "../../types";

function inputRequest(overrides: Partial<TerminalInputRequest> = {}): TerminalInputRequest {
  return {
    id: "req-3f8a",
    terminal_id: "term-9cd7e14b2a66",
    profile_id: "profile-work",
    profile_name: "work",
    reason: "sudo password",
    prompt_excerpt: "Password:",
    redacted: true,
    requested_at: "2026-08-25T12:44:00Z",
    ...overrides,
  };
}

describe("projectTerminalBadge", () => {
  it("Should count only input and approval rows owned by the profile", () => {
    expect(
      projectTerminalBadge({
        scopeKey: "work-scope",
        profileId: "profile-work",
        inputRequests: [inputRequest(), inputRequest({ id: "req-9c11" })],
        pendingApprovals: [{ profileId: "profile-work" }, { profileId: "profile-personal" }],
      })
    ).toEqual({ scopeKey: "work-scope", count: 3 });
  });
});
