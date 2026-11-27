import { http, HttpResponse } from "msw";

import type {
  AvailableComputersResponse,
  ComputerListResponse,
  ComputerSSHHostsResponse,
  RemoteDevicesResponse,
  RemoteStatusPayload,
} from "../types";

export const DEFAULT_COMPUTERS_LIST: ComputerListResponse = {
  local: { run_agents: true, load_balancing: false, load_balancing_weight: 50 },
  computers: [
    {
      id: "cmp_devbox",
      account_id: "acct_default",
      name: "devbox",
      platform: "linux_x86_64",
      version: "0.8.0",
      protocol: { min: 1, max: 1 },
      status: "connected",
      enabled: true,
      plan_state: "ok",
      load_balancing_weight: 50,
      route_in_use: {
        id: "rt_ssh_1",
        kind: "ssh",
        label: "via SSH pedro@10.0.0.42",
      },
      routes: [
        {
          id: "rt_lan_1",
          kind: "direct",
          address: "192.168.1.42:47823",
          in_use: false,
          learned: true,
          last_error: null,
        },
        {
          id: "rt_ssh_1",
          kind: "ssh",
          address: "pedro@10.0.0.42",
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
      added_at: "2026-11-10T09:00:00Z",
      last_connected_at: "2026-11-22T20:44:00Z",
    },
  ],
};

export const DEFAULT_AVAILABLE_COMPUTERS: AvailableComputersResponse = {
  computers: [
    {
      id: "cmp_office",
      name: "office-linux",
      online: true,
      added: false,
    },
  ],
};

export const DEFAULT_SSH_HOSTS: ComputerSSHHostsResponse = {
  hosts: [
    {
      alias: "staging-box",
      hostname: "10.0.0.71",
      user: "deploy",
      port: 22,
      source: "ssh_config",
    },
  ],
};

export const DEFAULT_REMOTE_STATUS: RemoteStatusPayload = {
  enabled: true,
  computer_id: "cmp_local",
  control_link: "connected",
  listener: "0.0.0.0:47823",
  require_device_approval: false,
  direct: { enabled: true, addresses: ["192.168.1.10:47823"] },
  cloud_route: { enabled: false, hostname: null, state: "off" },
  entitlement: { plan: "pro", valid_until: "2026-12-22T00:00:00Z" },
};

export const DEFAULT_REMOTE_DEVICES: RemoteDevicesResponse = {
  devices: [
    {
      id: "dev_local",
      name: "MacBook Pro",
      admin: true,
      pending_approval: false,
      last_seen_at: "2026-11-22T20:44:00Z",
      scopes: [
        "sessions:operate",
        "terminal:operate",
        "files:read",
        "files:write",
        "tasks:operate",
        "settings:write",
        "remote:admin",
      ],
    },
  ],
};

export const computersHandlers = [
  http.get("/api/migration-notices", () => HttpResponse.json({ notices: [] })),
  http.get("/api/computers", () => HttpResponse.json(DEFAULT_COMPUTERS_LIST)),
  http.get("/api/computers/available", () => HttpResponse.json(DEFAULT_AVAILABLE_COMPUTERS)),
  http.get("/api/computers/ssh-hosts", () => HttpResponse.json(DEFAULT_SSH_HOSTS)),
  http.get(
    "/api/computers/stream",
    () =>
      new HttpResponse("", {
        status: 200,
        headers: { "Content-Type": "text/event-stream" },
      })
  ),
  http.patch("/api/computers/local", async ({ request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    return HttpResponse.json({ ...DEFAULT_COMPUTERS_LIST.local, ...body });
  }),
  http.post("/api/computers/placement", () =>
    HttpResponse.json({
      computer_id: DEFAULT_COMPUTERS_LIST.computers[0]?.id ?? "local",
      name: DEFAULT_COMPUTERS_LIST.computers[0]?.name ?? "This computer",
      workspace_id: "ws_devbox_api",
    })
  ),
  http.get("/api/host/resources", () =>
    HttpResponse.json({
      cpu_count: 8,
      cpu_util: 0.22,
      mem_total: 32 * 1024 * 1024 * 1024,
      mem_available: 18 * 1024 * 1024 * 1024,
      sampled_at: new Date().toISOString(),
    })
  ),
  http.get("/c/:computerId/api/host/resources", () =>
    HttpResponse.json({
      cpu_count: 16,
      cpu_util: 0.18,
      mem_total: 64 * 1024 * 1024 * 1024,
      mem_available: 42 * 1024 * 1024 * 1024,
      sampled_at: new Date().toISOString(),
    })
  ),
  http.get("/api/computers/:id", ({ params }) => {
    const found = DEFAULT_COMPUTERS_LIST.computers.find(c => c.id === params.id);
    if (!found) {
      return HttpResponse.json({ error: "Not found" }, { status: 404 });
    }
    return HttpResponse.json(found);
  }),
  http.patch("/api/computers/:id", async ({ params, request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    const found = DEFAULT_COMPUTERS_LIST.computers.find(c => c.id === params.id);
    if (!found) {
      return HttpResponse.json({ error: "Not found" }, { status: 404 });
    }
    return HttpResponse.json({ ...found, ...body });
  }),
  http.delete("/api/computers/:id", () =>
    HttpResponse.json({ removed: true, uninstall: "skipped" })
  ),
  http.delete("/api/computers/:id/routes/:routeId", () => new HttpResponse(null, { status: 204 })),
  http.get("/api/remote", () => HttpResponse.json(DEFAULT_REMOTE_STATUS)),
  http.patch("/api/remote", async ({ request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    return HttpResponse.json({ ...DEFAULT_REMOTE_STATUS, ...body });
  }),
  http.get("/api/remote/devices", () => HttpResponse.json(DEFAULT_REMOTE_DEVICES)),
  http.patch("/api/remote/devices/:id", async ({ params, request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    const found = DEFAULT_REMOTE_DEVICES.devices.find(d => d.id === params.id);
    return HttpResponse.json({ ...(found ?? DEFAULT_REMOTE_DEVICES.devices[0]), ...body });
  }),
  http.delete("/api/remote/devices/:id", () => new HttpResponse(null, { status: 204 })),
  http.delete("/api/remote/link", () => new HttpResponse(null, { status: 204 })),
];
