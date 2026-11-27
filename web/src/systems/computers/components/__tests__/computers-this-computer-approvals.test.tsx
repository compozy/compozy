import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ComputersThisComputerSection } from "../computers-this-computer-section";
import { useLocalRemoteControls } from "../../hooks/use-local-remote-controls";
import { toLocalComputerView } from "../../lib/computer-view";
import type { AccountStateFixture } from "../../mocks/remote-computers-types";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const PRO_ACCOUNT: AccountStateFixture = {
  signedIn: true,
  accountId: "acc-1",
  name: "Pedro",
  email: "pedro@example.com",
  provider: "google",
  source: "session",
  tokenName: null,
  marketingOptIn: false,
  installationLinked: true,
  plan: "pro",
  remoteComputersUsed: 1,
  remoteComputersLimit: 10,
  cloudRoutesUsed: 0,
  cloudRoutesLimit: 5,
  paymentIssue: false,
  cancelAtPeriodEnd: false,
  renewsAt: null,
  endsAt: null,
  formerSubscriber: false,
  cloudUnreachable: false,
  hostedRouteGraceDaysLeft: null,
};

function ThisComputerHarness({ currentDeviceIsAdmin = true }: { currentDeviceIsAdmin?: boolean }) {
  const localRemote = useLocalRemoteControls({
    signedIn: true,
    currentDeviceId: "dev-desktop",
    currentDeviceIsAdmin,
  });
  const local = toLocalComputerView(
    { run_agents: true, load_balancing: false, load_balancing_weight: 50 },
    { name: "Studio Mac", remote: localRemote.remoteStatus }
  );

  return (
    <ComputersThisComputerSection
      account={PRO_ACCOUNT}
      defaultDevicesOpen
      devices={localRemote.localDevices}
      hasRemoteComputers
      local={local}
      onApprovePending={approvalId => localRemote.handleApproveLocalPending(approvalId)}
      onDenyPending={approvalId => localRemote.handleDenyLocalPending(approvalId)}
      onRequestRevokeDevice={localRemote.setRevokeTarget}
      onRequestToggleRunAgents={() => undefined}
      onToggleDeviceScope={localRemote.handleToggleLocalDeviceScope}
      onToggleHostedRoute={localRemote.handleToggleHostedRoute}
      onToggleRemoteEnabled={localRemote.handleToggleRemoteEnabled}
      onToggleRequireApproval={localRemote.handleToggleRequireApproval}
      pendingApprovals={localRemote.localPendingApprovals}
    />
  );
}

describe("This computer → Devices approvals & Hosted route (U1)", () => {
  beforeEach(() => {
    vi.stubGlobal("EventSource", undefined);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("displays pending browser approval and approves via admin-signed POST /api/account/approvals/{id}/decision", async () => {
    const user = userEvent.setup();
    const decisions: Array<{ path: string; body: unknown }> = [];
    const patches: Array<{ path: string; body: unknown }> = [];
    let cloudRouteOn = false;

    vi.stubGlobal(
      "fetch",
      vi.fn<typeof globalThis.fetch>().mockImplementation(async (input, init) => {
        const request = input instanceof Request ? input : new Request(String(input), init);
        const path = new URL(request.url, window.location.origin).pathname;

        if (path === "/api/remote" && request.method === "GET") {
          return jsonResponse({
            enabled: true,
            computer_id: "cmp-studio",
            require_device_approval: true,
            cloud_route: {
              enabled: cloudRouteOn,
              state: cloudRouteOn ? "ready" : "off",
              hostname: cloudRouteOn ? "studio.compozy.run" : null,
            },
            direct: { enabled: true, addresses: [] },
          });
        }
        if (path === "/api/remote" && request.method === "PATCH") {
          const body = (await request.json()) as {
            cloud_route?: boolean;
            require_device_approval?: boolean;
          };
          patches.push({ path, body });
          if (typeof body.cloud_route === "boolean") cloudRouteOn = body.cloud_route;
          return jsonResponse({
            enabled: true,
            computer_id: "cmp-studio",
            require_device_approval: body.require_device_approval ?? true,
            cloud_route: {
              enabled: cloudRouteOn,
              state: cloudRouteOn ? "ready" : "off",
              hostname: cloudRouteOn ? "studio.compozy.run" : null,
            },
            direct: { enabled: true, addresses: [] },
          });
        }
        if (path === "/api/remote/devices") {
          return jsonResponse({
            devices: [
              {
                id: "dev-desktop",
                name: "MacBook Pro",
                admin: true,
                scopes: ["sessions:operate", "terminal:operate"],
                approvals: [],
              },
            ],
          });
        }
        if (path === "/api/account/devices") {
          return jsonResponse({
            devices: [
              {
                id: "dev-desktop",
                name: "MacBook Pro",
                kind: "desktop",
                admin: true,
                this_device: true,
              },
            ],
          });
        }
        if (path === "/api/account/approvals") {
          return jsonResponse({
            approvals: [
              {
                id: "apr-iphone-1",
                computer_id: "cmp-studio",
                device_name: "iPhone (Safari)",
                device_kind: "browser",
                jkt: "jkt-fingerprint-1234",
                created_at: new Date().toISOString(),
                state: "pending",
              },
            ],
          });
        }
        if (path === "/api/account/approvals/apr-iphone-1/decision" && request.method === "POST") {
          decisions.push({ path, body: await request.json() });
          return jsonResponse({ id: "apr-iphone-1", state: "decided" });
        }
        return jsonResponse({}, 404);
      })
    );

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <ThisComputerHarness currentDeviceIsAdmin />
      </QueryClientProvider>
    );

    await expect(screen.findByText("iPhone (Safari)")).resolves.toBeVisible();
    expect(screen.getByText("1 waiting for approval")).toBeInTheDocument();

    // Approve through the admin-signed decision API.
    await user.click(screen.getByRole("button", { name: "Approve" }));
    await waitFor(() => expect(decisions).toHaveLength(1));
    expect(decisions[0]).toEqual({
      path: "/api/account/approvals/apr-iphone-1/decision",
      body: { approve: true },
    });

    // Toggle Hosted route ON and verify Hosted route ready + hostname renders.
    await user.click(screen.getByRole("switch", { name: "Hosted route" }));
    await waitFor(() =>
      expect(
        patches.some(item => (item.body as { cloud_route?: boolean }).cloud_route === true)
      ).toBe(true)
    );
    await expect(screen.findByText("Hosted route ready")).resolves.toBeVisible();
    expect(screen.getByText("studio.compozy.run")).toBeInTheDocument();
  });
});
