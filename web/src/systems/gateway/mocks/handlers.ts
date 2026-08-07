import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import { gatewayAuditFixture, gatewayDeviceFixture, gatewayStatusFixture } from "./fixtures";

const devices = [
  gatewayDeviceFixture(),
  gatewayDeviceFixture({
    actor_kind: "cli_profile",
    created_at: "2026-07-20T10:00:00Z",
    id: "dev_laptop",
    last_seen_at: "2026-08-06T18:30:00Z",
    name: "Work laptop",
    pairing_origin: "local",
  }),
];

/** Local-only posture with a small paired inventory — the shape a fresh install shows. */
export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/gateway/status", () =>
    HttpResponse.json(gatewayStatusFixture({ devices }))
  ),
  compozyApiMock.get("/api/gateway/devices", () => HttpResponse.json({ devices })),
  compozyApiMock.get("/api/gateway/audit", () =>
    HttpResponse.json(gatewayAuditFixture({ status: gatewayStatusFixture({ devices }) }))
  ),
  compozyApiMock.post("/api/gateway/pairings", () =>
    HttpResponse.json(
      { artifact: "cpz_gwp_2f8c1d5a9b4e7c0d", expires_at: "2099-01-01T00:00:00Z" },
      { status: 201 }
    )
  ),
  compozyApiMock.post("/api/gateway/surfaces", () =>
    HttpResponse.json(gatewayStatusFixture({ devices, changed: true }))
  ),
];
