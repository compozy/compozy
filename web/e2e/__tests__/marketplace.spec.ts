import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Locator, Page } from "@playwright/test";

import { captureRouteState } from "../fixtures/browser-artifact-session";
import { closeMCPAuthServer, startMCPAuthServer } from "../fixtures/mcp-auth-server";
import { appWindow, sessionWindow, switchWorkspace } from "../fixtures/os-navigation";
import {
  marketplaceOperatorSelectors,
  sessionLifecycleSelectors,
  sessionWindowSelectors,
} from "../fixtures/selectors";
import {
  type BrowserRuntime,
  type BrowserSettingsFixturesResult,
  type RuntimePaths,
  type WorkspacePayload,
  cleanupBrowserSettingsFixtures,
  seedBrowserSettingsFixtures,
} from "../fixtures/runtime";
import {
  assertNoSensitiveArtifactPayload,
  captureBrowserTransportSnapshot,
  captureViewportEvidence,
  requestBrowserRuntimeOperatorJSON,
  runBrowserRuntimeCLIJSON,
  sensitiveArtifactPattern,
} from "../fixtures/scenario-contracts";
import { createPluginMarketplaceFixture } from "../fixtures/marketplace-server";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace, completeOnboardingIfPrompted } from "../fixtures/workspace";

// Invariant: Browse preserves independent source identity, complete counts and query recovery.
// Owner: Marketplace window against a real daemon; canonical Marketplace E2E suite.
test.describe("Marketplace source catalog", () => {
  test.use({
    runtimeOptions: {
      extensionsAllowUnverified: true,
      seed: {
        marketplaceCatalog: {
          extensions: [
            {
              entry_id: "reference",
              name: "Reference extension",
              description: "Curated reference entry",
              version: "1.0.0",
              install_slug: "compozy/reference",
              artifact_url: "https://example.test/reference.tar.gz",
              digest_sha256: "a".repeat(64),
              tier: "unverified",
            },
          ],
        },
      },
    },
  });

  test("E2E-001: operator browses three source sections and resolves duplicate entry IDs by source", async ({
    appPage,
    runtime,
  }) => {
    const team = await createPluginMarketplaceFixture();
    const partner = await createPluginMarketplaceFixture();
    try {
      for (const [name, source] of [
        ["team", team.source],
        ["partner", partner.source],
      ]) {
        await runtime.requestJSON("/api/marketplace/sources", {
          method: "POST",
          body: JSON.stringify({ name, ref: source }),
        });
      }
      await runtime.requestJSON("/api/marketplace/refresh", { method: "POST" });
      await ensureProjectWorkspace(appPage, runtime);
      await completeOnboardingIfPrompted(appPage);
      await appPage.goto(runtime.url("/marketplace"), { waitUntil: "domcontentloaded" });
      const win = appWindow(appPage, "marketplace");
      for (const name of ["compozy-catalog", "team", "partner"]) {
        await expect(win.getByTestId(`marketplace-section-${name}`)).toBeVisible();
      }
      const teamCard = win
        .getByTestId("marketplace-section-team")
        .getByTestId("marketplace-card-tool");
      const partnerCard = win
        .getByTestId("marketplace-section-partner")
        .getByTestId("marketplace-card-tool");
      await expect(teamCard).toBeVisible();
      await expect(partnerCard).toBeVisible();
      await teamCard.getByRole("link", { name: /View .* details/ }).click();
      await expect.poll(() => new URL(appPage.url()).searchParams.get("source")).toBe("team");
      await expect(win.getByTestId("marketplace-detail")).toContainText("Loop");
      await appPage.goto(runtime.url("/marketplace?q=does-not-match-any-plugin"), {
        waitUntil: "domcontentloaded",
      });
      await expect(win.getByTestId("marketplace-query-empty")).toBeVisible();
      await win.getByRole("button", { name: "Clear search", exact: true }).click();
      await expect(win.getByTestId("marketplace-section-team")).toBeVisible();
    } finally {
      await team.cleanup();
      await partner.cleanup();
    }
  });
  // Invariant: a checked source can be added, but changed package bytes require fresh consent.
  test("E2E-004: Add marketplace installs client-layout plugins only after approving current bytes", async ({
    appPage,
    runtime,
  }) => {
    const fixture = await createPluginMarketplaceFixture();
    try {
      if (!runtime.paths) throw new Error("Plugin acquisition requires an isolated runtime");
      const workspace = await runtime.resolveWorkspace(runtime.paths.workspaceDir);
      const scopeQuery = `workspace=${encodeURIComponent(workspace.id)}&profile=default`;
      await ensureProjectWorkspace(appPage, runtime);
      await completeOnboardingIfPrompted(appPage);
      await appPage.goto(runtime.url("/marketplace"));
      const win = appWindow(appPage, "marketplace");
      await win.getByTestId("marketplace-add").click();
      await appPage.getByTestId("marketplace-add-marketplace").click();
      await appPage.getByTestId("add-marketplace-ref").fill(fixture.source);
      await appPage.getByTestId("add-marketplace-ref").blur();
      await expect(appPage.getByTestId("add-marketplace-found")).toContainText("1");
      await appPage.getByTestId("add-marketplace-submit").click();
      await expect(appPage.getByTestId("add-marketplace-dialog")).not.toBeVisible();
      const card = win
        .getByTestId("marketplace-section-source")
        .getByTestId("marketplace-card-tool");
      await card.getByTestId("marketplace-action-tool").click();
      await appPage.getByTestId("extension-trust-confirm").click();
      await expect(appPage.getByTestId("extension-install-summary-dialog")).toBeVisible();
      await fixture.changePackage();
      // A cached approved package is immutable and remains installable. Refresh publishes the
      // changed package before submitting the old approval, exercising the listing digest fence.
      await runtime.requestJSON("/api/marketplace/refresh", { method: "POST" });
      const rejected = appPage.waitForResponse(
        response =>
          response.request().method() === "POST" &&
          new URL(response.url()).pathname === "/api/extensions"
      );
      await appPage.getByTestId("extension-install-summary-confirm").click();
      const conflict = await rejected;
      expect(conflict.status()).toBe(409);
      expect(await conflict.json()).toMatchObject({ code: "extension_source_changed" });
      const before = await runtime.requestJSON<{ extensions: Array<{ name: string }> }>(
        `/api/extensions?${scopeQuery}`
      );
      expect(before.extensions.some(item => item.name === fixture.instanceName)).toBe(false);
      await expect(appPage.getByTestId("extension-trust-dialog")).toBeVisible();
      await appPage.getByTestId("extension-trust-confirm").click();
      const installed = appPage.waitForResponse(
        response =>
          response.request().method() === "POST" &&
          new URL(response.url()).pathname === "/api/extensions"
      );
      await appPage.getByTestId("extension-install-summary-confirm").click();
      expect((await installed).status()).toBe(201);
      const detail = await runtime.requestJSON<{ extension: { name: string; format: string } }>(
        `/api/extensions/${fixture.instanceName}?${scopeQuery}`
      );
      expect(detail.extension).toMatchObject({
        name: fixture.instanceName,
        format: "agent-plugin",
      });
      await appPage.goto(runtime.url("/marketplace/installed"));
      await expect(
        win.getByTestId(`marketplace-installed-card-${fixture.instanceName}`)
      ).toBeVisible();
    } finally {
      await fixture.cleanup();
    }
  });

  // Invariant: source availability/removal changes discovery while preserving installed packages.
  test("E2E-005: source settings retain cached diagnostics and installed packages across remove/re-add", async ({
    appPage,
    runtime,
  }) => {
    const fixture = await createPluginMarketplaceFixture();
    try {
      await runtime.requestJSON("/api/marketplace/sources", {
        method: "POST",
        body: JSON.stringify({ name: "team", ref: fixture.source }),
      });
      const listing = await runtime.requestJSON<{
        items: Array<{ entry_id: string; source: string; digest_sha256: string }>;
      }>("/api/marketplace");
      const entry = listing.items.find(item => item.source === "team" && item.entry_id === "tool");
      if (!entry) throw new Error("Registered fixture plugin is missing from the catalog");
      await runtime.requestJSON("/api/extensions", {
        method: "POST",
        body: JSON.stringify({
          source: "marketplace",
          ref: "team/tool",
          expected_digest: entry.digest_sha256,
          allow_unverified: true,
        }),
      });
      await ensureProjectWorkspace(appPage, runtime);
      await completeOnboardingIfPrompted(appPage);
      await appPage.goto(runtime.url("/settings/marketplace"));
      const settings = appWindow(appPage, "settings");
      const row = settings.getByTestId("settings-page-marketplace-source-team");
      await expect(row).toBeVisible();
      await row.getByTestId("settings-page-marketplace-source-team-disclosure").click();
      const documentPath = path.join(fixture.source, "marketplace.json");
      const document = await readFile(documentPath, "utf8");
      await writeFile(documentPath, "invalid document");
      await row.getByTestId("settings-page-marketplace-source-team-refresh").click();
      await expect(row.getByTestId("settings-page-marketplace-source-team-degraded")).toBeVisible();
      await expect(row.getByTestId("settings-page-marketplace-source-team-reason")).not.toBeEmpty();
      await expect(row.getByTestId("settings-page-marketplace-source-team-count")).toContainText(
        "1"
      );
      await writeFile(documentPath, document);
      await row.getByTestId("settings-page-marketplace-source-team-refresh").click();
      await expect(
        row.getByTestId("settings-page-marketplace-source-team-degraded")
      ).not.toBeVisible();
      await row.getByTestId("settings-page-marketplace-source-team-toggle").click();
      await expect(row.getByTestId("settings-page-marketplace-source-team-off")).toBeVisible();
      const disabled = await runtime.requestJSON<{ items: Array<{ source: string }> }>(
        "/api/marketplace"
      );
      expect(disabled.items.some(item => item.source === "team")).toBe(false);
      await row.getByTestId("settings-page-marketplace-source-team-remove").click();
      await appPage.getByTestId("settings-page-marketplace-sources-remove-confirm").click();
      await expect(row).not.toBeVisible();
      const installed = await runtime.requestJSON<{ extension: { name: string } }>(
        `/api/extensions/${fixture.instanceName}`
      );
      expect(installed.extension.name).toBe(fixture.instanceName);
      await runtime.requestJSON("/api/marketplace/sources", {
        method: "POST",
        body: JSON.stringify({ name: "team", ref: fixture.source }),
      });
      const restored = await runtime.requestJSON<{
        items: Array<{ source: string; entry_id: string; installed: boolean }>;
      }>("/api/marketplace");
      expect(
        restored.items.find(item => item.source === "team" && item.entry_id === "tool")?.installed
      ).toBe(true);
      await appPage.reload();
      await expect(row).toBeVisible();
    } finally {
      await fixture.cleanup();
    }
  });
});

