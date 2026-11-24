// Spec: Remote Computers S8 replaces `/settings/gateway` with a browser redirect to
// `/settings/computers`. The legacy `web/src/systems/gateway` backend/system removal lands in S14.
import { appWindow } from "../fixtures/os-navigation";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted, ensureProjectWorkspace } from "../fixtures/workspace";

test("retired /settings/gateway route redirects to /settings/computers", async ({
  appPage,
  runtime,
}) => {
  await ensureProjectWorkspace(appPage, runtime);
  await completeOnboardingIfPrompted(appPage);

  await appPage.goto(runtime.url("/settings/gateway"), { waitUntil: "domcontentloaded" });
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/settings/computers");

  const settingsWin = appWindow(appPage, "settings");
  await expect(settingsWin).toBeVisible({ timeout: 20_000 });
  await expect(settingsWin.getByTestId("computers-this-computer")).toBeVisible({
    timeout: 20_000,
  });
});
