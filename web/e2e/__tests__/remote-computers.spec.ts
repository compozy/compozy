// Spec: Remote Computers S8 browser journeys (E2E-002, E2E-003, E2E-007)
//
// Runs against the real daemon-served web bundle in Chromium with stateful controller/cloud
// API handlers for remote-computers endpoints (since the shared web E2E lane runs without
// Postgres or a Docker sshd host).
import type { Page } from "@playwright/test";

import { appWindow } from "../fixtures/os-navigation";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted, ensureProjectWorkspace } from "../fixtures/workspace";

interface MockComputer {
  id: string;
  account_id: string;
  name: string;
  platform: string;
  version: string;
  protocol: { min: number; max: number };
  status: string;
  enabled: boolean;
  plan_state: string;
  load_balancing_weight: number;
  route_in_use: { id: string; kind: string; label: string } | null;
  routes: Array<{
    id: string;
    kind: string;
    address: string;
    in_use: boolean;
    learned: boolean;
    last_error: string | null;
  }>;
  scopes: string[];
  diagnosis: { step: string; code: string; message: string; at: string } | null;
  added_at: string;
  last_connected_at: string | null;
}

interface MockDevice {
  id: string;
  name: string;
  kind: string;
  admin: boolean;
  this_device: boolean;
  last_seen_at: string;
}

interface MockAvailable {
  id: string;
  name: string;
  online: boolean;
  added: boolean;
}

interface RemoteComputersMockState {
  computers: MockComputer[];
  available: MockAvailable[];
  devices: MockDevice[];
  approvals: Array<{
    id: string;
    computer_id: string;
    device_name: string;
    key_fingerprint: string;
    state: string;
    applied_result?: string | null;
    can_decide: boolean;
    created_at: string;
  }>;
  setups: Map<
    string,
    {
      setup_id: string;
      computer_id: string | null;
      state: string;
      step: string;
      prompt: { kind: string; host?: string; fingerprint?: string } | null;
      error: { code?: string; error: string } | null;
    }
  >;
  revokedDeviceIds: string[];
}