test.describe("MCP Settings authorization", () => {
  const SERVER_NAME = "linear";

  test("operator authorizes an OAuth MCP server end to end against the fake AS and lands preselected", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    const sessionUI = sessionLifecycleSelectors(appPage);
    const authServer = await startMCPAuthServer();
    const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-mcp-authorize-"));
    const workspace = await runtime.resolveWorkspace(workspaceRoot);

    let seeded: BrowserSettingsFixturesResult | undefined;
    try {
      seeded = await seedBrowserSettingsFixtures(runtime, {
        mcpServers: [
          {
            name: SERVER_NAME,
            scope: "workspace",
            workspaceId: workspace.id,
            workspaceRootDir: workspaceRoot,
            target: "config",
            server: {
              name: SERVER_NAME,
              transport: "http",
              url: `${authServer.baseURL}/mcp`,
              auth: {
                client_id: "compozy-e2e-public",
                issuer_url: authServer.baseURL,
                registration: "pre_registered",
                scopes: ["read", "write"],
              },
            },
          },
        ],
      });

      await ensureProjectWorkspace(appPage, runtime);
      await completeOnboardingIfPrompted(sessionUI);

      await appPage.goto(runtime.url("/settings/mcp"), {
        waitUntil: "domcontentloaded",
      });
      await switchWorkspace(appPage, workspace.id, workspace.name);

      await appPage.goto(runtime.url("/settings/mcp"), {
        waitUntil: "domcontentloaded",
      });

      const serverRow = appWindow(appPage, "settings").getByTestId(
        `settings-page-mcp-servers-row-${SERVER_NAME}`
      );
      await expect(serverRow).toBeVisible();
      await serverRow
        .getByRole("button", { name: `Authorize ${SERVER_NAME}`, exact: true })
        .click();
      const urlBlock = appPage.getByTestId("settings-page-mcp-authorize-url");
      await expect(urlBlock).toBeVisible();
      await appPage.getByTestId("settings-page-mcp-authorize-manual-trigger").click();
      await expect(appPage.getByTestId("settings-page-mcp-authorize-manual-input")).toBeVisible();
      const manualLiveUrl = (await urlBlock.locator("code").innerText()).trim();
      const manualAuthorizationURL = new URL(manualLiveUrl);
      const manualState = manualAuthorizationURL.searchParams.get("state");
      const manualRedirectURI = manualAuthorizationURL.searchParams.get("redirect_uri");
      expect(manualState, "manual begin must return a live PKCE state").toBeTruthy();
      expect(manualRedirectURI, "manual begin must return the pending redirect URI").toBeTruthy();
      const callbackURL = new URL(manualRedirectURI ?? "");
      callbackURL.searchParams.set("code", "auth-code");
      callbackURL.searchParams.set("state", manualState ?? "");
      await appPage
        .getByTestId("settings-page-mcp-authorize-manual-input")
        .fill(callbackURL.toString());
      await appPage.getByTestId("settings-page-mcp-authorize-exchange").click();

      await expect(appPage.getByTestId("settings-page-mcp-authorize-confirmed")).toBeVisible({
        timeout: 15_000,
      });
      await appPage.getByTestId("settings-page-mcp-authorize-done").click();

      await expect(
        serverRow.getByTestId(`settings-page-mcp-servers-row-${SERVER_NAME}-auth`)
      ).toContainText("Authenticated", { timeout: 15_000 });

      await browserArtifacts.captureScreenshot("mcp-authorize-confirmed", appPage);
    } finally {
      if (seeded) await cleanupBrowserSettingsFixtures(runtime, seeded);
      await closeMCPAuthServer(authServer.server);
    }
  });

  test("operator completes the browser authorize path and the scoped list polls to confirmation", async ({
    appPage,
    runtime,
  }) => {
    const sessionUI = sessionLifecycleSelectors(appPage);
    const authServer = await startMCPAuthServer();
    const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-mcp-browser-authorize-"));
    const workspace = await runtime.resolveWorkspace(workspaceRoot);

    let popup: typeof appPage | undefined;
    let seeded: BrowserSettingsFixturesResult | undefined;
    try {
      seeded = await seedBrowserSettingsFixtures(runtime, {
        mcpServers: [
          {
            name: SERVER_NAME,
            scope: "workspace",
            workspaceId: workspace.id,
            workspaceRootDir: workspaceRoot,
            target: "config",
            server: {
              name: SERVER_NAME,
              transport: "http",
              url: `${authServer.baseURL}/mcp`,
              auth: {
                client_id: "compozy-e2e-public",
                issuer_url: authServer.baseURL,
                registration: "pre_registered",
                scopes: ["read", "write"],
              },
            },
          },
        ],
      });

      await ensureProjectWorkspace(appPage, runtime);
      await completeOnboardingIfPrompted(sessionUI);
      await appPage.goto(runtime.url("/settings/mcp"), {
        waitUntil: "domcontentloaded",
      });
      await switchWorkspace(appPage, workspace.id, workspace.name);
      await appPage.goto(runtime.url("/settings/mcp"), {
        waitUntil: "domcontentloaded",
      });
      const serverRow = appWindow(appPage, "settings").getByTestId(
        `settings-page-mcp-servers-row-${SERVER_NAME}`
      );
      await expect(serverRow).toBeVisible();
      await serverRow
        .getByRole("button", { name: `Authorize ${SERVER_NAME}`, exact: true })
        .click();

      const popupPromise = appPage.context().waitForEvent("page");
      await appPage.getByTestId("settings-page-mcp-authorize-open-url").click();
      popup = await popupPromise;

      await expect.poll(() => authServer.requests, { timeout: 10_000 }).toContain("GET /authorize");
      await expect.poll(() => authServer.requests, { timeout: 10_000 }).toContain("POST /token");
      await expect(appPage.getByTestId("settings-page-mcp-authorize-confirmed")).toBeVisible({
        timeout: 15_000,
      });
    } finally {
      if (popup && !popup.isClosed()) await popup.close();
      if (seeded) await cleanupBrowserSettingsFixtures(runtime, seeded);
      await closeMCPAuthServer(authServer.server);
    }
  });
});

