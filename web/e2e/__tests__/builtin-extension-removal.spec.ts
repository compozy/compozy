import { runBrowserRuntimeCLIJSON } from "../fixtures/scenario-contracts";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

interface RemovedBuiltinsPayload {
  removed: Array<{ name: string; removed_by: string }>;
}

interface ExtensionListEntry {
  name: string;
  source: string;
  enabled: boolean;
}

// E2E-004: the operator removes the bundled spec-cycle permanently in the web UI, finds it under
// "Removed built-ins", restores it, and sees it in the installed list as enabled.
test.describe("Built-in permanent removal in the web UI", () => {
  test("E2E-004: remove permanently, restore from Removed built-ins, and see it installed and enabled", async ({
    appPage,
    runtime,
  }) => {
    await completeOnboardingIfPrompted(appPage);

    await test.step("open the spec-cycle remove dialog from its installed detail", async () => {
      await appPage.goto(runtime.url("/marketplace/installed"), { waitUntil: "domcontentloaded" });
      await expect(appPage.getByTestId("marketplace-installed-page")).toBeVisible();
      await appPage.getByRole("link", { name: "spec-cycle" }).first().click();
      await appPage.getByRole("button", { name: "Actions for spec-cycle" }).click();
      await appPage.getByRole("menuitem", { name: "Remove…" }).click();
    });

    await test.step("choose permanent removal and confirm by typing the name", async () => {
      await appPage.getByTestId("remove-extension-choice-permanent").click();
      await appPage.getByLabel("Type to confirm").fill("spec-cycle");
      await appPage.getByTestId("remove-extension-confirm").click();
      await expect(appPage).toHaveURL(/\/marketplace\/installed/);
    });

    await test.step("the daemon lists the tombstone and the installed CLI view omits spec-cycle", async () => {
      const removed = await runBrowserRuntimeCLIJSON<RemovedBuiltinsPayload>(runtime, [
        "extension",
        "removed",
        "-o",
        "json",
      ]);
      expect(removed.removed.map(item => item.name)).toContain("spec-cycle");
      const installed = await runBrowserRuntimeCLIJSON<ExtensionListEntry[]>(runtime, [
        "extension",
        "list",
        "-o",
        "json",
      ]);
      expect(installed.some(item => item.name === "spec-cycle")).toBe(false);
    });

    await test.step("restore it from the Removed built-ins section", async () => {
      await expect(appPage.getByText("Removed built-ins")).toBeVisible();
      await appPage.getByRole("button", { name: "Restore spec-cycle" }).click();
      await expect(appPage.getByRole("button", { name: "Restore spec-cycle" })).toHaveCount(0);
    });

    await test.step("the installed list shows spec-cycle as bundled and enabled", async () => {
      await expect(
        appPage.getByTestId("marketplace-installed-page").getByText("spec-cycle").first()
      ).toBeVisible();
      const installed = await runBrowserRuntimeCLIJSON<ExtensionListEntry[]>(runtime, [
        "extension",
        "list",
        "-o",
        "json",
      ]);
      const restored = installed.find(item => item.name === "spec-cycle");
      expect(restored?.source).toBe("bundled");
      expect(restored?.enabled).toBe(true);
    });
  });
});