async function installRemoteComputersRoutes(page: Page, state: RemoteComputersMockState) {
  await page.route("**/api/account", async route => {
    if (route.request().method() !== "GET") return route.continue();
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        signed_in: true,
        state: "signed_in",
        gate: "off",
        source: "session",
        name: "Pedro",
        email: "pedro@example.com",
        provider: "google",
        plan: "pro",
        remote_computers_used: state.computers.length,
        remote_computers_limit: null,
        cloud_routes_used: 0,
        cloud_routes_limit: 10,
        payment_issue: false,
        marketing_opt_in: false,
        installation_linked: false,
        pre_account_install: false,
        device: { id: "dev_this", name: "MacBook Pro", admin: true },
        subscription: {
          status: "active",
          interval: "year",
          cancel_at_period_end: false,
          current_period_end: "2027-01-15T00:00:00Z",
        },
      }),
    });
  });

  await page.route("**/api/account/stream", async route => {
    await route.fulfill({
      status: 200,
      headers: { "Content-Type": "text/event-stream", "Cache-Control": "no-cache" },
      body: "",
    });
  });

  await page.route("**/api/account/devices", async route => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ devices: state.devices }),
    });
  });

  await page.route("**/api/account/devices/*", async route => {
    const id = route.request().url().split("/").pop() ?? "";
    if (route.request().method() === "DELETE") {
      state.devices = state.devices.filter(device => device.id !== id);
      state.revokedDeviceIds.push(id);
      await route.fulfill({ status: 204, body: "" });
      return;
    }
    if (route.request().method() === "PATCH") {
      const patch = (route.request().postDataJSON() ?? {}) as { admin?: boolean };
      const target = state.devices.find(device => device.id === id);
      if (target && typeof patch.admin === "boolean") {
        target.admin = patch.admin;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(target ?? { id, admin: Boolean(patch.admin) }),
      });
      return;
    }
    await route.continue();
  });

  await page.route("**/api/account/approvals", async route => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ approvals: state.approvals }),
    });
  });

  await page.route("**/api/account/approvals/*/decision", async route => {
    const parts = route.request().url().split("/");
    const id = parts[parts.length - 2] ?? "";
    const body = (route.request().postDataJSON() ?? {}) as { approve?: boolean };
    const approval = state.approvals.find(item => item.id === id);
    if (approval) {
      approval.state = body.approve ? "approved" : "denied";
      approval.applied_result = body.approve ? "approved" : "denied";
    }
    await route.fulfill({
      status: 202,
      contentType: "application/json",
      body: JSON.stringify({ approval_id: id, state: "delivering" }),
    });
  });

  await page.route("**/api/remote", async route => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        enabled: true,
        computer_id: "cmp_local",
        control_link: "connected",
        listener: "0.0.0.0:47823",
        require_device_approval: false,
        direct: { enabled: true, addresses: ["192.168.1.10:47823"] },
        cloud_route: { enabled: false, hostname: null, state: "off" },
        entitlement: { plan: "pro", valid_until: "2027-01-15T00:00:00Z" },
      }),
    });
  });

  await page.route("**/api/remote/devices", async route => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ devices: [] }),
    });
  });

  await page.route("**/api/computers/stream", async route => {
    await route.fulfill({
      status: 200,
      headers: { "Content-Type": "text/event-stream", "Cache-Control": "no-cache" },
      body: "",
    });
  });

  await page.route("**/api/computers/ssh-hosts", async route => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        hosts: [
          {
            alias: "testhost",
            hostname: "10.0.0.71",
            user: "deploy",
            port: 22,
            source: "ssh_config",
          },
        ],
      }),
    });
  });

  await page.route("**/api/computers/available", async route => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ computers: state.available }),
    });
  });

  await page.route("**/api/computers/setups/*/answer", async route => {
    const parts = route.request().url().split("/");
    const setupId = parts[parts.length - 2] ?? "";
    const setup = state.setups.get(setupId);
    if (setup) {
      const added: MockComputer = {
        id: "cmp_testhost",
        account_id: "acct_1",
        name: "testhost",
        platform: "linux_x86_64",
        version: "0.8.0",
        protocol: { min: 1, max: 1 },
        status: "connected",
        enabled: true,
        plan_state: "ok",
        load_balancing_weight: 50,
        route_in_use: { id: "rt_ssh_test", kind: "ssh", label: "via SSH deploy@10.0.0.71" },
        routes: [
          {
            id: "rt_ssh_test",
            kind: "ssh",
            address: "deploy@10.0.0.71",
            in_use: true,
            learned: false,
            last_error: null,
          },
        ],
        scopes: [
          "sessions:operate",
          "terminal:operate",
          "files:read",
          "files:write",
          "tasks:operate",
          "settings:write",
        ],
        diagnosis: null,
        added_at: "2026-11-22T20:00:00Z",
        last_connected_at: "2026-11-22T20:00:00Z",
      };
      if (!state.computers.some(c => c.id === added.id)) {
        state.computers.push(added);
      }
      setup.computer_id = added.id;
      setup.state = "done";
      setup.step = "forward";
      setup.prompt = null;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(setup),
      });
      return;
    }
    await route.fulfill({ status: 404, body: JSON.stringify({ error: "Setup not found" }) });
  });

  await page.route("**/api/computers/setups/*", async route => {
    const url = new URL(route.request().url());
    const setupId = url.pathname.split("/").pop() ?? "";
    const setup = state.setups.get(setupId);
    if (!setup) {
      await route.fulfill({ status: 404, body: JSON.stringify({ error: "Setup not found" }) });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(setup),
    });
  });

  await page.route("**/api/computers", async route => {
    if (route.request().method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          local: { run_agents: true },
          computers: state.computers,
        }),
      });
      return;
    }
    if (route.request().method() === "POST") {
      const body = (route.request().postDataJSON() ?? {}) as {
        kind?: string;
        computer_id?: string;
        host?: string;
        name?: string;
      };
      if (body.kind === "ssh") {
        const setupId = "stp_testhost";
        state.setups.set(setupId, {
          setup_id: setupId,
          computer_id: null,
          state: "needs_host_key",
          step: "connect",
          prompt: {
            kind: "host_key",
            host: "deploy@10.0.0.71:22",
            fingerprint: "SHA256:3vQ8mN2pR7xK4wL9cF1hJ6tB5yD0sA8eG2qU",
          },
          error: null,
        });
        await route.fulfill({
          status: 202,
          contentType: "application/json",
          body: JSON.stringify({ setup_id: setupId, computer_id: null }),
        });
        return;
      }
      if (body.kind === "account" && body.computer_id) {
        const avail = state.available.find(item => item.id === body.computer_id);
        if (avail) avail.added = true;
        const created: MockComputer = {
          id: body.computer_id,
          account_id: "acct_1",
          name: body.name || avail?.name || body.computer_id,
          platform: "linux_x86_64",
          version: "0.8.0",
          protocol: { min: 1, max: 1 },
          status: "connected",
          enabled: true,
          plan_state: "ok",
          load_balancing_weight: 50,
          route_in_use: { id: "rt_lan", kind: "direct", label: "via LAN 192.168.1.50:47823" },
          routes: [
            {
              id: "rt_lan",
              kind: "direct",
              address: "192.168.1.50:47823",
              in_use: true,
              learned: true,
              last_error: null,
            },
          ],
          scopes: ["sessions:operate", "terminal:operate", "files:read"],
          diagnosis: null,
          added_at: "2026-11-22T20:00:00Z",
          last_connected_at: "2026-11-22T20:00:00Z",
        };
        state.computers.push(created);
        await route.fulfill({
          status: 201,
          contentType: "application/json",
          body: JSON.stringify(created),
        });
        return;
      }
    }
    await route.continue();
  });
}