test.describe("Extension marketplace runtime", () => {
  const execFileAsync = promisify(execFile);

  const extensionName = "browser-tool-provider";
  const toolID = "ext__browser_tool_provider__search";
  const extensionSecretSentinel = "browser-extension-secret-value-13";
  const toolPermissionFixture = path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    "..",
    "..",
    "..",
    "internal",
    "testutil",
    "acpmock",
    "testdata",
    "tool_permission_fixture.json"
  );
  const denyPermissionFixture = path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    "..",
    "..",
    "..",
    "internal",
    "testutil",
    "acpmock",
    "testdata",
    "browser_session_hardening_fixture.json"
  );
  const toolPermissionAgent = "golden";
  const denyPermissionAgent = "permission-hardening-agent";

  interface ExtensionPayload {
    name: string;
    enabled: boolean;
    state: string;
    daemon_running: boolean;
    health?: string;
    capabilities?: string[];
  }

  interface ExtensionResponse {
    extension: ExtensionPayload;
  }

  interface ExtensionsResponse {
    extensions: ExtensionPayload[];
  }

  interface ToolPayload {
    descriptor: {
      tool_id: string;
      backend: { kind: string; extension_id?: string; handler?: string };
      source: { kind: string; owner: string; resource_id?: string };
      visibility?: string;
      read_only: boolean;
      risk: string;
    };
    availability: { available: boolean; executable: boolean; reason_codes?: string[] };
    decision: { approval_required: boolean; callable: boolean; visible_to_operator: boolean };
  }

  interface ToolsResponse {
    tools: ToolPayload[];
  }

  interface ToolInvokeResponse {
    tool_id: string;
    status: string;
    result: {
      content?: Array<{ type: string; text?: string }>;
      structured?: unknown;
      preview?: string;
      redactions?: Array<{ path: string; reason: string }>;
      truncated: boolean;
      bytes: number;
      duration_ms: number;
    };
    events: Array<{
      tool_id: string;
      source_kind?: string;
      source_owner?: string;
      redacted_input_fields?: string[];
      input_digest?: string;
      correlation_id?: string;
    }>;
  }

  interface ToolApprovalPayload {
    approval_token: string;
    input_digest: string;
    tool_id: string;
  }

  interface ToolApprovalResponse {
    approval: ToolApprovalPayload;
  }

  interface SettingsRestartAction {
    operation_id: string;
    status_url: string;
  }

  interface SettingsRestartStatus {
    status: string;
  }

  interface SessionEnvelope {
    session: {
      id: string;
      agent_name: string;
      state: string;
      workspace_id: string;
    };
  }

  interface SessionHistoryEnvelope {
    messages?: unknown[];
  }

  interface SessionEventEnvelope {
    events?: unknown[];
  }

  interface ExecFailure {
    stdout: string;
    stderr: string;
    code?: unknown;
  }

  test.use({
    runtimeOptions: {
      env: {
        ...process.env,
        COMPOZY_BROWSER_EXTENSION_SECRET: extensionSecretSentinel,
      },
      extensionsAllowUnverified: true,
      readyTimeoutMs: 45_000,
      seed: {
        mockAgents: [
          {
            fixturePath: toolPermissionFixture,
            fixtureAgent: toolPermissionAgent,
          },
          {
            fixturePath: denyPermissionFixture,
            fixtureAgent: denyPermissionAgent,
          },
        ],
      },
      toolsExternalDefault: "enabled",
    },
  });

  test("operator installs a local extension tool provider, invokes it over transports, and verifies fail-closed manifest security", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    assertLaunchRuntime(runtime, "extensibility tool/resource lifecycle");

    const extensionDir = await createBrowserToolProviderExtension();
    const checksumFailureDir = await createChecksumFailureExtension();
    const badManifestDir = await createInvalidExtensionManifest();
    const installedExtensionCard = appPage
      .getByTestId(/^marketplace-installed-card-/)
      .filter({ hasText: extensionName })
      .first();

    await completeOnboardingIfPrompted(appPage);

    const installed = await runBrowserRuntimeCLIJSON<ExtensionPayload>(runtime, [
      "extension",
      "install",
      "--allow-unverified",
      "--yes",
      extensionDir,
    ]);
    expect(projectExtension(installed)).toMatchObject({
      name: extensionName,
      enabled: true,
      state: "active",
    });

    await expect
      .poll(async () => {
        const payload = await runtime.requestJSON<ExtensionResponse>(
          `/api/extensions/${extensionName}`
        );
        return payload.extension.state;
      })
      .toBe("active");

    await appPage.goto(runtime.url("/marketplace/installed"), {
      waitUntil: "domcontentloaded",
    });
    const marketplaceWin = appWindow(appPage, "marketplace");
    await expect(marketplaceWin).toBeVisible();
    await expect(marketplaceWin.getByTestId("marketplace-installed-grid")).toBeVisible({
      timeout: 20_000,
    });
    await expect(installedExtensionCard).toBeVisible();
    await expect(installedExtensionCard.getByRole("switch")).toBeChecked();

    const routeState = await captureRouteState(appPage);
    const viewportEvidence = await captureViewportEvidence({
      page: appPage,
      browserArtifacts,
      moduleName: "extensibility-tools-resources",
      assertVisible: async () => {
        await expect(marketplaceWin.getByTestId("marketplace-installed-grid")).toBeVisible();
        await expect(installedExtensionCard).toBeVisible();
      },
    });

    const httpExtension = await runtime.requestJSON<ExtensionResponse>(
      `/api/extensions/${extensionName}`
    );
    const udsExtension = await requestBrowserRuntimeOperatorJSON<ExtensionResponse>(
      runtime,
      `/api/extensions/${extensionName}`
    );
    const cliExtension = await runBrowserRuntimeCLIJSON<ExtensionPayload>(runtime, [
      "extension",
      "status",
      extensionName,
    ]);
    await captureBrowserTransportSnapshot(runtime, "TC-EXT-001-extension", {
      http: projectExtension(httpExtension.extension),
      uds: projectExtension(udsExtension.extension),
      cli: projectExtension(cliExtension),
    });

    const httpTools = await runtime.requestJSON<ToolsResponse>("/api/tools");
    const udsTools = await requestBrowserRuntimeOperatorJSON<ToolsResponse>(runtime, "/api/tools");
    const cliTools = await runBrowserRuntimeCLIJSON<ToolsResponse>(runtime, ["tool", "list"]);
    const httpTool = expectTool(httpTools, "HTTP tool registry");
    const udsTool = expectTool(udsTools, "UDS tool registry");
    const cliTool = expectTool(cliTools, "CLI tool registry");
    expect(projectTool(httpTool)).toEqual(projectTool(udsTool));
    expect(projectTool(cliTool)).toMatchObject(projectTool(httpTool));
    await captureBrowserTransportSnapshot(runtime, "TC-EXT-001-tool-registry", {
      http: projectTool(httpTool),
      uds: projectTool(udsTool),
      cli: projectTool(cliTool),
    });

    const safeInput = { query: "release-readiness" };
    const sensitiveInput = { query: "browser-extension-api-key=secret-13" };
    const sessionID = "browser-extensibility-session";
    const httpApproval = await createToolApprovalHTTP(runtime, safeInput, sessionID);
    const udsApproval = await createToolApprovalUDS(runtime, safeInput, sessionID);
    const cliApproval = await runBrowserRuntimeCLIJSON<ToolApprovalResponse>(runtime, [
      "tool",
      "approve",
      toolID,
      "--session",
      sessionID,
      "--input",
      JSON.stringify(sensitiveInput),
    ]);
    expect(httpApproval.approval.input_digest).toBe(udsApproval.approval.input_digest);
    expect(cliApproval.approval.tool_id).toBe(toolID);

    const httpInvoke = await invokeToolHTTP(runtime, safeInput, "tc-ext-001-http");
    const udsInvoke = await invokeToolUDS(runtime, safeInput, "tc-ext-001-uds");
    const cliInvoke = await runBrowserRuntimeCLIJSON<ToolInvokeResponse>(runtime, [
      "tool",
      "invoke",
      toolID,
      "--input",
      JSON.stringify(sensitiveInput),
      "--sensitive-input-field",
      "query",
      "--correlation-id",
      "tc-ext-001-cli",
    ]);

    expectInvokeSucceeded(httpInvoke, "release-readiness");
    expectInvokeSucceeded(udsInvoke, "release-readiness");
    expect(cliInvoke.result.preview).not.toContain("browser-extension-api-key");
    expect(JSON.stringify(cliInvoke)).not.toContain(sensitiveInput.query);
    expect(JSON.stringify(cliInvoke)).not.toMatch(sensitiveArtifactPattern);
    expect(cliInvoke.result.redactions?.some(redaction => redaction.path === "query")).toBe(true);

    await captureBrowserTransportSnapshot(runtime, "TC-EXT-001-tool-invoke", {
      http: projectInvoke(httpInvoke),
      uds: projectInvoke(udsInvoke),
      cli: projectInvoke(cliInvoke),
    });

    const restart = await runtime.requestJSON<SettingsRestartAction>(
      "/api/settings/actions/restart",
      {
        method: "POST",
        body: "{}",
      }
    );
    expect(restart.operation_id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
    );
    await expect
      .poll(async () => await pollRestartStatus(runtime, restart.status_url), {
        timeout: 45_000,
      })
      .toBe("ready");
    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(marketplaceWin.getByTestId("marketplace-installed-grid")).toBeVisible({
      timeout: 20_000,
    });
    await expect(installedExtensionCard.getByRole("switch")).toBeChecked();
    await expect
      .poll(async () => {
        const payload = await runtime.requestJSON<ExtensionResponse>(
          `/api/extensions/${extensionName}`
        );
        return payload.extension.state;
      })
      .toBe("active");
    const postRestartTool = expectTool(
      await runtime.requestJSON<ToolsResponse>("/api/tools"),
      "HTTP tool registry after restart"
    );
    expect(projectTool(postRestartTool)).toMatchObject(projectTool(httpTool));
    const postRestartInvoke = await invokeToolHTTP(
      runtime,
      { query: "post-restart-readiness" },
      "tc-ext-001-restart"
    );
    expectInvokeSucceeded(postRestartInvoke, "post-restart-readiness");

    const crashFailure = await requestToolInvokeFailure(appPage, runtime, "crash-once");
    expect(crashFailure.status).toBeGreaterThanOrEqual(500);
    expect(crashFailure.body).toMatch(/extension|tool|process|provider|closed|failed/i);
    await expect
      .poll(
        async () => {
          try {
            const recovered = await invokeToolHTTP(
              runtime,
              { query: "recovered-after-crash" },
              "tc-ext-001-crash-recovery"
            );
            return recovered.result.preview ?? "";
          } catch {
            return "";
          }
        },
        { timeout: 30_000 }
      )
      .toContain("recovered-after-crash");

    const failureHTTP = await appPage.request.post(runtime.url("/api/extensions"), {
      data: { ref: checksumFailureDir, source: "local_path" },
    });
    expect(failureHTTP.status()).toBe(422);
    const checksumFailureBody = (await failureHTTP.json()) as { error?: unknown };
    expect(checksumFailureBody).toHaveProperty("error");
    expect(JSON.stringify(checksumFailureBody.error)).toMatch(/unverified|checksum/i);

    const malformedSourceHTTP = await appPage.request.post(runtime.url("/api/extensions"), {
      data: { ref: checksumFailureDir, source: "ftp" },
    });
    expect(malformedSourceHTTP.status()).toBe(400);
    expect(JSON.stringify(await malformedSourceHTTP.json())).toMatch(
      /unsupported extension source/i
    );

    const invalidInstall = await runCLIExpectFailure(runtime.paths, [
      "extension",
      "install",
      badManifestDir,
    ]);
    expect(`${invalidInstall.stdout}\n${invalidInstall.stderr}`).toMatch(
      /manifest|capabilities|invalid/i
    );

    const extensionList = await runtime.requestJSON<ExtensionsResponse>("/api/extensions");
    const daemonLog = await readFile(runtime.paths.daemonLog, "utf8");
    const finalArtifacts = {
      route_state: routeState,
      viewport_evidence: viewportEvidence,
      extensions: extensionList.extensions.map(projectExtension),
      fail_closed: {
        checksum_status: failureHTTP.status(),
        checksum_error: checksumFailureBody.error,
        invalid_manifest_stderr: invalidInstall.stderr.slice(0, 500),
      },
      disruption: {
        crash_status: crashFailure.status,
        post_restart: projectInvoke(postRestartInvoke),
      },
    };
    expect(daemonLog).not.toContain(extensionSecretSentinel);
    expect(JSON.stringify(finalArtifacts)).not.toContain(extensionSecretSentinel);
    assertNoSensitiveArtifactPayload([finalArtifacts, daemonLog]);
    await runtime.artifactCollector.captureJSON("browser_api_snapshots", finalArtifacts);
    await runtime.artifactCollector.captureJSON("browser_route_state", routeState);
    await browserArtifacts.persist(appPage);
  });

  test("operator approves and denies tool permission prompts with accessible browser controls and transport evidence", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    assertLaunchRuntime(runtime, "extensibility permission prompt lifecycle");
    const workspace = await prepareExtensibilitySessionRuntime(runtime, appPage);

    const approved = await createSession(runtime, toolPermissionAgent, workspace.id);
    await appPage.goto(runtime.url(sessionPath(toolPermissionAgent, approved.session.id)), {
      waitUntil: "domcontentloaded",
    });
    const approvedWin = sessionWindow(appPage, approved.session.id);
    const approvedUI = sessionWindowSelectors(approvedWin);
    await expect(approvedWin).toBeVisible();
    await approvedUI.composerTextarea.fill("exercise golden");
    await approvedUI.composerTextarea.press("Enter");
    await expect(approvedUI.chatView).toContainText("hello from golden");
    await expect(approvedUI.permissionPrompt).toBeVisible();
    await expect(approvedWin.getByRole("region", { name: "Permission required" })).toBeVisible();
    await expect(approvedWin.getByRole("button", { name: /always allow/i })).toBeVisible();
    await expect(approvedWin.getByRole("button", { name: /^reject 3$/i })).toBeVisible();
    await assertPermissionKeyboardPath(approvedWin);

    const approveResponsePromise = appPage.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        response.url().endsWith(sessionAPIPath(workspace.id, approved.session.id, "/approve"))
    );
    await approvedWin.getByTestId("permission-allow-always").click();
    expect((await approveResponsePromise).ok()).toBe(true);
    await expect(approvedUI.permissionPrompt).toBeHidden();
    const approvedSnapshot = await captureSessionSnapshot(
      runtime,
      workspace.id,
      approved.session.id
    );
    expect(JSON.stringify(approvedSnapshot.events)).toContain("allow-always");

    const denied = await createSession(runtime, denyPermissionAgent, workspace.id);
    await appPage.goto(runtime.url(sessionPath(denyPermissionAgent, denied.session.id)), {
      waitUntil: "domcontentloaded",
    });
    const deniedWin = sessionWindow(appPage, denied.session.id);
    const deniedUI = sessionWindowSelectors(deniedWin);
    await expect(deniedWin).toBeVisible();
    await deniedUI.composerTextarea.fill("exercise permission hardening");
    await deniedUI.composerTextarea.press("Enter");
    await expect(deniedUI.chatView).toContainText("Permission hardening started.");
    await expect(deniedUI.permissionPrompt).toBeVisible();
    await expect(deniedWin.getByTestId("permission-dock-subject")).toHaveText("hardening.txt");

    const rejectResponsePromise = appPage.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        response.url().endsWith(sessionAPIPath(workspace.id, denied.session.id, "/approve"))
    );
    await deniedWin.getByTestId("permission-reject-menu-trigger").click();
    await appPage.getByTestId("permission-reject-always").click();
    expect((await rejectResponsePromise).ok()).toBe(true);
    await expect(deniedUI.permissionPrompt).toBeHidden();
    const rejectedNotice = deniedWin.getByTestId("permission-rejected-notice");
    await expect(rejectedNotice).toHaveAttribute("data-decision", "reject-always");
    await expect(rejectedNotice).toContainText("hardening.txt");
    const deniedSnapshot = await captureSessionSnapshot(runtime, workspace.id, denied.session.id);
    expect(JSON.stringify(deniedSnapshot.events)).toContain("perm-hardening-reject-1");
    expect(JSON.stringify(deniedSnapshot.events)).toContain("reject-always");

    await runtime.artifactCollector.captureJSON("browser_api_snapshots", {
      approved: approvedSnapshot,
      denied: deniedSnapshot,
    });
    await browserArtifacts.captureScreenshot("extensibility-tool-permission-approve-deny", appPage);
    const manifest = await browserArtifacts.persist(appPage);
    expect(manifest.artifacts).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ kind: "browser_api_snapshots" }),
        expect.objectContaining({ kind: "browser_route_state" }),
        expect.objectContaining({ kind: "browser_screenshots" }),
      ])
    );
  });

  function assertLaunchRuntime(
    runtime: BrowserRuntime,
    label: string
  ): asserts runtime is BrowserRuntime & {
    paths: RuntimePaths;
  } {
    if (!runtime.paths) {
      throw new Error(`${label} requires launch-mode runtime paths`);
    }
  }

  function canonicalizeJSON(value: unknown): string {
    if (value === null || typeof value !== "object") {
      return JSON.stringify(value);
    }
    if (Array.isArray(value)) {
      return `[${value.map(canonicalizeJSON).join(",")}]`;
    }
    const record = value as Record<string, unknown>;
    return `{${Object.keys(record)
      .sort()
      .map(key => `${JSON.stringify(key)}:${canonicalizeJSON(record[key])}`)
      .join(",")}}`;
  }

  function schemaDigest(schema: unknown): string {
    return createHash("sha256").update(canonicalizeJSON(schema)).digest("hex");
  }

  async function createBrowserToolProviderExtension(): Promise<string> {
    const rootDir = await mkdtemp(path.join(os.tmpdir(), "compozy-browser-tool-provider-"));

    const inputSchema = {
      type: "object",
      required: ["query"],
      properties: {
        query: { type: "string" },
      },
    };

    await writeFile(
      path.join(rootDir, "extension.json"),
      JSON.stringify(
        {
          extension: {
            name: extensionName,
            version: "0.1.0",
            description: "Browser E2E extension tool provider",
            min_compozy_version: "0.0.0",
          },
          capabilities: { provides: ["tool.provider"] },
          subprocess: {
            command: "node",
            args: ["extension.js"],
          },
          resources: {
            tools: {
              search: {
                id: toolID,
                display_title: "Browser extension search",
                description: "Search extension-owned browser evidence",
                read_only: true,
                visibility: "session",
                risk: "read",
                max_result_bytes: 4096,
                backend: { kind: "extension_host", handler: "search" },
                input_schema: inputSchema,
                toolsets: ["compozy__catalog"],
              },
            },
          },
        },
        null,
        2
      ),
      "utf8"
    );

    await writeFile(
      path.join(rootDir, "extension.js"),
      extensionRuntimeSource(schemaDigest(inputSchema)),
      "utf8"
    );

    return rootDir;
  }

  async function createInvalidExtensionManifest(): Promise<string> {
    const rootDir = await mkdtemp(path.join(os.tmpdir(), "compozy-invalid-extension-"));
    await writeFile(
      path.join(rootDir, "extension.json"),
      JSON.stringify(
        {
          extension: {
            name: "browser-invalid-extension",
            version: "0.1.0",
            description: "Invalid browser E2E extension",
            min_compozy_version: "0.0.0",
          },
          capabilities: { provides: ["bad capability"] },
        },
        null,
        2
      ),
      "utf8"
    );
    return rootDir;
  }

  async function createChecksumFailureExtension(): Promise<string> {
    const rootDir = await mkdtemp(path.join(os.tmpdir(), "compozy-checksum-failure-extension-"));
    await writeFile(
      path.join(rootDir, "extension.json"),
      JSON.stringify(
        {
          extension: {
            name: "browser-checksum-reject",
            version: "0.1.0",
            description: "Valid extension used to prove checksum rejection",
            min_compozy_version: "0.0.0",
          },
          capabilities: { provides: [] },
        },
        null,
        2
      ),
      "utf8"
    );
    return rootDir;
  }

  function extensionRuntimeSource(inputSchemaDigest: string): string {
    return `
  const readline = require("node:readline");

  let initialized = false;

  const rl = readline.createInterface({ input: process.stdin });

  function sendResult(id, result) {
    process.stdout.write(JSON.stringify({ jsonrpc: "2.0", id, result }) + "\\n");
  }

  function sendError(id, code, message) {
    process.stdout.write(JSON.stringify({ jsonrpc: "2.0", id, error: { code, message } }) + "\\n");
  }

  rl.on("line", line => {
    if (!line.trim()) {
      return;
    }
    let frame;
    try {
      frame = JSON.parse(line);
    } catch {
      sendError(null, -32700, "Parse error");
      return;
    }
    const id = frame.id ?? null;
    if (frame.method === "initialize") {
      initialized = true;
      sendResult(id, {
        protocol_version: "1",
        extension_info: {
          name: "${extensionName}",
          version: "0.1.0",
          sdk_name: "browser-e2e-fixture",
          sdk_version: "0.1.0"
        },
        accepted_capabilities: { provides: ["tool.provider"], permissions: [] },
        implemented_methods: ["health_check", "provide_tools", "tools/call", "shutdown"],
        supported_hook_events: [],
        supports: { health_check: true }
      });
      return;
    }
    if (!initialized) {
      sendError(id, -32003, "Not initialized");
      return;
    }
    if (frame.method === "health_check") {
      sendResult(id, { healthy: true, message: "", details: {} });
      return;
    }
    if (frame.method === "provide_tools") {
      sendResult(id, {
        tools: [{
          id: "${toolID}",
          handler: "search",
          input_schema_digest: "${inputSchemaDigest}",
          read_only: true,
          risk: "read",
          capabilities: []
        }]
      });
      return;
    }
    if (frame.method === "tools/call") {
      const params = frame.params || {};
      const input = params.input || {};
      const query = typeof input.query === "string" ? input.query : "";
      if (query === "crash-once") {
        process.stderr.write("browser extension crash-once disruption probe\\n");
        process.exit(42);
        return;
      }
      const containsSecret = /secret|token|api[_-]?key/i.test(query);
      const safeQuery = containsSecret ? "[redacted]" : query;
      sendResult(id, {
        result: {
          content: [{ type: "text", text: "result " + safeQuery }],
          structured: { query: safeQuery, extension: "${extensionName}" },
          preview: "result " + safeQuery,
          redactions: containsSecret ? [{ path: "query", reason: "secret_metadata" }] : [],
          truncated: false,
          bytes: Buffer.byteLength(safeQuery),
          duration_ms: 0
        }
      });
      return;
    }
    if (frame.method === "shutdown") {
      sendResult(id, {});
      process.exit(0);
    }
    sendError(id, -32601, "Method not found");
  });
  `;
  }

  async function requestToolInvokeFailure(
    page: Page,
    runtime: BrowserRuntime,
    query: string
  ): Promise<{ status: number; body: string }> {
    const response = await page.request.post(runtime.url(`/api/tools/${toolID}/invoke`), {
      data: {
        input: { query },
        sensitive_input_fields: ["query"],
        correlation_id: "tc-ext-001-crash",
      },
      timeout: 15_000,
    });
    const body = await response.text();
    if (response.ok()) {
      throw new Error(`tool crash probe unexpectedly succeeded: ${body}`);
    }
    return { status: response.status(), body };
  }

  async function invokeToolHTTP(
    runtime: BrowserRuntime,
    input: Record<string, string>,
    correlationID: string
  ): Promise<ToolInvokeResponse> {
    return await runtime.requestJSON<ToolInvokeResponse>(`/api/tools/${toolID}/invoke`, {
      method: "POST",
      body: JSON.stringify({
        input,
        sensitive_input_fields: ["query"],
        correlation_id: correlationID,
      }),
    });
  }

  async function invokeToolUDS(
    runtime: BrowserRuntime,
    input: Record<string, string>,
    correlationID: string
  ): Promise<ToolInvokeResponse> {
    return await requestBrowserRuntimeOperatorJSON<ToolInvokeResponse>(
      runtime,
      `/api/tools/${toolID}/invoke`,
      {
        method: "POST",
        body: JSON.stringify({
          input,
          sensitive_input_fields: ["query"],
          correlation_id: correlationID,
        }),
      }
    );
  }

  async function createToolApprovalHTTP(
    runtime: BrowserRuntime,
    input: Record<string, string>,
    sessionID: string
  ): Promise<ToolApprovalResponse> {
    return await runtime.requestJSON<ToolApprovalResponse>(`/api/tools/${toolID}/approvals`, {
      method: "POST",
      body: JSON.stringify({
        session_id: sessionID,
        input,
      }),
    });
  }

  async function createToolApprovalUDS(
    runtime: BrowserRuntime,
    input: Record<string, string>,
    sessionID: string
  ): Promise<ToolApprovalResponse> {
    return await requestBrowserRuntimeOperatorJSON<ToolApprovalResponse>(
      runtime,
      `/api/tools/${toolID}/approvals`,
      {
        method: "POST",
        body: JSON.stringify({
          session_id: sessionID,
          input,
        }),
      }
    );
  }

  async function pollRestartStatus(runtime: BrowserRuntime, statusURL: string): Promise<string> {
    try {
      return (await runtime.requestJSON<SettingsRestartStatus>(statusURL)).status;
    } catch {
      return "restarting";
    }
  }

  async function prepareExtensibilitySessionRuntime(
    runtime: BrowserRuntime,
    page: Page
  ): Promise<WorkspacePayload> {
    const workspace = await runtime.resolveWorkspace(runtime.paths?.workspaceDir ?? process.cwd());
    await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
    await completeOnboardingIfPrompted(page);
    await switchWorkspace(page, workspace.id, workspace.name);
    return workspace;
  }

  function sessionPath(agentName: string, sessionID: string): string {
    return `/agents/${agentName}/sessions/${sessionID}`;
  }

  async function createSession(
    runtime: BrowserRuntime,
    agentName: string,
    workspaceID: string
  ): Promise<SessionEnvelope> {
    const payload = await runtime.requestJSON<SessionEnvelope>("/api/sessions", {
      method: "POST",
      body: JSON.stringify({
        agent_name: agentName,
        workspace: workspaceID,
      }),
    });
    expect(payload.session.id).not.toBe("");
    expect(payload.session.agent_name).toBe(agentName);
    expect(payload.session.workspace_id).toBe(workspaceID);
    return payload;
  }

  async function captureSessionSnapshot(
    runtime: BrowserRuntime,
    workspaceID: string,
    sessionID: string
  ): Promise<{
    events: SessionEventEnvelope;
    history: SessionHistoryEnvelope;
    session: SessionEnvelope;
    udsSession?: SessionEnvelope;
  }> {
    const basePath = sessionAPIPath(workspaceID, sessionID);
    const snapshot = {
      events: await runtime.requestJSON<SessionEventEnvelope>(`${basePath}/events`),
      history: await runtime.requestJSON<SessionHistoryEnvelope>(`${basePath}/history`),
      session: await runtime.requestJSON<SessionEnvelope>(basePath),
      udsSession: runtime.requestOperatorJSON
        ? await runtime.requestOperatorJSON<SessionEnvelope>(basePath)
        : undefined,
    };
    if (snapshot.udsSession) {
      expect(snapshot.udsSession.session.id).toBe(snapshot.session.session.id);
      expect(snapshot.udsSession.session.state).toBe(snapshot.session.session.state);
    }
    return snapshot;
  }

  function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
    return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
      sessionID
    )}${suffix}`;
  }

  async function assertPermissionKeyboardPath(win: Locator): Promise<void> {
    const page = win.page();
    const focusOrder = [
      "permission-allow-once",
      "permission-allow-always",
      "permission-reject-once",
      "permission-reject-menu-trigger",
    ];
    await win.getByTestId(focusOrder[0]).focus();
    for (const testID of focusOrder) {
      await expect(win.locator(`[data-testid="${testID}"]`)).toBeFocused();
      if (testID !== focusOrder.at(-1)) {
        await page.keyboard.press("Tab");
      }
    }
    await page.keyboard.press("Enter");
    await expect(page.getByTestId("permission-reject-menu")).toBeVisible();
    await expect(page.getByTestId("permission-reject-always")).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(page.getByTestId("permission-reject-menu")).toBeHidden();
    await expect(win.getByTestId("permission-reject-menu-trigger")).toBeFocused();
  }

  function expectTool(response: ToolsResponse, label: string): ToolPayload {
    const tool = response.tools.find(item => item.descriptor.tool_id === toolID);
    if (!tool) {
      throw new Error(`${label} did not expose ${toolID}`);
    }
    if (!tool.availability.available || !tool.availability.executable) {
      throw new Error(`${label} exposed an unavailable tool: ${JSON.stringify(projectTool(tool))}`);
    }
    expect(tool.availability.available).toBe(true);
    expect(tool.availability.executable).toBe(true);
    expect(tool.decision.visible_to_operator).toBe(true);
    if (!tool.decision.callable) {
      throw new Error(`${label} exposed a non-callable tool: ${JSON.stringify(projectTool(tool))}`);
    }
    expect(tool.descriptor.backend).toMatchObject({
      kind: "extension_host",
      extension_id: extensionName,
      handler: "search",
    });
    expect(tool.descriptor.source).toMatchObject({
      kind: "extension",
      owner: extensionName,
    });
    return tool;
  }

  function expectInvokeSucceeded(response: ToolInvokeResponse, query: string): void {
    expect(response.tool_id).toBe(toolID);
    expect(response.status).toBe("completed");
    expect(response.result.preview).toContain(query);
    expect(response.result.truncated).toBe(false);
  }

  function projectExtension(extension: ExtensionPayload): Record<string, unknown> {
    return {
      name: extension.name,
      enabled: extension.enabled,
      state: extension.state,
      daemon_running: extension.daemon_running,
      health: extension.health,
      capabilities: extension.capabilities ?? [],
    };
  }

  function projectTool(tool: ToolPayload): Record<string, unknown> {
    return {
      id: tool.descriptor.tool_id,
      backend: tool.descriptor.backend,
      source: tool.descriptor.source,
      visibility: tool.descriptor.visibility,
      read_only: tool.descriptor.read_only,
      risk: tool.descriptor.risk,
      available: tool.availability.available,
      executable: tool.availability.executable,
      reason_codes: tool.availability.reason_codes ?? [],
      decision: tool.decision,
    };
  }

  function projectInvoke(response: ToolInvokeResponse): Record<string, unknown> {
    return {
      tool_id: response.tool_id,
      status: response.status,
      preview: response.result.preview,
      truncated: response.result.truncated,
      redactions: response.result.redactions ?? [],
      event_count: response.events.length,
      events: response.events.map(event => ({
        tool_id: event.tool_id,
        source_kind: event.source_kind,
        source_owner: event.source_owner,
        redacted_input_fields: event.redacted_input_fields ?? [],
        has_input_digest: Boolean(event.input_digest),
        correlation_id: event.correlation_id,
      })),
    };
  }

  async function runCLIExpectFailure(paths: RuntimePaths, args: string[]): Promise<ExecFailure> {
    const finalArgs =
      args.includes("-o") || args.includes("--output") ? args : [...args, "-o", "json"];
    try {
      const result = await execFileAsync(paths.cliShim, finalArgs, {
        env: browserRuntimeCLIEnv(paths),
        maxBuffer: 10 * 1024 * 1024,
        timeout: 30_000,
      });
      throw new Error(`CLI command unexpectedly succeeded: ${result.stdout}`);
    } catch (error) {
      const failure = error as Error & {
        stdout?: string | Buffer;
        stderr?: string | Buffer;
        code?: unknown;
      };
      if (failure.message.startsWith("CLI command unexpectedly succeeded")) {
        throw failure;
      }
      return {
        stdout: String(failure.stdout ?? ""),
        stderr: String(failure.stderr ?? ""),
        code: failure.code,
      };
    }
  }

  function browserRuntimeCLIEnv(paths: RuntimePaths): NodeJS.ProcessEnv {
    return {
      ...process.env,
      COMPOZY_E2E_CLI_BIN: paths.cliShim,
      COMPOZY_HOME: paths.homeDir,
      HOME: paths.operatorHomeDir,
      PATH: [path.dirname(paths.cliShim), process.env.PATH ?? ""]
        .filter(Boolean)
        .join(path.delimiter),
    };
  }
});

test.describe("Extension update affordance", () => {
  const extensionName = "browser-update-extension";
  const repository = "acme/browser-update-extension";
  const catalogEntryID = "browser-update-extension";

  test.use({
    runtimeOptions: {
      extensionsAllowUnverified: true,
      seed: {
        extensionRegistry: {
          description: "Browser lane extension with two published releases.",
          extensionName,
          releases: [
            { tag: "v0.1.0", version: "0.1.0" },
            { tag: "v0.2.0", version: "0.2.0" },
          ],
          repository,
        },
        marketplaceCatalog: {
          extensions: [
            {
              artifact_url: `https://github.com/${repository}/releases/download/v0.2.0/${extensionName}.tar.gz`,
              author: "acme",
              description: "Browser lane extension with a newer catalog release.",
              digest_release_tag: "v0.2.0",
              entry_id: catalogEntryID,
              install_slug: repository,
              name: extensionName,
              repository: `https://github.com/${repository}`,
              tier: "unverified",
              version: "0.2.0",
            },
          ],
        },
      },
    },
  });

  test("operator sees a daemon-projected update on the landing scope and applies it from the extension detail", async ({
    appPage,
    runtime,
  }) => {
    if (!runtime.paths) {
      throw new Error("extension update affordance requires launch-mode runtime paths");
    }

    const installed = await runBrowserRuntimeCLIJSON<{
      name: string;
      version: string;
      provenance?: { slug?: string; digest_matched?: boolean };
    }>(runtime, [
      "extension",
      "install",
      `github:${repository}@v0.1.0`,
      "--allow-unverified",
      "--yes",
    ]);
    expect(installed.name).toBe(extensionName);
    expect(installed.version).toBe("0.1.0");
    expect(installed.provenance?.slug).toBe(repository);

    await completeOnboardingIfPrompted(appPage);
    await appPage.goto(runtime.url("/marketplace"), {
      waitUntil: "domcontentloaded",
    });
    const marketplaceWin = appWindow(appPage, "marketplace");
    await expect(marketplaceWin).toBeVisible();
    const marketplace = marketplaceOperatorSelectors(marketplaceWin);
    await expect(marketplaceWin.getByTestId("marketplace-card-" + catalogEntryID)).toBeVisible({
      timeout: 20_000,
    });

    await expect(marketplaceWin.getByTestId("marketplace-installed-shelf-updates")).toContainText(
      "1"
    );
    const catalogCard = marketplace.card(catalogEntryID);
    await expect(catalogCard).toContainText("v0.2.0");
    await expect(
      catalogCard.getByRole("button", { name: `Update ${extensionName}` })
    ).toBeVisible();

    await catalogCard.getByRole("link", { name: `View ${extensionName} details` }).click();
    await expect(marketplace.detail).toBeVisible({ timeout: 20_000 });
    const updateAction = marketplaceWin.getByRole("button", {
      name: `Update ${extensionName}`,
    });
    await expect(updateAction).toBeVisible();

    const updateResponse = appPage.waitForResponse(
      response =>
        response.request().method() === "PUT" &&
        new URL(response.url()).pathname === `/api/extensions/${extensionName}`
    );
    await updateAction.click();
    const trustDialog = appPage.getByTestId("extension-trust-dialog");
    await expect(
      trustDialog.getByRole("heading", { name: `Update ${extensionName}?` })
    ).toBeVisible();
    await trustDialog.getByRole("button", { name: "Update anyway" }).click();
    expect((await updateResponse).status()).toBe(200);

    await expect
      .poll(
        async () => {
          const payload = await runtime.requestJSON<{
            extension: { version: string };
          }>(`/api/extensions/${extensionName}`);
          return payload.extension.version;
        },
        { timeout: 20_000 }
      )
      .toBe("0.2.0");

    await appPage.goto(runtime.url("/marketplace/installed"), {
      waitUntil: "domcontentloaded",
    });
    const installedCard = appPage
      .getByTestId(`marketplace-installed-card-${extensionName}`)
      .or(appPage.getByTestId(`marketplace-installed-card-${catalogEntryID}`))
      .first();
    await expect(installedCard).toContainText("v0.2.0");
    await expect(installedCard.getByRole("button", { exact: true, name: "Update" })).toHaveCount(0);
  });
});

