// Suite: session prompt capability
// Invariant: the composer exposes a definitive capability result only when the
// current prompt target matches the daemon's ready ACP binding.
// Owning layer: systems/session/lib pure runtime-capability projection.
import { describe, expect, it } from "vitest";

import { primarySessionFixture } from "@/systems/session/mocks/fixtures";

import { sessionPromptCapability } from "../session-prompt-capability";
import type { SessionPayload } from "../../types";

const READY_ACP_CAPS = {
  prompt_audio: false,
  prompt_embedded_context: true,
  prompt_image: true,
  supports_load_session: false,
};

function readySession(): SessionPayload {
  return {
    ...primarySessionFixture,
    runtime: {
      acp_caps: READY_ACP_CAPS,
      effective: { model: "gpt-5", provider: "openai" },
      selected: { model: "gpt-5", provider: "openai" },
      selection_revision: 1,
      status: "ready",
    },
    state: "active",
  };
}

describe("sessionPromptCapability", () => {
  it.each([
    ["unknown for an inactive session", { state: "stopped" }, "unknown"],
    ["unknown for an unbound runtime", { runtime: { status: "unbound" } }, "unknown"],
    [
      "unknown when ACP capabilities are absent",
      { runtime: { ...readySession().runtime, acp_caps: undefined } },
      "unknown",
    ],
    [
      "unknown for a stale selected runtime",
      { runtime: { ...readySession().runtime, selected: { provider: "other" } } },
      "unknown",
    ],
    ["supported for a matching runtime", {}, "supported"],
    [
      "unsupported for a matching runtime",
      {
        runtime: {
          ...readySession().runtime,
          acp_caps: { ...READY_ACP_CAPS, prompt_image: false },
        },
      },
      "unsupported",
    ],
  ] as const)("Should return %s", (_name, override, expected) => {
    const session = { ...readySession(), ...override } as SessionPayload;
    expect(sessionPromptCapability(session, "prompt_image")).toBe(expected);
  });

  it("Should use the local prompt target instead of the stale daemon selection", () => {
    const session = {
      ...readySession(),
      runtime: { ...readySession().runtime, selected: { model: "previous", provider: "openai" } },
    };

    expect(
      sessionPromptCapability(session, "prompt_image", { model: "gpt-5", provider: "openai" })
    ).toBe("supported");
  });
});
