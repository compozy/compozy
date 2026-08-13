// Spec: gateway operator surface (E2E-001, E2E-005, E2E-006)
//
// The browser lane launches a real daemon on its **local** listener. Everything the local listener
// serves is exercised here for real, against real daemon responses — exposure ladder, the refusal a
// fail-closed transition produces, the per-enable public consent step, pairing mint and redeem,
// device rename and revoke, the delivery-URL surface, and the self-audit.
//
// BLOCKER — the legs marked `test.fixme` below need a *bound tier listener*, which this lane cannot
// produce:
//   • `gateway.Reconciler.enableTier` refuses a tier with no enabled connectivity provider
//     (`internal/gateway/reconciler.go`), and `validateTransitionTarget` refuses a surface enable
//     before that (`internal/gateway/policy_transition.go`).
//   • A provider endpoint is advertised only after `EndpointVerifier.Verify`
//     (`internal/gateway/endpoint_verify.go`), which hard-requires `https` and validates TLS against
//     the system pool; `EndpointVerifier.rootCAs` is package-private and settable only from Go unit
//     tests, so no fixture CA can be trusted here.
//   • `registerLocalRoutes` (`internal/api/httpapi/routes.go`) deliberately omits
//     `/api/gateway/stream-tickets`, and device authentication exists only on tier listeners
//     (`internal/api/httpapi/surface_routes.go`), so the local lane never returns the 401 that
//     produces the pairing gate or the access-ended state.
// Clearing this needs a connectivity-provider fixture plus a trusted TLS path in the browser lane.
// Tasks 08/09 own execution; nothing below is simulated or intercepted.
import type { Page } from "@playwright/test";

import type { BrowserRuntime, RuntimePaths } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureGlobalWorkspace, completeOnboardingIfPrompted } from "../fixtures/workspace";

interface GatewayStatusPayload {
  addresses: { address: string; live: boolean; tier: string }[];
  devices: { id: string; name: string; revoked_at?: string | null }[];
  enabled: boolean;
  refusal?: { cause: string; fix: string } | null;
  surfaces: {
    desired: string;
    generation: number;
    observed: string;
    surface: string;
    tier: string;
  }[];
  tiers: { advertised: boolean; desired: string; observed: string; tier: string }[];
}

interface GatewayAuditPayload {
  findings: { id: string; remediation: string; severity: string; summary: string }[];
  local_only: boolean;
  no_findings: boolean;
  status: GatewayStatusPayload;
}

// Covers the E2E-001 legs the local listener serves. The pairing artifact is redeemed through the
// daemon's own redeem endpoint, not from a second browser context — that leg is the fixme below.
test("E2E-001 (local legs) operator mints a pairing, the daemon redeems it once, and the inventory shows the device", async ({
  appPage,
  runtime,
}) => {
  assertLaunchRuntime(runtime, "gateway pairing");
  await ensureGlobalWorkspace(runtime);
  await completeOnboardingIfPrompted(appPage);

  await openGatewaySettings(appPage, runtime);

  // A fresh install is local-only, and the surface says so rather than implying reach.
  await expect(appPage.getByTestId("gateway-local-only")).toBeVisible();
  const status = await runtime.requestJSON<GatewayStatusPayload>("/api/gateway/status");
  expect(status.addresses).toEqual([]);
  expect(status.tiers.every(tier => !tier.advertised)).toBe(true);

  // Named exposure modes with their risk framing — never a bare toggle.
  await expect(appPage.getByTestId("gateway-exposure-private-operator-ui")).toContainText(
    "Private overlay"
  );
  await expect(appPage.getByTestId("gateway-exposure-public-webhook-ingress")).toContainText(
    "signed webhook and bridge deliveries only"
  );
  await expect(appPage.getByTestId("gateway-exposure-public-operator-ui")).toContainText(
    "pairing is never offered there"
  );

  await expect(appPage.getByText("No paired devices yet")).toBeVisible();

  // Mint through the UI; the artifact is shown once, and it is obtainable without a camera.
  await appPage.getByTestId("gateway-pair-device").click();
  await expect(appPage.getByTestId("gateway-pairing-dialog")).toBeVisible();
  const artifact = (await appPage.getByTestId("gateway-pairing-code").innerText()).trim();
  expect(artifact).not.toBe("");
  await expect(
    appPage
      .getByTestId("gateway-pairing-dialog")
      .getByRole("button", { name: /copy pairing code/i })
  ).toBeVisible();

  // The one-time secret never reaches a server-visible surface.
  expect(new URL(appPage.url()).search).not.toContain(artifact);
  expect(new URL(appPage.url()).pathname).not.toContain(artifact);

  // A second device redeems the same artifact against the daemon.
  const redeemed = await runtime.requestJSON<{ device: { id: string; name: string } }>(
    "/api/gateway/pairings/redeem",
    {
      method: "POST",
      body: JSON.stringify({
        actor_kind: "operator_device",
        artifact,
        name: "Second browser",
      }),
    }
  );
  expect(redeemed.device.name).toBe("Second browser");

  // A single-use artifact cannot be redeemed twice.
  await expect(
    runtime.requestJSON("/api/gateway/pairings/redeem", {
      method: "POST",
      body: JSON.stringify({
        actor_kind: "operator_device",
        artifact,
        name: "Third browser",
      }),
    })
  ).rejects.toThrow();

  await appPage.keyboard.press("Escape");
  await expect(appPage.getByTestId("gateway-pairing-dialog")).toBeHidden();
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(appPage.getByTestId(`gateway-device-${redeemed.device.id}`)).toContainText(
    "Second browser"
  );
});

