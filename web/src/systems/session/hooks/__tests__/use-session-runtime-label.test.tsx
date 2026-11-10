// Suite: Session runtime label (UT-029)
// Invariant: a running session whose provider was turned off keeps its provider's display name —
// `/api/providers` is an inventory read and still lists disabled providers.
// Boundary IN: the hook and the provider/model-catalog adapters over a stubbed `fetch`.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useSessionRuntimeLabel } from "../use-session-runtime-label";

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" } });
}

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: Request) => {
      const { pathname } = new URL(input.url);
      if (pathname === "/api/providers") {
        return json({
          providers: [
            {
              name: "gemini",
              display_name: "Gemini CLI",
              enabled: false,
              default: false,
              reviewer_mode: "auto",
              auth_status: {
                mode: "none",
                env_policy: "filtered",
                home_policy: "operator",
                state: "none",
                login: { configured: false, presence: "unknown" },
              },
            },
          ],
        });
      }
      return json({ models: [] });
    })
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useSessionRuntimeLabel", () => {
  it("Should name a running session's provider even after it was disabled (UT-029)", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(
      () => useSessionRuntimeLabel({ provider: "gemini", model: "", reasoning_effort: "" }),
      { wrapper }
    );

    await waitFor(() => expect(result.current?.label).toBe("Gemini CLI"));
  });
});
