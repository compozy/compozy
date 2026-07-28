import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";

import { listMemoryDecisions } from "../../adapters/knowledge-api";
import { handlers } from "../handlers";

const server = setupServer(...handlers);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe("knowledge MSW handlers", () => {
  it("Should apply decision selectors and filters before the response limit", async () => {
    const response = await listMemoryDecisions({
      scope: "global",
      filename: "operator-style.md",
      op: "update",
      since: "2026-04-17T17:00:00Z",
      limit: 1,
    });

    expect(response.decisions).toHaveLength(1);
    expect(response.decisions[0]).toMatchObject({
      id: "dec_edit_fixture",
      op: "update",
      target_filename: "operator-style.md",
    });
  });
});