test.fixme("E2E-001 (tier-listener leg) paired device loads the full UI over the private tier while an unpaired context sees only the gate", async () => {
  // Needs a bound private-tier listener: an enabled connectivity provider whose HTTPS endpoint
  // passes challenge verification, so that a second browser context reaches a listener which
  // answers 401 `gateway_device_unauthenticated` before pairing and serves the full UI with a
  // ticketed live stream afterwards. See the BLOCKER note at the top of this file.
});

// Covers the E2E-005 legs the local listener serves. Reaching the consented public UI from a
// paired device and watching it land on access-ended is the fixme below.
test("E2E-005 (local legs) public operator access requires fresh consent, revocation is immediate, and the surface stays local-only", async ({
  appPage,
  runtime,
}) => {
  assertLaunchRuntime(runtime, "gateway consent and revocation");
  await ensureGlobalWorkspace(runtime);
  await completeOnboardingIfPrompted(appPage);

  await openGatewaySettings(appPage, runtime);

  // Turning public operator access on always passes through the consent step.
  await appPage.getByTestId("gateway-exposure-public-operator-ui-switch").click();
  const consent = appPage.getByTestId("gateway-public-consent-dialog");
  await expect(consent).toBeVisible();
  await expect(consent).toContainText("operator UI and the whole management API");
  await expect(consent).toContainText("Pairing codes are never minted or redeemed");
  await expect(appPage.getByTestId("gateway-public-consent-confirm")).toBeDisabled();

  await appPage.getByTestId("gateway-public-consent-acknowledge").click();
  await appPage.getByTestId("gateway-public-consent-confirm").click();

  // The daemon fails closed and the surface reports its refusal instead of a success it did not get.
  await expect(appPage.getByTestId("gateway-exposure-error")).toBeVisible();
  const refused = await runtime.requestJSON<GatewayStatusPayload>("/api/gateway/status");
  expect(
    refused.surfaces.some(
      surface =>
        surface.surface === "operator_ui" &&
        surface.tier === "public" &&
        surface.desired === "enabled"
    )
  ).toBe(false);

  // Consent is asked for again on the next attempt — the acknowledgement is never remembered.
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await appPage.getByTestId("gateway-exposure-public-operator-ui-switch").click();
  await expect(appPage.getByTestId("gateway-public-consent-confirm")).toBeDisabled();
  await appPage.getByRole("button", { name: "Cancel" }).click();

  // Revocation is immediate and terminal, and the row stays for audit with no actions.
  const paired = await runtime.requestJSON<{ artifact: string }>("/api/gateway/pairings", {
    method: "POST",
  });
  const device = await runtime.requestJSON<{ device: { id: string } }>(
    "/api/gateway/pairings/redeem",
    {
      method: "POST",
      body: JSON.stringify({
        actor_kind: "operator_device",
        artifact: paired.artifact,
        name: "Revoked phone",
      }),
    }
  );

  await appPage.reload({ waitUntil: "domcontentloaded" });
  const row = appPage.getByTestId(`gateway-device-${device.device.id}`);
  await expect(row).toContainText("Revoked phone");
  await appPage.getByTestId(`gateway-device-${device.device.id}-revoke`).click();
  const revokeDialog = appPage.getByTestId(`gateway-device-${device.device.id}-revoke-dialog`);
  await expect(revokeDialog).toBeVisible();
  await revokeDialog.getByRole("button", { name: "Revoke access" }).click();

  await expect(row).toContainText("Revoked");
  await expect(appPage.getByTestId(`gateway-device-${device.device.id}-revoke`)).toHaveCount(0);

  const afterRevoke = await runtime.requestJSON<GatewayStatusPayload>("/api/gateway/status");
  expect(afterRevoke.devices.find(entry => entry.id === device.device.id)?.revoked_at).toBeTruthy();

  // Nothing was ever exposed, and the surface still says local-only.
  await expect(appPage.getByTestId("gateway-local-only")).toBeVisible();
  expect(afterRevoke.addresses).toEqual([]);
});