test.describe("Agent Plugins marketplace journeys", () => {
  const extensionName = "acme.tools";
  const repository = "acme/acme-tools";
  const catalogEntryID = "browser-agent-plugin";
  const driftedName = "drifted.tools";
  const driftedRepository = "acme/drifted-tools";
  const driftedEntryID = "browser-drifted-plugin";

  test.describe("Portable catalog entry", () => {
    test.use({
      runtimeOptions: {
        extensionsAllowUnverified: true,
        seed: {
          extensionRegistry: {
            description: "Portable package with one skill and one skipped sse server.",
            extensionName,
            layout: "agent-plugin",
            releases: [{ tag: "v2.1.0", version: "2.1.0" }],
            repository,
          },
          marketplaceCatalog: {
            extensions: [
              {
                artifact_url: `https://github.com/${repository}/releases/download/v2.1.0/${extensionName}.tar.gz`,
                author: "acme",
                description: "Deploy checks and a tools API server in the Agent Plugins format.",
                digest_release_tag: "v2.1.0",
                entry_id: catalogEntryID,
                format: "agent-plugin",
                install_slug: repository,
                name: extensionName,
                repository: `https://github.com/${repository}`,
                tier: "unverified",
                version: "2.1.0",
              },
            ],
          },
        },
      },
    });

    test("operator installs a badged catalog entry and reads its ingested and skipped components", async ({
      appPage,
      runtime,
    }) => {
      await ensureProjectWorkspace(appPage, runtime);
      await appPage.reload({ waitUntil: "domcontentloaded" });
      await completeOnboardingIfPrompted(appPage);
      await appPage.goto(runtime.url("/marketplace"), {
        waitUntil: "domcontentloaded",
      });

      const marketplaceWin = appWindow(appPage, "marketplace");
      await expect(marketplaceWin).toBeVisible();
      const marketplace = marketplaceOperatorSelectors(marketplaceWin);
      await expect(marketplaceWin.getByTestId("marketplace-card-" + catalogEntryID)).toBeVisible({
        timeout: 20_000,
      });

      const catalogCard = marketplace.card(catalogEntryID);
      await expect(catalogCard).toBeVisible({ timeout: 20_000 });
      await expect(catalogCard).toContainText("unverified");

      const installResponse = appPage.waitForResponse(
        response =>
          response.request().method() === "POST" &&
          new URL(response.url()).pathname === "/api/extensions"
      );
      await marketplace.action(catalogEntryID).click();

      const trustDialog = marketplace.extensionTrustDialog;
      await expect(trustDialog).toBeVisible({ timeout: 20_000 });
      await expect(trustDialog).not.toContainText(/permission/i);
      await marketplace.extensionTrustConfirm.click();
      await appPage.getByTestId("extension-install-summary-confirm").click();
      const install = await installResponse;
      expect(install.status()).toBe(201);

      await expect
        .poll(
          async () => {
            const payload = await runtime.requestJSON<{ extension: { format: string } }>(
              `/api/extensions/${extensionName}`
            );
            return payload.extension.format;
          },
          { timeout: 20_000 }
        )
        .toBe("agent-plugin");

      await appPage.goto(runtime.url("/marketplace/installed"), {
        waitUntil: "domcontentloaded",
      });
      const installedCard = appPage
        .getByTestId(`marketplace-installed-card-${extensionName}`)
        .or(appPage.getByTestId(`marketplace-installed-card-${catalogEntryID}`))
        .first();
      await expect(installedCard).toBeVisible({ timeout: 20_000 });
      await expect(installedCard).toContainText("1 skill");

      await installedCard.getByRole("link", { name: `View ${extensionName} details` }).click();
      const detail = marketplaceOperatorSelectors(appWindow(appPage, "marketplace"));
      await expect(detail.detail).toBeVisible({ timeout: 20_000 });
      await expect(detail.extensionFormatBadge.first()).toHaveText("agent plugin");

      await expect(detail.extensionKitInventory).toBeVisible({ timeout: 20_000 });
      await expect(detail.extensionKitInventoryItem.first()).toBeVisible();
      await expect(detail.extensionSkippedComponents).toBeVisible();
      const skippedRow = detail.extensionSkippedRow.first();
      await expect(skippedRow).toContainText("mcp: legacy-events");
      await expect(skippedRow).toContainText("sse transport is not supported");
      await expect(detail.extensionSkippedZeroResources).toHaveCount(0);
    });
  });

  test.describe("Drifted catalog entry", () => {
    test.use({
      runtimeOptions: {
        extensionsAllowUnverified: true,
        seed: {
          extensionRegistry: {
            description: "Upstream drifted to a Claude Code plugin layout.",
            extensionName: driftedName,
            layout: "client-layout",
            releases: [{ tag: "v1.0.0", version: "1.0.0" }],
            repository: driftedRepository,
          },
          marketplaceCatalog: {
            extensions: [
              {
                artifact_url: `https://github.com/${driftedRepository}/releases/download/v1.0.0/${driftedName}.tar.gz`,
                author: "acme",
                description: "A curated entry whose upstream no longer ships the standard layout.",
                digest_release_tag: "v1.0.0",
                entry_id: driftedEntryID,
                format: "agent-plugin",
                install_slug: driftedRepository,
                name: driftedName,
                repository: `https://github.com/${driftedRepository}`,
                tier: "unverified",
                version: "1.0.0",
              },
            ],
          },
        },
      },
    });

    test("operator reads the layout diagnostic in the dialog when a curated entry has drifted", async ({
      appPage,
      runtime,
    }) => {
      await ensureProjectWorkspace(appPage, runtime);
      await appPage.reload({ waitUntil: "domcontentloaded" });
      await completeOnboardingIfPrompted(appPage);
      await appPage.goto(runtime.url("/marketplace"), {
        waitUntil: "domcontentloaded",
      });

      const marketplaceWin = appWindow(appPage, "marketplace");
      await expect(marketplaceWin).toBeVisible();
      const marketplace = marketplaceOperatorSelectors(marketplaceWin);
      await expect(marketplaceWin.getByTestId("marketplace-card-" + driftedEntryID)).toBeVisible({
        timeout: 20_000,
      });
      await expect(marketplace.card(driftedEntryID)).toBeVisible({ timeout: 20_000 });

      await marketplace.action(driftedEntryID).click();
      const trustDialog = marketplace.extensionTrustDialog;
      await expect(trustDialog).toBeVisible({ timeout: 20_000 });
      await marketplace.extensionTrustConfirm.click();

      const failure = trustDialog.getByRole("alert");
      await expect(failure).toBeVisible({ timeout: 20_000 });
      await expect(failure).toContainText(".claude-plugin/plugin.json");
      await expect(failure).toContainText("Agent Plugins");
      await expect(trustDialog).toBeVisible();

      const installed = await runtime.requestJSON<{ extensions: Array<{ name: string }> }>(
        "/api/extensions"
      );
      expect(installed.extensions.some(entry => entry.name === driftedName)).toBe(false);
    });
  });
});
