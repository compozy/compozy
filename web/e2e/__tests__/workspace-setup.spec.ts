import { fileURLToPath } from "node:url";
import path from "node:path";

import { sessionLifecycleSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

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

test("operator runs onboarding, then re-opens the ruled workspace setup dialog from the sidebar add button", async ({
  appPage,
}) => {
  const ui = sessionLifecycleSelectors(appPage);

  await useGlobalWorkspaceIfPrompted(ui);
  await expect(ui.osDesktop).toBeVisible();

  await appPage.locator('[data-slot="os-menubar-workspace"]').click();
  await appPage.getByTestId("os-workspace-add").click();

  const dialog = appPage.getByTestId("workspace-setup-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveAttribute("data-frame", "unframed");

  const ruledHeader = dialog.locator('[data-slot="dialog-header"]');
  await expect(ruledHeader).toHaveAttribute("data-variant", "ruled");
  await expect(ruledHeader).toContainText("Add workspace");

  const dialogGlobalCard = dialog.getByTestId("workspace-setup-global-card");
  await expect(dialogGlobalCard).toHaveAttribute("data-size", "compact");

  // The ruled chrome means the trigger row uses px-5 / py-4. Verify computed rule via DOM box.
  const ruledHeaderBox = await ruledHeader.evaluate(node => {
    const computed = window.getComputedStyle(node as HTMLElement);
    return {
      borderBottomStyle: computed.borderBottomStyle,
      borderBottomWidth: computed.borderBottomWidth,
    };
  });
  expect(ruledHeaderBox.borderBottomStyle).toBe("solid");
  expect(parseFloat(ruledHeaderBox.borderBottomWidth)).toBeGreaterThan(0);

  // Closing the dialog returns to the operator app shell without losing the registered workspace.
  await appPage.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(ui.osDesktop).toBeVisible();
});

test.describe("first-run default model", () => {
  test.use({
    runtimeOptions: {
      seed: {
        mockAgents: [
          {
            fixturePath: browserLifecycleFixture,
            fixtureAgent: "browser-lifecycle-agent",
          },
        ],
      },
    },
  });

  test("operator persists a model and reasoning default through the unified selector", async ({
    appPage,
    runtime,
  }) => {
    await expect(appPage.getByTestId("onboarding-step-default-model")).toBeVisible();

    const runtimeTrigger = appPage.getByTestId("onboarding-runtime-select");
    await expect(runtimeTrigger).toBeVisible();
    // The closed selector is one button that opens the popup (no per-segment zones).
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
    // Reasoning is projected into the trigger's accessible summary, not a sub-button.
    await expect(runtimeTrigger).toHaveAccessibleName(/reasoning High/);

    const providerRequestPromise = appPage.waitForRequest(
      request =>
        request.method() === "PUT" &&
        request.url().endsWith(`/api/settings/providers/${mockAgentProvider}`)
    );
    const providerResponsePromise = appPage.waitForResponse(
      response =>
        response.request().method() === "PUT" &&
        response.url().endsWith(`/api/settings/providers/${mockAgentProvider}`)
    );
    const generalRequestPromise = appPage.waitForRequest(
      request => request.method() === "PATCH" && request.url().endsWith("/api/settings/general")
    );
    const generalResponsePromise = appPage.waitForResponse(
      response =>
        response.request().method() === "PATCH" && response.url().endsWith("/api/settings/general")
    );

    await expect(appPage.getByTestId("onboarding-continue")).toBeEnabled();
    await appPage.getByTestId("onboarding-continue").click();

    const providerRequest = await providerRequestPromise;
    const providerResponse = await providerResponsePromise;
    expect(providerRequest.postDataJSON()).toMatchObject({
      settings: { models: { default: reasoningCatalogModel } },
      model_curation: {
        model_id: reasoningCatalogModel,
        default_effort: "high",
      },
    });
    expect(providerResponse.ok()).toBe(true);

    const generalRequest = await generalRequestPromise;
    const generalResponse = await generalResponsePromise;
    expect(generalRequest.postDataJSON()).toMatchObject({
      config: { defaults: { provider: mockAgentProvider } },
    });
    expect(generalResponse.ok()).toBe(true);
    await expect(appPage.getByTestId("onboarding-step-workspaces")).toBeVisible();

    const provider = await runtime.requestJSON<{
      provider: {
        settings: {
          models?: {
            default?: string;
            curated?: Array<{ id: string; default_reasoning_effort?: string }>;
          };
        };
      };
    }>(`/api/settings/providers/${mockAgentProvider}`);
    expect(provider.provider.settings.models?.default).toBe(reasoningCatalogModel);
    expect(provider.provider.settings.models?.curated).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          id: reasoningCatalogModel,
          default_reasoning_effort: "high",
        }),
      ])
    );
  });
});
