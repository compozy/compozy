// Suite: RunOnPicker & SessionCreateDialog remote targeting (UT-080, UT-081, UT-083, UT-084, UT-106)
// Invariant: Selecting a remote computer queries ["computer", id, "workspaces"], an offline
//            computer disables Send with "<name> is offline" while preserving draft text,
//            Needs Pro computers are non-selectable, single-computer installations hide RunOnPicker,
//            local execution off omits This computer, and Auto with no eligible computer disables
//            Send with "No computer can run this project".
// Boundary IN: RunOnPicker, RunOnNotices, runOnOptions, resolveEffectiveRunOnTarget,
//              useComputerWorkspaces, and SessionCreateDialog.
// Boundary OUT: Backend HTTP bridge and remote daemon responses.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Alert, AlertDescription } from "@compozy/ui";
import { computerQueryKey, type ComputerId } from "@/lib/computer-scope";
import * as workspaceApi from "@/systems/workspace/adapters/workspace-api";
import { SessionCreateDialog } from "@/systems/session/components/session-create-dialog";
import type { ComputerFixture, LocalComputerFixture } from "../../mocks/remote-computers-types";
import { useComputerWorkspaces } from "../../hooks/use-computer-workspaces";
import { runOnOptions } from "../../lib/computer-view";
import {
  describeNoEligibleComputerReason,
  resolveEffectiveRunOnTarget,
} from "../../lib/placement-view";
import { RunOnNotices } from "../run-on-notices";
import { RunOnPicker } from "../run-on-picker";

const LOCAL_COMPUTER: LocalComputerFixture = {
  id: "local",
  name: "MacBook Pro",
  platform: "darwin_arm64",
  iconKind: "laptop",
  version: "0.8.0",
  localRunAgents: true,
  activeLocalSessions: 0,
  remoteEnabled: false,
  hostedRouteEnabled: false,
  hostedRouteState: "off",
  hostedUrl: "",
  requireDeviceApproval: false,
  load: {
    cpuCount: 8,
    cpuUtil: 0.2,
    memTotalBytes: 16e9,
    memAvailableBytes: 8e9,
    sampledAt: "2026-04-10T00:00:00Z",
    sampledAgoLabel: "2s ago",
  },
  loadBalancingWeight: 50,
  projects: [],
};

