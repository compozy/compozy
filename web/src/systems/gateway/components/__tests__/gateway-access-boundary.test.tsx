// Suite: gateway access boundary
// Invariant: an unpaired browser sees only the gate and a revoked one only the access-ended state,
// with the protected cache cancelled and dropped before that state is painted so no residual data
// can reappear. Transient failures never block the app. The pairing artifact reaches the daemon
// only in the redeem body — never in the request URL or the address bar.
// Owning layer: the app shell boundary between the transport's access signals and the router tree.
import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import userEvent from "@testing-library/user-event";
import type { PropsWithChildren } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { reportGatewayAccess } from "@/lib/gateway-access-signal";

import { gatewayAccessStore } from "../../stores/gateway-access-store";
import { GatewayAccessBoundary } from "../gateway-access-boundary";

const ARTIFACT = "cpz_gwp_2f8c1d5a9b";

function wrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: PropsWithChildren) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

function renderBoundary(queryClient: QueryClient, reload = vi.fn()) {
  render(
    <GatewayAccessBoundary reload={reload}>
      <div data-testid="protected-app">Session transcript</div>
    </GatewayAccessBoundary>,
    { wrapper: wrapper(queryClient) }
  );
  return reload;
}

describe("GatewayAccessBoundary", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    gatewayAccessStore.trigger.accessRestored();
    window.history.replaceState(null, "", "/");
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    gatewayAccessStore.trigger.accessRestored();
    window.history.replaceState(null, "", "/");
  });

  it("Should render the app while access is intact", () => {
    renderBoundary(queryClient);
    expect(screen.getByTestId("protected-app")).toBeInTheDocument();
  });

  it("Should keep the app rendered through transient failures", async () => {
    renderBoundary(queryClient);
    act(() => reportGatewayAccess(503, { error: "temporarily unavailable" }));
    act(() => reportGatewayAccess(500, { code: "gateway_device_revoked" }));

    await waitFor(() => expect(screen.getByTestId("protected-app")).toBeInTheDocument());
  });

  it("Should show only the pairing gate when this device is unauthenticated", async () => {
    renderBoundary(queryClient);
    act(() => reportGatewayAccess(401, { code: "gateway_device_unauthenticated" }));

    await screen.findByTestId("gateway-pairing-gate");
    expect(screen.queryByTestId("protected-app")).not.toBeInTheDocument();
  });

  it("Should show the access-ended state with no residual data when the device is revoked", async () => {
    queryClient.setQueryData(["sessions", "list"], [{ id: "sess_1", title: "Deploy review" }]);
    renderBoundary(queryClient);
    act(() => reportGatewayAccess(401, { code: "gateway_device_revoked" }));

    await screen.findByTestId("gateway-access-ended");
    expect(screen.queryByTestId("protected-app")).not.toBeInTheDocument();
    expect(screen.queryByText("Deploy review")).not.toBeInTheDocument();
  });

  it("Should drop settled protected data when the device is revoked", async () => {
    queryClient.setQueryData(["sessions", "list"], [{ id: "sess_1" }]);
    renderBoundary(queryClient);

    act(() => reportGatewayAccess(401, { code: "gateway_device_revoked" }));
    await screen.findByTestId("gateway-access-ended");

    await waitFor(() => expect(queryClient.getQueryData(["sessions", "list"])).toBeUndefined());
  });

  it("Should not let an in-flight protected response repopulate the cache after revocation", async () => {
    // A protected read is still in flight when the daemon ends the session, and
    // it resolves afterwards. Neither the cache nor the screen may take it.
    let releaseTranscript: (value: string) => void = () => {};
    const pending = new Promise<string>(resolve => {
      releaseTranscript = resolve;
    });

    function ProtectedRead() {
      const { data } = useQuery({
        queryKey: ["sessions", "transcript"],
        queryFn: () => pending,
      });
      return <div data-testid="protected-app">{data ?? "loading"}</div>;
    }

    render(
      <GatewayAccessBoundary reload={vi.fn()}>
        <ProtectedRead />
      </GatewayAccessBoundary>,
      { wrapper: wrapper(queryClient) }
    );
    await screen.findByTestId("protected-app");

    act(() => reportGatewayAccess(401, { code: "gateway_device_revoked" }));
    await screen.findByTestId("gateway-access-ended");

    await act(async () => {
      releaseTranscript("Deploy review transcript");
      await Promise.resolve();
    });

    await waitFor(() =>
      expect(queryClient.getQueryData(["sessions", "transcript"])).toBeUndefined()
    );
    expect(screen.queryByText("Deploy review transcript")).not.toBeInTheDocument();
    expect(screen.queryByTestId("protected-app")).not.toBeInTheDocument();
  });

  it("Should redeem a scanned artifact without ever putting it in a URL", async () => {
    window.history.replaceState(null, "", `/#pair=${ARTIFACT}`);
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ device: { id: "dev_new", name: "Work phone" } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    );
    vi.stubGlobal("fetch", fetchMock);

    const reload = renderBoundary(queryClient);
    act(() => reportGatewayAccess(401, { code: "gateway_device_unauthenticated" }));
    await screen.findByTestId("gateway-pairing-gate");

    // The scanned secret is prefilled from the fragment and immediately erased
    // from the address bar, before anything can leave the page.
    expect(screen.getByTestId("gateway-pairing-gate-code")).toHaveValue(ARTIFACT);
    expect(window.location.search).toBe("");
    expect(window.location.hash).toBe("");

    await userEvent.type(screen.getByTestId("gateway-pairing-gate-name"), "Work phone");
    await userEvent.click(screen.getByTestId("gateway-pairing-gate-submit"));

    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    const [input, init] = fetchMock.mock.calls[0] as [RequestInfo | URL, RequestInit];
    const requestUrl = input instanceof Request ? input.url : String(input);
    const body =
      init?.body !== undefined && init.body !== null
        ? String(init.body)
        : await (input as Request).clone().text();

    expect(requestUrl).not.toContain(ARTIFACT);
    expect(requestUrl).toContain("/api/gateway/pairings/redeem");
    // The artifact leaves the browser exactly once, in the request body.
    expect(body).toContain(ARTIFACT);
    await waitFor(() => expect(reload).toHaveBeenCalledTimes(1));
  });

  it("Should not redeem until both the code and a device name are supplied", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    renderBoundary(queryClient);
    act(() => reportGatewayAccess(401, { code: "gateway_device_unauthenticated" }));
    await screen.findByTestId("gateway-pairing-gate");

    await userEvent.click(screen.getByTestId("gateway-pairing-gate-submit"));
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
