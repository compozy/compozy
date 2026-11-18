// Suite: pairing redeem gate
// Invariant: one field redeems either a six-digit manual code (typed with or without grouping) or a
// pasted artifact/link, and the request carries exactly one of them. Daemon refusals become plain,
// non-technical copy, and an invalid code stays generic because unknown and wrong codes look the
// same on purpose.
// Owning layer: the unpaired-browser pairing gate component.
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import userEvent from "@testing-library/user-event";
import type { PropsWithChildren } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";

import { GatewayPairingGate } from "../gateway-pairing-gate";

const ARTIFACT = "cpz_gwp_2f8c1d5a9b4e7c0d";
const DEVICE = { id: "dev_new", name: "Work phone" };

function wrapper({ children }: PropsWithChildren) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

function renderGate() {
  const reload = vi.fn();
  render(<GatewayPairingGate reload={reload} tier="private" />, { wrapper });
  return reload;
}

async function redeemWith(code: string): Promise<void> {
  await userEvent.type(screen.getByTestId("gateway-pairing-gate-code"), code);
  await userEvent.type(screen.getByTestId("gateway-pairing-gate-name"), DEVICE.name);
  await userEvent.click(screen.getByTestId("gateway-pairing-gate-submit"));
}

function mockRefusal(status: number, body: Record<string, string>): void {
  vi.mocked(globalThis.fetch).mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    })
  );
}

describe("GatewayPairingGate", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
    window.history.replaceState(null, "", "/");
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it.each(["123456", "123 456", "123-456"])(
    "Should redeem the typed code %j as manual_code with digits only",
    async code => {
      mockJsonResponse({ device: DEVICE });
      const reload = renderGate();

      await redeemWith(code);

      await expectFetchRequest({
        body: { actor_kind: "operator_device", manual_code: "123456", name: DEVICE.name },
        method: "POST",
        path: "/api/gateway/pairings/redeem",
      });
      await waitFor(() => expect(reload).toHaveBeenCalledTimes(1));
    }
  );

  it("Should send a pasted artifact as artifact and never attach a manual code", async () => {
    mockJsonResponse({ device: DEVICE });
    const reload = renderGate();

    await redeemWith(ARTIFACT);

    await expectFetchRequest({
      body: { actor_kind: "operator_device", artifact: ARTIFACT, name: DEVICE.name },
      method: "POST",
      path: "/api/gateway/pairings/redeem",
    });
    await waitFor(() => expect(reload).toHaveBeenCalledTimes(1));
  });

  it.each([
    {
      status: 401,
      code: "gateway_pairing_invalid",
      message: /that code isn't valid/i,
    },
    {
      status: 409,
      code: "gateway_pairing_spent",
      message: /already used/i,
    },
    {
      status: 410,
      code: "gateway_pairing_expired",
      message: /has expired/i,
    },
    {
      status: 429,
      code: "gateway_auth_rate_limited",
      message: /wait a few minutes before trying again/i,
    },
  ])(
    "Should explain a $status refusal in plain words and not echo the daemon code",
    async ({ status, code, message }) => {
      mockRefusal(status, { error: "internal refusal detail", code });
      renderGate();

      await redeemWith("123456");

      const alert = await screen.findByTestId("gateway-pairing-gate-error");
      expect(alert).toHaveTextContent(message);
      expect(alert).not.toHaveTextContent(code);
      expect(alert).not.toHaveTextContent("internal refusal detail");
    }
  );

  it("Should keep an invalid code generic, never saying whether it was unknown or wrong", async () => {
    mockRefusal(401, { error: "pairing rejected", code: "gateway_pairing_invalid" });
    renderGate();

    await redeemWith("654321");

    const alert = await screen.findByTestId("gateway-pairing-gate-error");
    expect(alert.textContent ?? "").not.toMatch(/unknown|wrong|not found|mismatch/i);
  });

  it("Should fall back to a generic retry message for an unexpected failure", async () => {
    mockRefusal(500, { error: "database locked" });
    renderGate();

    await redeemWith("123456");

    const alert = await screen.findByTestId("gateway-pairing-gate-error");
    expect(alert).toHaveTextContent("Couldn't pair this device. Try again in a moment.");
    expect(alert).not.toHaveTextContent("database locked");
  });
});
