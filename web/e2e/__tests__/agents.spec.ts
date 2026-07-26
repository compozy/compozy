import { fileURLToPath } from "node:url";
import path from "node:path";

import { openAppWindow } from "../fixtures/os-navigation";
import { sessionLifecycleSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { ensureGlobalWorkspace, useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const browserLifecycleFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "browser_session_lifecycle_fixture.json"
);
const mockAgentProvider = "acpmock";
const reasoningCatalogModel = "qa-browser-model-alt";
const reasoningCatalogModelLabel = "QA Browser Model Alt";

test.use({ viewport: { width: 1440, height: 900 } });

test("agent navigation renders the managed default agent after first-run setup", async ({
  appPage,
  runtime,
}) => {
  const ui = sessionLifecycleSelectors(appPage);

  await ensureGlobalWorkspace(runtime);
  await appPage.goto(runtime.url("/agents"), { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(ui);

  const agentsWin = appPage.getByTestId("os-window-app:agents");
  await expect(agentsWin).toBeVisible();
  const fleet = sessionLifecycleSelectors(agentsWin);
  await expect(agentsWin.getByTestId("agent-fleet-empty")).toHaveCount(0);
  await expect(fleet.agentRow("general")).toBeVisible();
});

test.describe("seeded agent detail", () => {
  test.use({
    runtimeOptions: {
      seed: {
        mockAgents: [
          {
            fixturePath: browserLifecycleFixture,
            fixtureAgent: "browser-lifecycle-agent",
            agentName: "agent-detail-primary",
          },
          {
            fixturePath: browserLifecycleFixture,
            fixtureAgent: "browser-lifecycle-agent",
            agentName: "agent-detail-secondary",
            category_path: ["Engineering"],
          },
        ],
      },
    },
  });

  test("operator opens an agent detail page and changes the session command picker selection", async ({
    appPage,
  }) => {
    const ui = sessionLifecycleSelectors(appPage);

    await useGlobalWorkspaceIfPrompted(ui);
    const agentsWin = await openAppWindow(appPage, "Agents", "agents");
    const fleet = sessionLifecycleSelectors(agentsWin);
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents");

    await expect(fleet.agentRow("agent-detail-primary")).toBeVisible();
    await expect(fleet.agentRow("agent-detail-secondary")).toBeVisible();
    await fleet.agentRow("agent-detail-primary").click();
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents/agent-detail-primary");

    await expect(appPage.getByTestId("agent-detail-page")).toBeVisible();
    await expect(
      appPage.getByRole("heading", { level: 1, name: "agent-detail-primary" })
    ).toBeVisible();
    await expect(appPage.getByTestId("agent-detail-tabs")).toBeVisible();
    await expect(appPage.getByTestId("agent-overview-tab")).toBeVisible();
    await expect(appPage.getByTestId("agent-stats-grid")).not.toContainText(
      String.fromCharCode(0x2014)
    );

    await appPage.getByTestId("agent-tab-configuration").click();
    await expect.poll(() => new URL(appPage.url()).searchParams.get("tab")).toBe("configuration");
    await expect(appPage.getByTestId("agent-configuration-tab")).toBeVisible();
    await expect(appPage.getByTestId("agent-mcp-empty")).toBeVisible();

    await appPage.getByTestId("agent-tab-sessions").click();
    await expect.poll(() => new URL(appPage.url()).searchParams.get("tab")).toBe("sessions");
    await expect(appPage.getByTestId("agent-sessions-empty")).toBeVisible();

    await fleet.agentPageNewSession.click();
    const trigger = appPage.getByTestId("session-create-agent-select");
    await expect(trigger).toBeVisible();
    await trigger.click();

    const secondary = appPage.getByTestId("agent-command-item-agent-detail-secondary");
    await expect(secondary).toBeVisible();
    await secondary.click();
    await expect(trigger).toContainText("agent-detail-secondary");

    await trigger.click();
    await expect(secondary).toHaveAttribute("data-checked", "true");
  });

  test("operator edits agent settings, saves, and returns to overview", async ({ appPage }) => {
    const ui = sessionLifecycleSelectors(appPage);
    await useGlobalWorkspaceIfPrompted(ui);
    const agentsWin = await openAppWindow(appPage, "Agents", "agents");
    const fleet = sessionLifecycleSelectors(agentsWin);
    await fleet.agentRow("agent-detail-primary").click();
    await expect(appPage.getByTestId("agent-detail-page")).toBeVisible();

    await appPage.getByTestId("agent-page-overflow").click();
    await appPage.getByTestId("agent-page-edit-settings").click();
    await expect
      .poll(() => new URL(appPage.url()).pathname)
      .toBe("/agents/agent-detail-primary/settings");
    await expect(appPage.getByTestId("agent-settings-page")).toBeVisible();
    await expect(appPage.getByTestId("agent-settings-basics")).toBeVisible();
    await expect(appPage.getByTestId("agent-settings-name")).toHaveAttribute("readonly");

    await appPage.getByTestId("agent-settings-nav-instructions").click();
    await expect
      .poll(() => new URL(appPage.url()).searchParams.get("section"))
      .toBe("instructions");
    const prompt = appPage.getByTestId("agent-settings-prompt");
    await prompt.fill("Updated prompt for settings journey.");
    await expect(appPage.getByTestId("agent-settings-unsaved")).toBeVisible();
    await expect(appPage.getByTestId("agent-settings-save")).toBeEnabled();

    await appPage.evaluate(() => window.history.back());
    await expect(appPage.getByTestId("unsaved-guard-dialog")).toBeVisible();
    await appPage.getByTestId("unsaved-guard-keep-editing").click();
    await expect
      .poll(() => new URL(appPage.url()).pathname)
      .toBe("/agents/agent-detail-primary/settings");
    await expect(prompt).toHaveValue("Updated prompt for settings journey.");

    const saveResponse = appPage.waitForResponse(
      response =>
        response.request().method() === "PUT" &&
        response.url().includes("/api/agents/agent-detail-primary")
    );
    await appPage.getByRole("button", { name: "Save changes" }).click();
    expect((await saveResponse).ok()).toBe(true);
    await expect(appPage.getByTestId("agent-settings-unsaved")).toHaveCount(0);

    await appPage.getByTestId("agent-settings-close").click();
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents/agent-detail-primary");
    await expect(appPage.getByTestId("agent-overview-tab")).toBeVisible();
  });

  test("operator duplicates an agent from the overflow menu", async ({ appPage }) => {
    const ui = sessionLifecycleSelectors(appPage);
    await useGlobalWorkspaceIfPrompted(ui);
    const agentsWin = await openAppWindow(appPage, "Agents", "agents");
    const fleet = sessionLifecycleSelectors(agentsWin);
    await fleet.agentRow("agent-detail-primary").click();

    await appPage.getByTestId("agent-page-overflow").click();
    await appPage.getByTestId("agent-page-duplicate").click();
    await expect(appPage.getByTestId("agent-create-dialog")).toBeVisible();
    await expect(appPage.getByTestId("agent-create-name")).toHaveValue("");
    await expect(appPage.getByText("MCP servers are not copied.")).toHaveCount(0);
  });

  test("operator deletes an agent from settings danger zone with typed confirm", async ({
    appPage,
  }) => {
    const ui = sessionLifecycleSelectors(appPage);
    await useGlobalWorkspaceIfPrompted(ui);
    const agentsWin = await openAppWindow(appPage, "Agents", "agents");
    const fleet = sessionLifecycleSelectors(agentsWin);
    await fleet.agentRow("agent-detail-secondary").click();
    await appPage.getByTestId("agent-page-overflow").click();
    await appPage.getByTestId("agent-page-edit-settings").click();
    await appPage.getByTestId("agent-settings-nav-danger").click();
    await appPage.getByTestId("agent-settings-delete").click();
    await expect(appPage.getByTestId("agent-delete-dialog")).toBeVisible();
    await expect(appPage.getByTestId("agent-delete-confirm")).toBeDisabled();
    await appPage.getByTestId("agent-delete-confirm-typing").fill("agent-detail-secondary");
    await expect(appPage.getByTestId("agent-delete-confirm")).toBeEnabled();
    await appPage.getByTestId("agent-delete-confirm").click();
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents");
    await expect(fleet.agentRow("agent-detail-secondary")).toHaveCount(0);
  });

  test("operator creates an agent with a persisted model and reasoning default", async ({
    appPage,
    runtime,
  }) => {
    if (!runtime.paths) {
      throw new Error("agent create browser test requires launch-mode runtime paths");
    }

    await ensureGlobalWorkspace(runtime);
    const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
    await appPage.goto(runtime.url("/agents"), { waitUntil: "domcontentloaded" });
    await useGlobalWorkspaceIfPrompted(sessionLifecycleSelectors(appPage));

    await appPage.getByTestId("agents-topbar-create").click();
    await expect(appPage.getByTestId("agent-create-dialog")).toBeVisible();
    await appPage.getByTestId("agent-create-name").fill("reasoning-default-agent");
    await appPage.getByTestId("agent-create-next").click();
    await expect(appPage.getByTestId("agent-create-runtime")).toBeVisible();

    const runtimeTrigger = appPage.getByTestId("agent-create-runtime-select");
    await runtimeTrigger.click();
    await expect(appPage.getByTestId("runtime-selector-popup")).toBeVisible();
    await appPage
      .locator(`[data-provider="${mockAgentProvider}"][data-model="${reasoningCatalogModel}"]`)
      .click();

    const reasoningStrip = appPage.getByTestId("runtime-selector-reasoning");
    await expect(reasoningStrip).toHaveAttribute("data-reasoning-mode", "levels");
    await reasoningStrip.locator('button[data-rz="high"]').click();
    await appPage.keyboard.press("Escape");
    await expect(runtimeTrigger).toContainText(reasoningCatalogModelLabel);
    await expect(runtimeTrigger).toContainText("High");

    await appPage.getByTestId("agent-create-next").click();
    await appPage.getByTestId("agent-create-prompt").fill("Keep runtime selection truthful.");
    await appPage.getByTestId("agent-create-next").click();
    await expect(appPage.getByTestId("agent-create-access")).toBeVisible();

    const createRequestPromise = appPage.waitForRequest(
      request => request.method() === "POST" && request.url().endsWith("/api/agents")
    );
    const createResponsePromise = appPage.waitForResponse(
      response => response.request().method() === "POST" && response.url().endsWith("/api/agents")
    );
    await appPage.getByTestId("submit-agent-create").click();

    const createRequest = await createRequestPromise;
    const createResponse = await createResponsePromise;
    expect(createRequest.postDataJSON()).toMatchObject({
      scope: "workspace",
      workspace: workspace.id,
      agent: {
        name: "reasoning-default-agent",
        provider: mockAgentProvider,
        model: reasoningCatalogModel,
        reasoning_effort: "high",
        prompt: "Keep runtime selection truthful.",
      },
    });
    expect(createResponse.ok()).toBe(true);

    const created = (await createResponse.json()) as {
      agent: { name: string; provider: string; model?: string; reasoning_effort?: string };
    };
    expect(created.agent).toMatchObject({
      name: "reasoning-default-agent",
      provider: mockAgentProvider,
      model: reasoningCatalogModel,
      reasoning_effort: "high",
    });
    await expect
      .poll(() => new URL(appPage.url()).pathname)
      .toBe("/agents/reasoning-default-agent");

    const rehydrated = await runtime.requestJSON<{
      agent: { name: string; provider: string; model?: string; reasoning_effort?: string };
    }>(`/api/agents/reasoning-default-agent?workspace=${encodeURIComponent(workspace.id)}`);
    expect(rehydrated.agent).toMatchObject({
      name: "reasoning-default-agent",
      provider: mockAgentProvider,
      model: reasoningCatalogModel,
      reasoning_effort: "high",
    });
  });
});

test.describe("fleet scan journey", () => {
  test.use({
    runtimeOptions: {
      seed: {
        mockAgents: [
          {
            fixturePath: browserLifecycleFixture,
            fixtureAgent: "browser-lifecycle-agent",
            agentName: "fleet-release",
            category_path: ["Engineering", "Release"],
          },
          {
            fixturePath: browserLifecycleFixture,
            fixtureAgent: "browser-lifecycle-agent",
            agentName: "fleet-ops",
            category_path: ["Operations"],
          },
        ],
      },
    },
  });

  test("operator lands on /agents, searches, filters by category, clears, and opens an agent", async ({
    appPage,
  }) => {
    const ui = sessionLifecycleSelectors(appPage);
    await useGlobalWorkspaceIfPrompted(ui);
    const agentsWin = await openAppWindow(appPage, "Agents", "agents");
    const fleet = sessionLifecycleSelectors(agentsWin);
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents");

    await expect(fleet.agentRow("fleet-release")).toBeVisible();
    await expect(fleet.agentRow("fleet-ops")).toBeVisible();

    await appPage.getByTestId("listing-view-cards").click();
    await expect.poll(() => new URL(appPage.url()).searchParams.get("view")).toBe("cards");
    await expect(appPage.getByTestId("agent-fleet-card-grid")).toBeVisible();
    await expect(appPage.getByTestId("agent-fleet-card-fleet-release")).toBeVisible();
    await appPage.getByTestId("listing-view-rows").click();
    await expect.poll(() => new URL(appPage.url()).searchParams.get("view")).toBeNull();
    await expect(fleet.agentRow("fleet-release")).toBeVisible();

    await appPage.getByTestId("agent-fleet-search").fill("release");
    await expect.poll(() => new URL(appPage.url()).searchParams.get("q")).toBe("release");
    await expect(fleet.agentRow("fleet-release")).toBeVisible();
    await expect(fleet.agentRow("fleet-ops")).toHaveCount(0);

    const search = appPage.getByTestId("agent-fleet-search");
    await appPage.getByTestId("agent-fleet-filters-add").click();
    await appPage.getByRole("option", { name: "Category" }).click();
    await appPage.getByRole("option", { name: "Engineering / Release" }).click();
    await expect
      .poll(() => new URL(appPage.url()).searchParams.get("category"))
      .toBe("Engineering / Release");

    await search.fill("ops");
    await expect(search).toHaveValue("ops");
    await expect.poll(() => new URL(appPage.url()).searchParams.get("q")).toBe("ops");
    await expect(appPage.getByTestId("agent-fleet-filtered-empty")).toBeVisible();
    await appPage.getByTestId("agent-fleet-clear-filters").click();
    await expect.poll(() => new URL(appPage.url()).search).toBe("");
    await expect(fleet.agentRow("fleet-ops")).toBeVisible();

    await fleet.agentRow("fleet-release").click();
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents/fleet-release");
  });
});

test.describe("empty-fleet first-contact journey", () => {
  test("operator sees first-run empty copy and can open New agent", async ({
    appPage,
    runtime,
  }) => {
    await appPage.route("**/api/agents**", async route => {
      const url = new URL(route.request().url());
      if (url.pathname === "/api/agents/catalog") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            agents: [],
            facets: { active: 0, categories: [], idle: 0, total: 0 },
            page: {
              has_more: false,
              limit: Number(url.searchParams.get("limit") ?? "50"),
              total: 0,
            },
            sessions_available: true,
          }),
        });
        return;
      }
      if (url.pathname !== "/api/agents") {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ agents: [] }),
      });
    });
    await ensureGlobalWorkspace(runtime);
    await appPage.goto(runtime.url("/agents"), { waitUntil: "domcontentloaded" });
    const ui = sessionLifecycleSelectors(appPage);
    await useGlobalWorkspaceIfPrompted(ui);
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents");

    await expect(appPage.getByTestId("agent-fleet-empty")).toBeVisible();
    await expect(appPage.getByText("No agents yet")).toBeVisible();
    await expect(
      appPage.getByText("Agents define the provider, model, and instructions a session runs with.")
    ).toBeVisible();
    await expect(appPage.getByTestId("agents-topbar-create")).toHaveCount(0);
    await appPage.getByTestId("agent-fleet-empty-create").click();
    await expect(appPage.getByTestId("agent-create-dialog")).toBeVisible();
  });
});