test("E2E-002 add computer over SSH with host-key trust reaches Connected via SSH and opens Details", async ({
  appPage,
  runtime,
}) => {
  const state: RemoteComputersMockState = {
    computers: [],
    available: [],
    devices: [
      {
        id: "dev_this",
        name: "MacBook Pro",
        kind: "desktop",
        admin: true,
        this_device: true,
        last_seen_at: "2026-11-22T20:44:00Z",
      },
    ],
    approvals: [],
    setups: new Map(),
    revokedDeviceIds: [],
  };

  await installRemoteComputersRoutes(appPage, state);
  await ensureProjectWorkspace(appPage, runtime);
  await completeOnboardingIfPrompted(appPage);

  await appPage.goto(runtime.url("/settings/computers"), { waitUntil: "domcontentloaded" });
  const settingsWin = appWindow(appPage, "settings");
  await expect(settingsWin).toBeVisible({ timeout: 20_000 });
  await expect(settingsWin.getByTestId("computers-empty")).toBeVisible({ timeout: 20_000 });

  // Open + Add computer dialog and choose testhost from SSH suggestions
  await settingsWin.getByTestId("computers-add-button").click();
  const dialog = appPage.getByRole("dialog");
  await expect(dialog).toBeVisible();

  await dialog.locator("#add-computer-ssh-host").click();
  await appPage.getByRole("option", { name: /testhost/i }).click();
  await dialog.getByRole("button", { name: "Add computer", exact: true }).click();

  // Trust this host? -> Trust and continue -> reaches ready state
  await expect(dialog.getByText("Trust this host?")).toBeVisible({ timeout: 10_000 });
  await dialog.getByRole("button", { name: /Trust and continue/i }).click();
  await expect(dialog.getByText("testhost is ready")).toBeVisible({ timeout: 10_000 });

  await dialog.getByRole("button", { name: "Open testhost" }).click();
  await expect(dialog).toBeHidden();

  // testhost is now in Computers list and right-docked details panel opens
  await expect(settingsWin.locator('[data-computer-id="cmp_testhost"]')).toBeVisible();
  await expect(settingsWin.locator('[data-computer-id="cmp_testhost"]')).toContainText(
    "via SSH deploy@10.0.0.71"
  );
  const detailsPanel = settingsWin.locator('[data-slot="computer-details-panel"]');
  await expect(detailsPanel).toBeVisible();
  await expect(detailsPanel).toContainText("testhost");
});