test.fixme("E2E-005 (tier-listener leg) a revoked device's open view lands on access ended", async () => {
  // Needs a bound public-tier listener with the consented operator UI, so a paired second context
  // is holding a live session when it is revoked and its next request returns 401
  // `gateway_device_revoked`. See the BLOCKER note at the top of this file.
});

// Covers the E2E-006 legs the local listener serves. The finding is cleared by resolving the
// refused intent through the API, not by running the remediation the finding prints — that
// remediation enables a connectivity provider, which is the fixme below.
test("E2E-006 (local legs) the self-audit reports a ranked finding with actionable remediation and reports it cleared once the refused intent is resolved", async ({
  appPage,
  runtime,
}) => {
  assertLaunchRuntime(runtime, "gateway self-audit");
  await ensureGlobalWorkspace(runtime);
  await completeOnboardingIfPrompted(appPage);

  await openGatewaySettings(appPage, runtime);

  // A fresh local-only install audits clean, and "no findings" is stated explicitly.
  await appPage.getByTestId("gateway-audit-run").click();
  await expect(appPage.getByText("No findings")).toBeVisible();
  const clean = await runtime.requestJSON<GatewayAuditPayload>("/api/gateway/audit");
  expect(clean.no_findings).toBe(true);
  expect(clean.local_only).toBe(true);

  // Drive the posture into a fail-closed refusal through the operator surface.
  await appPage.getByTestId("gateway-exposure-private-operator-ui-switch").click();
  await expect(appPage.getByTestId("gateway-exposure-error")).toBeVisible();

  // The audit now reports it, ranked first, with remediation that names the next action.
  await appPage.getByTestId("gateway-audit-run").click();
  const finding = appPage.getByTestId("gateway-audit-finding-gateway.exposure.refused");
  await expect(finding).toBeVisible();
  await expect(finding).toContainText("Critical");
  await expect(
    appPage.getByTestId("gateway-audit-remediation-gateway.exposure.refused")
  ).not.toBeEmpty();

  const flagged = await runtime.requestJSON<GatewayAuditPayload>("/api/gateway/audit");
  expect(flagged.no_findings).toBe(false);
  expect(flagged.findings[0].id).toBe("gateway.exposure.refused");
  expect(flagged.findings[0].severity).toBe("critical");
  expect(flagged.findings[0].remediation.trim()).not.toBe("");

  // Resolving the refused intent clears it, and the re-run says so.
  await runtime.requestJSON("/api/gateway/surfaces", {
    method: "POST",
    body: JSON.stringify({
      consent: false,
      desired: "disabled",
      expected_generation: surfaceGeneration(flagged, "operator_ui", "private"),
      surface: "operator_ui",
      tier: "private",
    }),
  });

  await appPage.getByTestId("gateway-audit-run").click();
  await expect(appPage.getByTestId("gateway-audit-clear")).toBeVisible();
  const cleared = await runtime.requestJSON<GatewayAuditPayload>("/api/gateway/audit");
  expect(cleared.no_findings).toBe(true);
});

test.fixme("E2E-006 (provider-backed leg) a provider finding is cleared by running its own printed remediation", async () => {
  // The findings whose remediation is a runnable command — `gateway.provider.unhealthy.*`,
  // `gateway.surface.drift.*`, `gateway.ingress.*` — only exist once a tier is bound and a
  // provider is enabled. See the BLOCKER note at the top of this file.
});

async function openGatewaySettings(page: Page, runtime: BrowserRuntime): Promise<void> {
  await page.goto(runtime.url("/settings/gateway"), { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("gateway-exposure-section")).toBeVisible();
}

/**
 * The optimistic-concurrency fence a transition must carry. A surface the daemon
 * never persisted has no row, and `0` is the generation the store expects for it.
 */
function surfaceGeneration(audit: GatewayAuditPayload, surface: string, tier: string): number {
  const match = audit.status.surfaces.find(
    entry => entry.surface === surface && entry.tier === tier
  );
  return match?.generation ?? 0;
}

function assertLaunchRuntime(
  runtime: BrowserRuntime,
  context: string
): asserts runtime is BrowserRuntime & { paths: RuntimePaths } {
  if (!runtime.paths) {
    throw new Error(`${context} checks require launch-mode runtime paths.`);
  }
}
