// Suite: gateway device record removal
// Invariant: removal deletes the record through the record route and drops that row from the
// inventory on success. A 404 means the record is already gone, so the inventory is refreshed and no
// failure is reported. Any other failure surfaces and leaves the row in place.
// Owning layer: the gateway device mutation hooks.
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { toast } from "@compozy/ui";
import type { PropsWithChildren } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";

import { gatewayDeviceFixture, gatewayStatusFixture } from "../../mocks/fixtures";
import { gatewayKeys } from "../../lib/query-keys";
import { useRemoveGatewayDevice } from "../use-gateway-mutations";

const phone = gatewayDeviceFixture();
const laptop = gatewayDeviceFixture({ id: "dev_laptop", name: "Work laptop" });

function setup() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  queryClient.setQueryData(gatewayKeys.devices(), [phone, laptop]);
  queryClient.setQueryData(
    gatewayKeys.status(),
    gatewayStatusFixture({ devices: [phone, laptop] })
  );
  function wrapper({ children }: PropsWithChildren) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  }
  const { result } = renderHook(() => useRemoveGatewayDevice(), { wrapper });
  return { queryClient, result };
}

describe("useRemoveGatewayDevice", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("Should delete the record through the record route and drop its row from the inventory", async () => {
    const success = vi.spyOn(toast, "success").mockReturnValue("toast");
    mockJsonResponse({ canceled: 1, device: phone, revoked: true });
    const { queryClient, result } = setup();

    act(() => result.current.mutate({ id: "dev_phone", name: "Work phone" }));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    await expectFetchRequest({
      method: "DELETE",
      path: "/api/gateway/devices/dev_phone/record",
    });
    expect(queryClient.getQueryData(gatewayKeys.devices())).toEqual([laptop]);
    expect(queryClient.getQueryState(gatewayKeys.devices())?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(gatewayKeys.status())?.isInvalidated).toBe(true);
    expect(success).toHaveBeenCalledWith("Work phone removed");
  });

  it("Should treat an already-removed record as settled: refresh the inventory without an error", async () => {
    const success = vi.spyOn(toast, "success").mockReturnValue("toast");
    mockJsonResponse(
      { error: "gateway device not found", code: "gateway_device_not_found" },
      { status: 404 }
    );
    const { queryClient, result } = setup();

    act(() => result.current.mutate({ id: "dev_phone", name: "Work phone" }));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.error).toBeNull();
    expect(queryClient.getQueryData(gatewayKeys.devices())).toEqual([laptop]);
    expect(queryClient.getQueryState(gatewayKeys.devices())?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(gatewayKeys.status())?.isInvalidated).toBe(true);
    expect(success).not.toHaveBeenCalled();
  });

  it("Should surface an unexpected failure and keep the row in place", async () => {
    const success = vi.spyOn(toast, "success").mockReturnValue("toast");
    mockJsonResponse({ error: "database locked" }, { status: 500 });
    const { queryClient, result } = setup();

    act(() => result.current.mutate({ id: "dev_phone", name: "Work phone" }));

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(queryClient.getQueryData(gatewayKeys.devices())).toEqual([phone, laptop]);
    expect(success).not.toHaveBeenCalled();
  });
});
