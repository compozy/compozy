import { describe, expect, it, vi } from "vitest";

import { createGoalAwareFetch } from "../session-goal-chat-transport";
import type { SessionGoalCommandResult } from "../../types";

function result(
  outcome: SessionGoalCommandResult["outcome"],
  reasonCode: SessionGoalCommandResult["reason_code"] = null
): SessionGoalCommandResult {
  return {
    outcome,
    reason_code: reasonCode,
    replaced_run_id: null,
    snapshot: null,
  };
}

describe("createGoalAwareFetch", () => {
  it("Should announce a later send before transport work without changing typed result ownership", async () => {
    const order: string[] = [];
    const payload = result("error", "goal_objective_required");
    const onResult = vi.fn(() => order.push("result"));
    const goalFetch = createGoalAwareFetch({
      fetch: vi.fn(async () => {
        order.push("fetch");
        return new Response(JSON.stringify(payload), {
          status: 422,
          headers: { "content-type": "application/json" },
        });
      }),
      onRequest: () => order.push("request"),
      onResult,
    });

    await goalFetch("/prompt", { method: "POST", body: JSON.stringify({ message: "/goal" }) });

    expect(order).toEqual(["request", "fetch", "result"]);
    expect(onResult).toHaveBeenCalledWith(payload, "/goal");
  });

  it.each([
    ["started", null, null],
    ["status", null, null],
    [
      "error",
      "goal_replace_required",
      "A Goal is already active. Replace it explicitly or keep the current Goal.",
    ],
    ["error", "goal_objective_required", "Add an objective after /goal, then try again."],
    ["error", "goal_objective_too_large", "Shorten the Goal objective, then try again."],
  ] as const)(
    "Should settle the %s structured outcome without AI stream decoding",
    async (outcome, reason, expectedFailure) => {
      const payload = result(outcome, reason);
      const source = vi.fn(
        async () =>
          new Response(JSON.stringify(payload), {
            status: outcome === "error" ? 409 : outcome === "started" ? 202 : 200,
            headers: { "content-type": "application/json; charset=utf-8" },
          })
      );
      const onResult = vi.fn();
      const goalFetch = createGoalAwareFetch({ fetch: source, onResult });

      const response = await goalFetch("/prompt", {
        method: "POST",
        body: JSON.stringify({
          messages: [{ role: "user", parts: [{ type: "text", text: "/goal ship it" }] }],
        }),
      });

      expect(onResult).toHaveBeenCalledWith(payload, "/goal ship it");
      if (outcome === "error") {
        expect(response.status).toBe(409);
        expect(response.headers.get("content-type")).toContain("text/plain");
        await expect(response.text()).resolves.toBe(expectedFailure);
        expect(expectedFailure).not.toBe(reason);
        return;
      }

      expect(response.headers.get("content-type")).toBe("text/event-stream");
      const stream = await response.text();
      expect(stream).toContain('"type":"start"');
      expect(stream).toContain('"type":"finish"');
      expect(stream).not.toContain('"outcome"');
    }
  );

  it("Should preserve direct message text for a structured replacement result", async () => {
    const payload = result("replaced");
    const onResult = vi.fn();
    const goalFetch = createGoalAwareFetch({
      fetch: vi.fn(
        async () =>
          new Response(JSON.stringify(payload), {
            headers: { "content-type": "application/problem+json" },
          })
      ),
      onResult,
    });

    await goalFetch("/prompt", {
      method: "POST",
      body: JSON.stringify({ message: "/goal replace run_1 ship it" }),
    });
    expect(onResult).toHaveBeenCalledWith(payload, "/goal replace run_1 ship it");
  });

  it("Should use operator-safe guidance for a malformed future reason code", async () => {
    const payload = {
      ...result("error"),
      reason_code: "goal_future_unknown",
    };
    const goalFetch = createGoalAwareFetch({
      fetch: vi.fn(
        async () =>
          new Response(JSON.stringify(payload), {
            status: 409,
            headers: { "content-type": "application/json" },
          })
      ),
      onResult: vi.fn(),
    });

    const response = await goalFetch("/prompt", { method: "POST" });

    await expect(response.text()).resolves.toBe(
      "Goal command failed. Refresh Goal status, then retry."
    );
  });

  it("Should preserve the typed unavailable-judge result and return actionable configuration guidance", async () => {
    const payload = result("error", "goal_judge_unavailable");
    const onResult = vi.fn();
    const goalFetch = createGoalAwareFetch({
      fetch: vi.fn(
        async () =>
          new Response(JSON.stringify(payload), {
            status: 422,
            headers: { "content-type": "application/json" },
          })
      ),
      onResult,
    });

    const response = await goalFetch("/prompt", {
      method: "POST",
      body: JSON.stringify({ message: "/goal ship the verified release" }),
    });

    expect(onResult).toHaveBeenCalledWith(payload, "/goal ship the verified release");
    await expect(response.text()).resolves.toBe(
      "Goal judge is not configured. Set loops.defaults.delivery.model_defaults.judge or use a session created with an explicit model, then retry."
    );
  });

  it("Should return normal and draft streams untouched", async () => {
    const normal = new Response("data: normal\n\n", {
      headers: { "content-type": "text/event-stream" },
    });
    const draft = new Response("data: draft\n\n", {
      headers: { "content-type": "text/event-stream" },
    });
    const source = vi.fn().mockResolvedValueOnce(normal).mockResolvedValueOnce(draft);
    const onResult = vi.fn();
    const goalFetch = createGoalAwareFetch({ fetch: source, onResult });

    await expect(goalFetch("/prompt", { method: "POST" })).resolves.toBe(normal);
    await expect(
      goalFetch("/prompt", {
        method: "POST",
        body: JSON.stringify({ message: "/goal draft expand this" }),
      })
    ).resolves.toBe(draft);
    expect(onResult).not.toHaveBeenCalled();
  });

  it("Should leave unrelated JSON responses untouched", async () => {
    const response = new Response(JSON.stringify({ prompt: { status: "queued" } }), {
      headers: { "content-type": "application/json" },
    });
    const onResult = vi.fn();
    const goalFetch = createGoalAwareFetch({ fetch: vi.fn(async () => response), onResult });

    await expect(goalFetch("/prompt", { method: "POST", body: "not-json" })).resolves.toBe(
      response
    );
    expect(onResult).not.toHaveBeenCalled();
  });

  it("Should preserve a malformed JSON response without consuming its body", async () => {
    const response = new Response("{not-json", {
      headers: { "content-type": "application/json" },
    });
    const onResult = vi.fn();
    const goalFetch = createGoalAwareFetch({ fetch: vi.fn(async () => response), onResult });

    await expect(goalFetch("/prompt", { method: "POST" })).resolves.toBe(response);
    await expect(response.text()).resolves.toBe("{not-json");
    expect(onResult).not.toHaveBeenCalled();
  });
});
