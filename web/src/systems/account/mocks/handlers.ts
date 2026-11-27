import { http, HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import type {
  AccountApprovalsResponse,
  AccountDevicesResponse,
  AccountStatusPayload,
} from "../adapters/account-api";

export const accountStatusStoryFixture: AccountStatusPayload = {
  gate: "off",
  grace: null,
  state: "signed_in",
  source: "session",
  signed_out_reason: null,
  pre_account_install: false,
  installation_linked: true,
  token_name: null,
  signed_in: true,
  email: "pedro@example.com",
  name: "Pedro",
  provider: "google",
  device: {
    id: "dev_2m8x0c",
    name: "MacBook Pro",
    admin: true,
  },
  marketing_opt_in: false,
  plan: "pro",
  subscription: {
    source: "stripe",
    interval: "year",
    current_period_end: "2027-03-01T00:00:00Z",
    cancel_at_period_end: false,
    refund_eligible_until: null,
  },
  remote_computers_used: 2,
  remote_computers_limit: null,
  cloud_routes_used: 1,
  cloud_routes_limit: 10,
  payment_issue: false,
  recent_sign_ins: [],
};

export const accountDevicesStoryFixture: AccountDevicesResponse = {
  devices: [
    {
      id: "dev_2m8x0c",
      name: "MacBook Pro",
      kind: "desktop",
      admin: true,
      last_seen_at: "2026-11-24T08:00:00Z",
      this_device: true,
    },
    {
      id: "dev_5k1p9z",
      name: "iPhone",
      kind: "browser",
      admin: false,
      last_seen_at: "2026-11-24T06:00:00Z",
      this_device: false,
    },
  ],
};

export const accountApprovalsStoryFixture: AccountApprovalsResponse = {
  approvals: [
    {
      id: "apr_01j9x",
      computer_id: "cmp_7f3k2q9x",
      device_name: "iPad Pro",
      key_fingerprint: "SHA256:9xK2mN4pR7wL1cF8hJ6tB5yD0sA3eG2qU",
      created_at: "2026-11-24T07:58:00Z",
      state: "pending",
      applied_result: null,
      can_decide: true,
    },
  ],
};

export const handlers = [
  compozyApiMock.get("/api/account", () => HttpResponse.json(accountStatusStoryFixture)),
  http.get(
    "/api/account/stream",
    () =>
      new HttpResponse("", {
        status: 200,
        headers: { "Content-Type": "text/event-stream" },
      })
  ),
  compozyApiMock.get("/api/account/devices", () => HttpResponse.json(accountDevicesStoryFixture)),
  compozyApiMock.get("/api/account/approvals", () =>
    HttpResponse.json(accountApprovalsStoryFixture)
  ),
  compozyApiMock.patch("/api/account/devices/{id}", async ({ params, request }) => {
    const patch = (await request.json()) as { admin?: boolean | null; name?: string | null };
    const existing =
      accountDevicesStoryFixture.devices.find(device => device.id === params.id) ??
      accountDevicesStoryFixture.devices[0]!;
    return HttpResponse.json({
      ...existing,
      admin: patch.admin ?? existing.admin,
      name: patch.name ?? existing.name,
    });
  }),
  compozyApiMock.delete("/api/account/devices/{id}", () => new HttpResponse(null, { status: 204 })),
  compozyApiMock.post("/api/account/approvals/{id}/decision", ({ params }) =>
    HttpResponse.json({ approval_id: params.id, state: "delivering" }, { status: 202 })
  ),
  compozyApiMock.post("/api/account/checkout", () =>
    HttpResponse.json({ url: "https://checkout.stripe.com/c/pay/cs_story_fixture" })
  ),
  compozyApiMock.post("/api/account/billing-portal", () =>
    HttpResponse.json({ url: "https://billing.stripe.com/p/session/bps_story_fixture" })
  ),
  compozyApiMock.post("/api/account/logout", () => HttpResponse.json({ state: "signed_out" })),
  compozyApiMock.patch("/api/account", async ({ request }) => {
    const patch = (await request.json()) as { marketing_opt_in?: boolean | null };
    return HttpResponse.json({
      ...accountStatusStoryFixture,
      marketing_opt_in: Boolean(patch.marketing_opt_in),
    });
  }),
  compozyApiMock.delete("/api/account", () =>
    HttpResponse.json(
      { state: "deleting", subscription_cancelled: false, refund: "none" },
      { status: 202 }
    )
  ),
  compozyApiMock.post(
    "/api/account/sign-in-notices/dismiss",
    () => new HttpResponse(null, { status: 204 })
  ),
  compozyApiMock.get("/api/account/tokens", () => HttpResponse.json({ tokens: [] })),
  compozyApiMock.post("/api/account/tokens", async ({ request }) => {
    const body = (await request.json()) as { name: string };
    return HttpResponse.json(
      { id: "atk_story", name: body.name, token: "cpzat_story_token_secret" },
      { status: 201 }
    );
  }),
  compozyApiMock.delete("/api/account/tokens/{id}", () => new HttpResponse(null, { status: 204 })),
];
