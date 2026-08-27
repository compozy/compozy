import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Locator, Page } from "@playwright/test";

import { appWindow, commandPalette, switchWorkspace } from "../fixtures/os-navigation";
import { marketplaceOperatorSelectors, profilesOperatorSelectors } from "../fixtures/selectors";
import type { BrowserRuntime, RuntimePaths } from "../fixtures/runtime";
import { runBrowserRuntimeCLIJSON } from "../fixtures/scenario-contracts";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

// E2E-006: the browser faithfully binds workspace dev-overlay metadata, the redacted log stream,
// and the local-path branch of the source-union install contract.
test.describe("Extension dev overlay and source-union install", () => {
  const extensionName = "browser-dev-extension";
  const unionExtensionName = "browser-union-extension";
  const logSentinel = "browser-dev-extension online";
  const buildTimeoutMs = 300_000;
  const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..", "..");

  test.use({ runtimeOptions: { extensionsAllowUnverified: true } });

  test("operator inspects a workspace dev overlay with logs and installs a local path through the union contract", async ({
    appPage,
    runtime,
  }) => {
    test.slow();
    requireLaunchPaths(runtime);
    const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-browser-extension-"));
    const workspace = await runtime.resolveWorkspace(workspaceRoot);
    const sourceDir = await scaffoldGoExtension(
      runtime,
      extensionName,
      logSentinel,
      workspace.root_dir
    );
    const build = await runBrowserRuntimeCLIJSON<{ generation_dir: string }>(
      runtime,
      ["extension", "build", sourceDir],
      { timeoutMs: buildTimeoutMs }
    );

    // The published row the workspace overlay shadows.
    const published = await runBrowserRuntimeCLIJSON<{ name: string }>(runtime, [
      "extension",
      "install",
      build.generation_dir,
      "--allow-unverified",
      "--yes",
    ]);
    expect(published.name).toBe(extensionName);

    const linked = await runBrowserRuntimeCLIJSON<{
      dev: boolean;
      name: string;
      origin_path?: string;
      overrides_published?: boolean;
    }>(runtime, ["extension", "dev", sourceDir, "--workspace", workspace.root_dir], {
      timeoutMs: buildTimeoutMs,
    });
    expect(linked).toMatchObject({ dev: true, name: extensionName, overrides_published: true });

    await completeOnboardingIfPrompted(appPage);
    await switchWorkspace(appPage, workspace.id, workspace.name);
    await appPage.goto(runtime.url(`/marketplace/extension/${extensionName}`), {
      waitUntil: "domcontentloaded",
    });
    const marketplaceWin = appWindow(appPage, "marketplace");
    await expect(marketplaceWin).toBeVisible();
    const marketplace = marketplaceOperatorSelectors(marketplaceWin);
    await expect(marketplace.detail).toBeVisible({ timeout: 20_000 });

    await expect(marketplace.extensionDevBadge).toHaveText("dev");
    await expect(marketplace.extensionOverridesPublishedBadge).toHaveText("overrides published");
    await expect(marketplace.extensionOriginPath).toContainText(sourceDir);
    // Enable/disable and marketplace updates act on the published row, never on the overlay.
    await expect(marketplaceWin.getByTestId("extension-enabled-switch")).toHaveAttribute(
      "aria-disabled",
      "true"
    );
    await expect(
      marketplaceWin.getByRole("button", { name: `Update ${extensionName}` })
    ).toHaveCount(0);

    await expect(marketplace.extensionLogsPanel).toBeVisible();
    await expect(marketplace.extensionLogsLines).toContainText(logSentinel, { timeout: 30_000 });
    await marketplace.extensionLogsFollow.click();
    await expect(marketplace.extensionLogsStatus).toContainText("Paused");
    await expect(marketplace.extensionLogsLines).toContainText(logSentinel);

    await appPage.goto(runtime.url("/marketplace/extensions"), { waitUntil: "domcontentloaded" });
    await expect(marketplace.kind("extension")).toBeVisible({ timeout: 20_000 });
    await marketplace.extensionInstallEntry.click();
    await expect(marketplace.extensionInstallDialog).toBeVisible();

    await marketplace.extensionInstallRef.fill("relative/dist");
    await marketplace.extensionInstallSubmit.click();
    await expect(marketplace.extensionInstallRefError).toContainText("absolute path");

    const unionSourceDir = await scaffoldGoExtension(runtime, unionExtensionName);
    const unionBuild = await runBrowserRuntimeCLIJSON<{ generation_dir: string }>(
      runtime,
      ["extension", "build", unionSourceDir],
      { timeoutMs: buildTimeoutMs }
    );
    const unionDir = unionBuild.generation_dir;
    await marketplace.extensionInstallRef.fill(unionDir);
    await marketplace.extensionInstallAllowUnverified.click();
    await marketplace.extensionInstallSubmit.click();
    await expect(marketplaceWin.getByTestId("extension-install-summary")).toBeVisible();
    await marketplace.extensionInstallSubmit.click();
    // An unverified archive never installs without the daemon's explicit consent decision.
    await expect(marketplace.extensionTrustDialog).toBeVisible();

    const installResponse = appPage.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        new URL(response.url()).pathname === "/api/extensions"
    );
    await marketplace.extensionTrustConfirm.click();
    const response = await installResponse;
    expect(response.status(), await response.text()).toBe(201);
    expect(JSON.parse(response.request().postData() ?? "{}")).toMatchObject({
      allow_unverified: true,
      ref: unionDir,
      source: "local_path",
    });

    await expect
      .poll(async () => {
        const payload = await runtime.requestJSON<{ extensions: Array<{ name: string }> }>(
          "/api/extensions"
        );
        return payload.extensions.some(item => item.name === unionExtensionName);
      })
      .toBe(true);
  });

  function requireLaunchPaths(value: BrowserRuntime): RuntimePaths {
    if (!value.paths) {
      throw new Error("extension dev overlay coverage requires launch-mode runtime paths");
    }
    return value.paths;
  }

  async function scaffoldGoExtension(
    value: BrowserRuntime,
    name: string,
    stderrLine?: string,
    parentDir?: string
  ): Promise<string> {
    const stageRoot = parentDir
      ? await mkdtemp(path.join(parentDir, ".compozy-browser-dev-extension-"))
      : await mkdtemp(path.join(os.tmpdir(), "compozy-browser-dev-extension-"));
    const sourceDir = path.join(stageRoot, name);
    await runBrowserRuntimeCLIJSON<{ directory: string }>(value, [
      "extension",
      "init",
      name,
      "--template",
      "tool-provider-go",
      "--dir",
      sourceDir,
    ]);
    const goModPath = path.join(sourceDir, "go.mod");
    const goMod = await readFile(goModPath, "utf8");
    await writeFile(
      goModPath,
      `${goMod}\nreplace github.com/compozy/compozy/sdk/go => ${path.join(repoRoot, "sdk", "go")}\n`,
      "utf8"
    );
    if (stderrLine !== undefined) {
      // Deterministic stderr on activation so the redacted ring buffer has a line to stream.
      const mainPath = path.join(sourceDir, "main.go");
      const mainSource = await readFile(mainPath, "utf8");
      await writeFile(
        mainPath,
        mainSource.replace(
          "\tif err := extension.Run(context.Background()); err != nil {",
          `\tfmt.Fprintln(os.Stderr, ${JSON.stringify(stderrLine)})\n\tif err := extension.Run(context.Background()); err != nil {`
        ),
        "utf8"
      );
    }
    return sourceDir;
  }
});

