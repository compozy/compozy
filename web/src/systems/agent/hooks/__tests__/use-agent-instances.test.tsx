import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";

import { agentKeys } from "../../lib/query-keys";
import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import { applyAgentInstanceUpdate } from "../use-agent-instances";

describe("applyAgentInstanceUpdate", () => {
  it("Should patch a known instance in place from agent_instance.updated", () => {
    const client = new QueryClient();
    const first = agentInstanceFixture({ id: "a" });
    const second = agentInstanceFixture({ id: "b" });
    client.setQueryData(agentKeys.instances(), [first, second]);

    applyAgentInstanceUpdate(client, { ...second, activity: "working" });

    expect(client.getQueryData(agentKeys.instances())).toEqual([
      first,
      { ...second, activity: "working" },
    ]);
  });

  it("Should reread for an instance it hasn't seen", () => {
    const client = new QueryClient();
    client.setQueryData(agentKeys.instances(), []);
    const invalidate = vi.spyOn(client, "invalidateQueries");

    applyAgentInstanceUpdate(client, agentInstanceFixture());

    expect(invalidate).toHaveBeenCalledWith({ queryKey: agentKeys.instances() });
  });
});
