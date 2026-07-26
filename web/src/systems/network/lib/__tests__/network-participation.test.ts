import { describe, expect, it } from "vitest";

import {
  DEFAULT_NETWORK_PARTICIPATION_DRAFT,
  isNetworkParticipationDraftValid,
  serializeNetworkParticipation,
} from "../network-participation";

describe("serializeNetworkParticipation", () => {
  it("Should default to local without legacy participation fields", () => {
    const payload = serializeNetworkParticipation(DEFAULT_NETWORK_PARTICIPATION_DRAFT);
    expect(payload).toEqual({ mode: "local" });
  });

  it("Should serialize named Live mode with required channel addressing", () => {
    const payload = serializeNetworkParticipation({
      mode: "live",
      channelId: "builders",
      channelStrategy: "named",
    });
    expect(payload).toEqual({
      mode: "live",
      channel_id: "builders",
      channel_strategy: "named",
    });
  });

  it("Should serialize a derived run channel without caller-authored channel identity", () => {
    expect(
      serializeNetworkParticipation({ mode: "live", channelId: "ignored", channelStrategy: "run" })
    ).toEqual({ mode: "live", channel_strategy: "run" });
  });

  it("Should reject missing named channels and owner-incompatible strategies", () => {
    expect(
      isNetworkParticipationDraftValid({ mode: "live", channelId: "", channelStrategy: "named" }, [
        "named",
      ])
    ).toBe(false);
    expect(
      isNetworkParticipationDraftValid(
        { mode: "live", channelId: "", channelStrategy: "loop_run" },
        ["named", "run"]
      )
    ).toBe(false);
  });
});
