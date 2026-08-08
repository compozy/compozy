// Suite: explicit gateway audit execution
// Invariant: every Run audit action waits for a fresh daemon response and never presents cached
// report data as the result of the new run.
// Owning layer: the gateway audit query hook.
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { gatewayAuditFixture } from "../../mocks/fixtures";
import { gatewayKeys } from "../../lib/query-keys";
import { useGatewayAudit } from "../use-gateway-audit";

describe("useGatewayAudit", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("Should wait for a fresh report before completing the first run after remount", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const cached = gatewayAuditFixture({ local_only: false });
    const fresh = gatewayAuditFixture({ local_only: true });
    queryClient.setQueryData(gatewayKeys.audit(), cached);
    let resolveFetch: (response: Response) => void = () => {};
    vi.stubGlobal(
      "fetch",
      vi.fn(
        () =>
          new Promise<Response>(resolve => {
            resolveFetch = resolve;
          })
      )
    );
    const wrapper = ({ children }: PropsWithChildren) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useGatewayAudit(), { wrapper });

    expect(result.current.hasRun).toBe(false);
    act(() => result.current.run());
    await waitFor(() => expect(result.current.isRunning).toBe(true));
    expect(result.current.hasRun).toBe(false);

    resolveFetch(
      new Response(JSON.stringify(fresh), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    );
    await waitFor(() => expect(result.current.hasRun).toBe(true));
    expect(result.current.report).toEqual(fresh);
  });
});