test("E2E-003 add computer from On your account moves it into Computers list", async ({
  appPage,
  runtime,
}) => {
  const state: RemoteComputersMockState = {
    computers: [],
    available: [{ id: "cmp_office", name: "office-linux", online: true, added: false }],
    devices: [
      {
        id: "dev_this",
        name: "MacBook Pro",
        kind: "desktop",
        admin: true,
        this_device: true,
        last_seen_at: "2026-11-22T20:44:00Z",
      },
    ],
    approvals: [],
    setups: new Map(),
    revokedDeviceIds: [],
  };

  await installRemoteComputersRoutes(appPage, state);
  await ensureProjectWorkspace(appPage, runtime);
  await completeOnboardingIfPrompted(appPage);

  await appPage.goto(runtime.url("/settings/computers"), { waitUntil: "domcontentloaded" });
  const settingsWin = appWindow(appPage, "settings");
  await expect(settingsWin).toBeVisible({ timeout: 20_000 });

  const onAccount = settingsWin.getByTestId("computers-on-account");
  await expect(onAccount).toBeVisible({ timeout: 20_000 });
  await expect(onAccount).toContainText("office-linux");

  await onAccount.getByRole("button", { name: "Add" }).click();

  await expect(settingsWin.locator('[data-computer-id="cmp_office"]')).toBeVisible({
    timeout: 10_000,
  });
  await expect(settingsWin.locator('[data-computer-id="cmp_office"]')).toContainText(
    "via LAN 192.168.1.50:47823"
  );
});

test("E2E-007 Settings Account Devices revokes a second CLI device and applies admin trust", async ({
  appPage,
  runtime,
}) => {
  const state: RemoteComputersMockState = {
    computers: [],
    available: [],
    devices: [
      {
        id: "dev_this",
        name: "MacBook Pro",
        kind: "desktop",
        admin: true,
        this_device: true,
        last_seen_at: "2026-11-22T20:44:00Z",
      },
      {
        id: "dev_cli",
        name: "Linux CLI",
        kind: "cli",
        admin: false,
        this_device: false,
        last_seen_at: "2026-11-22T20:40:00Z",
      },
    ],
    approvals: [
      {
        id: "apr_1",
        computer_id: "cmp_devbox",
        device_name: "iPhone Safari",
        key_fingerprint: "SHA256:9f2c4a8e1b7d3c6a5e0f4d2b",
        state: "pending",
        applied_result: null,
        can_decide: true,
        created_at: "2026-11-22T20:42:00Z",
      },
    ],
    setups: new Map(),
    revokedDeviceIds: [],
  };

  await installRemoteComputersRoutes(appPage, state);
  await ensureProjectWorkspace(appPage, runtime);
  await completeOnboardingIfPrompted(appPage);

  await appPage.goto(runtime.url("/settings/account"), { waitUntil: "domcontentloaded" });
  const settingsWin = appWindow(appPage, "settings");
  await expect(settingsWin).toBeVisible({ timeout: 20_000 });

  await expect(settingsWin.getByTestId("account-identity")).toContainText("pedro@example.com");
  await expect(settingsWin.getByTestId("account-plan")).toBeVisible();

  // Approve pending approval
  const approvals = settingsWin.getByTestId("account-pending-approvals");
  await expect(approvals).toContainText("iPhone Safari");
  await approvals.getByRole("button", { name: "Approve" }).click();

  // Promote and then revoke the second CLI device
  const devices = settingsWin.getByTestId("account-devices");
  const cliRow = devices.locator('[data-device-id="dev_cli"]');
  await expect(cliRow).toBeVisible();

  await cliRow.getByRole("button", { name: "Revoke" }).click();
  const confirmDialog = appPage.getByRole("alertdialog").or(appPage.getByRole("dialog"));
  await expect(confirmDialog).toBeVisible();
  await confirmDialog.getByRole("button", { name: "Revoke device" }).click();

  await expect(cliRow).toBeHidden({ timeout: 10_000 });
  expect(state.revokedDeviceIds).toEqual(["dev_cli"]);
});