const DEVBOX_CONNECTED: ComputerFixture = {
  id: "cmp_7f3k2q9x",
  name: "devbox",
  platform: "linux_x86_64",
  iconKind: "server",
  version: "0.8.0",
  protocol: { min: 1, max: 1 },
  status: "connected",
  enabled: true,
  planState: "ok",
  originKind: "ssh",
  routeInUse: {
    id: "rt_1",
    kind: "direct",
    address: "10.0.0.42:7844",
    label: "via LAN",
    inUse: true,
    learned: false,
    lastError: null,
  },
  routes: [],
  scopes: ["sessions:operate"],
  load: null,
  loadBalancingWeight: 50,
  diagnosis: null,
  providers: [],
  projects: [],
  addedAt: "2026-04-01T00:00:00Z",
  lastConnectedAt: "2026-04-10T00:00:00Z",
  lastConnectedLabel: "just now",
};

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("RunOnPicker (UT-081, UT-083, UT-084)", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("UT-081: Should re-query projects with key ['computer', 'cmp_7f3k2q9x', 'workspaces'] when choosing devbox", async () => {
    const user = userEvent.setup();
    const fetchWorkspacesSpy = vi.spyOn(workspaceApi, "fetchWorkspaces").mockResolvedValue([]);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    function Harness() {
      const [computerId, setComputerId] = useState<ComputerId>("local");
      useComputerWorkspaces(computerId, true);
      return (
        <RunOnPicker
          allowAuto={false}
          computers={[DEVBOX_CONNECTED]}
          hideWhenSingleComputer
          localComputer={LOCAL_COMPUTER}
          onChange={next => {
            if (next !== "auto") setComputerId(next);
          }}
          value={computerId}
        />
      );
    }

    render(<Harness />, { wrapper: createWrapper(queryClient) });

    await user.click(screen.getByTestId("run-on-picker"));
    await user.click(screen.getByTestId("run-on-picker-item-cmp_7f3k2q9x"));

    await waitFor(() => {
      expect(fetchWorkspacesSpy).toHaveBeenCalledWith(
        expect.any(AbortSignal),
        false,
        "cmp_7f3k2q9x"
      );
    });

    const remoteQuery = queryClient
      .getQueryCache()
      .find({ queryKey: computerQueryKey("cmp_7f3k2q9x", "workspaces"), exact: true });
    expect(remoteQuery?.queryKey).toEqual(["computer", "cmp_7f3k2q9x", "workspaces"]);
  });

  it("UT-083: Should disable submit with '<name> is offline' while preserving draft text when selected computer is offline", async () => {
    const offlineDevbox: ComputerFixture = { ...DEVBOX_CONNECTED, status: "offline" };
    const onSubmit = vi.fn();

    render(
      <SessionCreateDialog
        agents={[{ name: "general", display_name: "General", builtin: true } as never]}
        destinationLabel="api"
        destinationReady
        isAwaitingEnvironment={false}
        isSubmitting={false}
        mode="advanced"
        notice={
          <Alert data-testid="session-create-offline-notice" variant="warning">
            <AlertDescription>devbox is offline</AlertDescription>
          </Alert>
        }
        onAgentChange={() => undefined}
        onCancelEnvironment={() => undefined}
        onModeChange={() => undefined}
        onOpenChange={() => undefined}
        onSessionNameChange={() => undefined}
        onSubmit={onSubmit}
        open
        restoreFocusOnClose={false}
        runOn={
          <RunOnPicker
            allowAuto={false}
            computers={[offlineDevbox]}
            localComputer={LOCAL_COMPUTER}
            onChange={() => undefined}
            value="cmp_7f3k2q9x"
          />
        }
        scope="project"
        selectedAgentName="general"
        sessionName="Investigate flaky queue"
        sessionRoot="/srv/api"
        submitDisabled
        submitError={null}
      />
    );

    expect(screen.getByTestId("session-create-offline-notice")).toHaveTextContent(
      "devbox is offline"
    );
    expect(screen.getByTestId("session-create-submit")).toBeDisabled();
    expect(screen.getByDisplayValue("Investigate flaky queue")).toBeInTheDocument();
  });

  it("UT-084: Should render nothing when only this computer exists", () => {
    const { container } = render(
      <RunOnPicker
        allowAuto={false}
        computers={[]}
        hideWhenSingleComputer
        localComputer={LOCAL_COMPUTER}
        onChange={() => undefined}
        value="local"
      />
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("UT-080: Should mark Needs Pro computers non-selectable with Needs Pro hint and inline Upgrade action", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const onUpgrade = vi.fn();
    const limitedComputer: ComputerFixture = {
      ...DEVBOX_CONNECTED,
      id: "cmp_limited",
      name: "gpu-vm",
      status: "needs_pro",
      planState: "limited",
    };

    expect(
      runOnOptions([
        { id: "cmp_limited", name: "gpu-vm", status: "needs_pro", enabled: true },
        { id: "cmp_7f3k2q9x", name: "devbox", status: "connected", enabled: true },
      ])
    ).toEqual([
      {
        id: "cmp_limited",
        name: "gpu-vm",
        status: "needs_pro",
        selectable: false,
        hint: "Needs Pro",
      },
      {
        id: "cmp_7f3k2q9x",
        name: "devbox",
        status: "connected",
        selectable: true,
      },
    ]);

    render(
      <RunOnPicker
        allowAuto={false}
        computers={[DEVBOX_CONNECTED, limitedComputer]}
        localComputer={LOCAL_COMPUTER}
        onChange={onChange}
        onUpgrade={onUpgrade}
        value="local"
      />
    );

    await user.click(screen.getByTestId("run-on-picker"));
    const limitedItem = screen.getByTestId("run-on-picker-item-cmp_limited");
    expect(limitedItem).toHaveTextContent("Needs Pro");
    expect(limitedItem).toHaveAttribute("aria-disabled", "true");

    await user.click(limitedItem);
    expect(onChange).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: /Upgrade/i }));
    expect(onUpgrade).toHaveBeenCalledWith("cmp_limited");
  });

  it("UT-106: Should show Auto → <computer> only when load balancing is on, omit This computer when local execution is off, and disable Send when no computer is eligible", async () => {
    const user = userEvent.setup();
    const localOff: LocalComputerFixture = { ...LOCAL_COMPUTER, localRunAgents: false };
    const manualOnlyDevbox: ComputerFixture = { ...DEVBOX_CONNECTED, loadBalancingWeight: 0 };

    expect(
      resolveEffectiveRunOnTarget({
        requested: "local",
        local: localOff,
        computers: [DEVBOX_CONNECTED],
        loadBalancingEnabled: true,
        allowAuto: true,
      })
    ).toBe("cmp_7f3k2q9x");

    const { rerender } = render(
      <RunOnPicker
        allowAuto
        autoResolution={{
          computerId: "cmp_7f3k2q9x",
          computerName: "devbox",
          reason: "Picks the least busy computer",
        }}
        computers={[DEVBOX_CONNECTED]}
        loadBalancingEnabled={false}
        localComputer={LOCAL_COMPUTER}
        onChange={() => undefined}
        value="local"
      />
    );

    await user.click(screen.getByTestId("run-on-picker"));
    expect(screen.queryByTestId("run-on-picker-item-auto")).not.toBeInTheDocument();

    rerender(
      <SessionCreateDialog
        agents={[{ name: "general", display_name: "General", builtin: true } as never]}
        destinationLabel="trainer"
        destinationReady
        isAwaitingEnvironment={false}
        isSubmitting={false}
        mode="simple"
        notice={
          <RunOnNotices
            autoNoEligible
            autoNoEligibleReason={describeNoEligibleComputerReason("trainer", {
              local: "local_execution_disabled",
              cmp_7f3k2q9x: "manual_only",
            })}
            projectName="trainer"
          />
        }
        onAgentChange={() => undefined}
        onCancelEnvironment={() => undefined}
        onModeChange={() => undefined}
        onOpenChange={() => undefined}
        onSessionNameChange={() => undefined}
        onSubmit={() => undefined}
        open
        restoreFocusOnClose={false}
        runOn={
          <RunOnPicker
            allowAuto
            autoResolution={null}
            computers={[manualOnlyDevbox]}
            loadBalancingEnabled
            localComputer={localOff}
            onChange={() => undefined}
            value="auto"
          />
        }
        scope="project"
        selectedAgentName="general"
        sessionName=""
        sessionRoot="/srv/trainer"
        submitDisabled
        submitError={null}
      />
    );

    expect(screen.getByTestId("auto-no-eligible-alert")).toHaveTextContent(
      "No computer can run this project."
    );
    expect(screen.getByTestId("session-create-submit")).toBeDisabled();

    await user.click(screen.getByTestId("run-on-picker"));
    expect(screen.queryByTestId("run-on-picker-item-local")).not.toBeInTheDocument();
    expect(screen.getByTestId("run-on-picker-item-auto")).toHaveTextContent(
      "No computer can run this project"
    );
  });
});
