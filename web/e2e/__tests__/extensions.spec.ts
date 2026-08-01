import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { appWindow } from "../fixtures/os-navigation";
import { marketplaceOperatorSelectors } from "../fixtures/selectors";
import type { BrowserRuntime, RuntimePaths } from "../fixtures/runtime";
import { runBrowserRuntimeCLIJSON } from "../fixtures/scenario-contracts";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

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
    const paths = requireLaunchPaths(runtime);
    const workspace = await runtime.resolveWorkspace(paths.homeDir);
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

    await useGlobalWorkspaceIfPrompted(appPage);
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
    await expect(marketplace.extensionUpdateAction).toBeHidden();

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