// Invariant: the extension detail projects one profile's effective state while
// declared-profile setup and absent name-bound placements remain visible.
// Owner: extension browser management journey.
// Canonical suite: extension Playwright tests.
test.describe("Profile-aware extension management", () => {
  const extensionName = "browser-profile-kit";

  test.use({ runtimeOptions: { extensionsAllowUnverified: true } });

  test("E2E-023: placement, setup, dormancy, and enablement follow the active profile", async ({
    appPage,
    runtime,
  }) => {
    const sourceDir = await scaffoldProfileKitExtension();
    await runBrowserRuntimeCLIJSON(runtime, [
      "extension",
      "install",
      sourceDir,
      "--allow-unverified",
      "--yes",
    ]);

    await completeOnboardingIfPrompted(appPage);
    await appPage.goto(runtime.url(`/marketplace/extension/${extensionName}`), {
      waitUntil: "domcontentloaded",
    });

    const marketplaceWin = appWindow(appPage, "marketplace");
    await expect(marketplaceWin).toBeVisible();
    const marketplace = marketplaceOperatorSelectors(marketplaceWin);
    await expect(marketplace.detail).toBeVisible({ timeout: 20_000 });

    const declaredProfiles = marketplaceWin.getByTestId("extension-declared-profiles");
    await expect(declaredProfiles).toContainText("growth");
    await expect(declaredProfiles).toContainText("Needs setup");
    await expect(declaredProfiles).toContainText("finance");
    await expect(marketplaceWin.getByTestId("extension-dormant-finance")).toContainText(
      "finance-only"
    );
    await declaredProfiles.getByText("Placement matrix").click();
    await expect(declaredProfiles).toContainText("shared");
    await expect(declaredProfiles).toContainText("growth-only");

    const defaultToggle = marketplaceWin.getByTestId("extension-enabled-switch");
    await expect(defaultToggle).toBeChecked();
    const disabledResponse = appPage.waitForResponse(
      response =>
        response.request().method() === "PUT" &&
        new URL(response.url()).pathname === `/api/extensions/${extensionName}/enablement`
    );
    await defaultToggle.click();
    expect((await disabledResponse).ok()).toBe(true);
    await expect(defaultToggle).not.toBeChecked();
    await expect
      .poll(async () => {
        const enablement = await runtime.requestJSON<Array<{ enabled: boolean; profile: string }>>(
          `/api/extensions/${extensionName}/enablement?profile=default`
        );
        return enablement.find(item => item.profile === "default")?.enabled;
      })
      .toBe(false);

    await fulfillGrowthProfileCredential(runtime);
    await appPage.reload({ waitUntil: "domcontentloaded" });
    const refreshedMarketplace = appWindow(appPage, "marketplace");
    const refreshedDeclaredProfiles = refreshedMarketplace.getByTestId(
      "extension-declared-profiles"
    );
    await expect(refreshedDeclaredProfiles).toBeVisible({ timeout: 20_000 });
    await expect(refreshedDeclaredProfiles.getByText("Needs setup")).toHaveCount(0);

    const profiles = profilesOperatorSelectors(appPage);
    await profiles.switcher.click();
    await profiles.switcherOption("growth").click();
    await expect(profiles.switcher).toContainText("growth");
    await appPage.goto(runtime.url(`/marketplace/extension/${extensionName}`), {
      waitUntil: "domcontentloaded",
    });
    const growthMarketplace = appWindow(appPage, "marketplace");
    await expect(growthMarketplace.getByTestId("extension-enabled-switch")).toBeChecked();
    await expect(growthMarketplace.getByText("growth", { exact: true }).last()).toBeVisible();
  });

  test("E2E-030: a placed palette command follows profile enablement and catalog revision", async ({
    appPage,
    runtime,
  }) => {
    const sourceDir = await scaffoldProfileKitExtension();
    const workspace = await runtime.resolveWorkspace(sourceDir);
    await runBrowserRuntimeCLIJSON(runtime, [
      "extension",
      "install",
      sourceDir,
      "--allow-unverified",
      "--yes",
    ]);
    await fulfillGrowthProfileCredential(runtime);

    await completeOnboardingIfPrompted(appPage);
    await switchWorkspace(appPage, workspace.id, workspace.name);
    const profiles = profilesOperatorSelectors(appPage);
    await profiles.switcher.click();
    await profiles.switcherOption("growth").click();

    const commandID = `ext.${extensionName}.open-growth`;
    const query = `/api/cmd-palette/commands?workspace=${encodeURIComponent(workspace.id)}&profile=growth`;
    const before = await runtime.requestJSON<{
      catalog_revision: string;
      commands: Array<{ id: string }>;
    }>(query);
    expect(before.commands.map(command => command.id)).toContain(commandID);

    let palette = await openAndFillCommandPalette(appPage, "Open Growth Dashboard");
    await expect(palette.getByTestId(`os-palette-command-${commandID}`)).toBeVisible();
    await appPage.keyboard.press("Escape");
    await expect(palette).toHaveCount(0);

    await runtime.requestJSON(`/api/extensions/${extensionName}/enablement`, {
      body: JSON.stringify({ enabled: false, profile: "growth" }),
      method: "PUT",
    });
    const disabled = await runtime.requestJSON<{
      catalog_revision: string;
      commands: Array<{ id: string }>;
    }>(query);
    expect(disabled.catalog_revision).not.toBe(before.catalog_revision);
    expect(disabled.commands.map(command => command.id)).not.toContain(commandID);

    palette = await openAndFillCommandPalette(appPage, "Open Growth Dashboard");
    await expect(palette.getByTestId(`os-palette-command-${commandID}`)).toHaveCount(0);
    await appPage.keyboard.press("Escape");
    await expect(palette).toHaveCount(0);

    await runtime.requestJSON(`/api/extensions/${extensionName}/enablement`, {
      body: JSON.stringify({ enabled: true, profile: "growth" }),
      method: "PUT",
    });
    const restored = await runtime.requestJSON<{
      catalog_revision: string;
      commands: Array<{ id: string }>;
    }>(query);
    expect(restored.catalog_revision).not.toBe(disabled.catalog_revision);
    expect(restored.commands.map(command => command.id)).toContain(commandID);

    palette = await openAndFillCommandPalette(appPage, "Open Growth Dashboard");
    await expect(palette.getByTestId(`os-palette-command-${commandID}`)).toBeVisible();
  });

  async function scaffoldProfileKitExtension(): Promise<string> {
    const rootDir = await mkdtemp(path.join(os.tmpdir(), "compozy-browser-profile-kit-"));
    for (const name of ["shared", "growth-only", "finance-only"]) {
      const skillDir = path.join(rootDir, "skills", name);
      await mkdir(skillDir, { recursive: true });
      await writeFile(
        path.join(skillDir, "SKILL.md"),
        `---\nname: ${name}\ndescription: Browser profile fixture\n---\n`,
        "utf8"
      );
    }
    await writeFile(
      path.join(rootDir, "extension.json"),
      JSON.stringify(
        {
          extension: {
            description: "Browser profile-aware extension fixture",
            min_compozy_version: "0.0.0",
            name: extensionName,
            version: "1.0.0",
          },
          profiles: [
            {
              color: "#5fbf85",
              credentials: [{ provider: "openai", slot: "api_key" }],
              icon: "chart-line",
              name: "growth",
            },
          ],
          resources: {
            cmd_palette: {
              commands: [
                {
                  action: { app: "dashboard", kind: "navigate" },
                  icon: "chart-line",
                  id: "open-growth",
                  profile: "growth",
                  title: "Open Growth Dashboard",
                },
              ],
            },
            skills: [
              { path: "skills/shared" },
              { path: "skills/growth-only", profile: "growth" },
              { path: "skills/finance-only", profile: "finance" },
            ],
          },
        },
        null,
        2
      ),
      "utf8"
    );
    return rootDir;
  }

  async function fulfillGrowthProfileCredential(
    runtime: Parameters<typeof runBrowserRuntimeCLIJSON>[0]
  ): Promise<void> {
    await runtime.requestJSON("/api/vault/secrets", {
      body: JSON.stringify({
        kind: "api_key",
        ref: "vault:profiles/growth/providers/openai/api_key",
        secret_value: "browser-growth-secret",
      }),
      method: "PUT",
    });
  }
});

async function openAndFillCommandPalette(page: Page, query: string): Promise<Locator> {
  const palette = commandPalette(page);
  const trigger = page.getByRole("button", { name: "Command palette", exact: true });
  await expect(palette).toHaveCount(0);
  await trigger.click();
  await expect(palette).toBeVisible();
  const input = palette.getByRole("combobox");
  await input.fill(query);
  await expect(input).toHaveValue(query);
  return palette;
}
