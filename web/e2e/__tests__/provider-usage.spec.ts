// Provider usage in the OS shell (E2E-001..E2E-006): the tray meter,
// its popover, the Usage app, and the Provider usage settings section against
// a real daemon whose Codex usage comes from the fake `codex` app-server on
// PATH (internal/providerusage/testdata/codex: Session 23%, Weekly 72%, 120
// credits). Nothing in the browser is mocked.
import { execFile } from "node:child_process";
import { chmodSync, copyFileSync, mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Page } from "@playwright/test";

import { appWindow } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
const FAKE_CODEX = path.join(REPO_ROOT, "internal/providerusage/testdata/codex");

/** A PATH entry holding the fake `codex`, and a file-store Codex home it reads. */
function codexFixture(): { binDir: string; codexHome: string } {
  const root = mkdtempSync(path.join(os.tmpdir(), "compozy-e2e-provider-usage-"));
  const binDir = path.join(root, "bin");
  const codexHome = path.join(root, "codex-home");
  mkdirSync(binDir, { recursive: true });
  mkdirSync(codexHome, { recursive: true });
  copyFileSync(FAKE_CODEX, path.join(binDir, "codex"));
  chmodSync(path.join(binDir, "codex"), 0o755);
  writeFileSync(
    path.join(codexHome, "auth.json"),
    JSON.stringify({ tokens: { access_token: "e2e-codex-fixture", account_id: "acct_e2e" } })
  );
  return { binDir, codexHome };
}

const codex = codexFixture();

test.use({
  runtimeOptions: {
    env: {
      ...process.env,
      PATH: [codex.binDir, process.env.PATH ?? ""].join(path.delimiter),
      CODEX_HOME: codex.codexHome,
    },
  },
});

function usageControl(page: Page) {
  return page.locator('[data-slot="os-menubar-usage"] button');
}

function usagePopover(page: Page) {
  return page.getByTestId("os-usage-popover");
}

async function openShell(page: Page, runtime: BrowserRuntime) {
  await ensureProjectWorkspace(page, runtime);
  await expect(usageControl(page)).toBeVisible({ timeout: 30_000 });
}

async function runCLI(runtime: BrowserRuntime, args: string[]) {
  if (!runtime.paths) throw new Error(`CLI ${args.join(" ")} needs launch-mode runtime paths`);
  // An agent session's identity variables would make the CLI act as that agent.
  const ambient = Object.fromEntries(
    Object.entries(process.env).filter(([name]) => !/^COMPOZY_(SESSION|AGENT)/.test(name))
  );
  return await execFileAsync(runtime.paths.cliShim, args, {
    env: {
      ...ambient,
      COMPOZY_E2E_CLI_BIN: runtime.paths.cliShim,
      COMPOZY_HOME: runtime.paths.homeDir,
      HOME: runtime.paths.operatorHomeDir,
    },
  });
}

test("glances at Codex Weekly right before the bell (E2E-001)", async ({ appPage, runtime }) => {
  await openShell(appPage, runtime);

  const tray = appPage.locator('[data-slot="os-menubar-tray"] [data-slot^="os-menubar-"]');
  const slots = await tray.evaluateAll(nodes => nodes.map(node => node.getAttribute("data-slot")));
  // The pager and All desktops lead the tray; the meter sits right before the bell.
  expect(slots.indexOf("os-menubar-desktops")).toBeLessThan(slots.indexOf("os-menubar-usage"));
  expect(slots.indexOf("os-menubar-usage")).toBe(slots.indexOf("os-menubar-bell") - 1);

  const control = usageControl(appPage);
  await expect(control).toHaveAccessibleName(/^Codex · Weekly 72% used · /);
  await control.hover();
  await expect(appPage.getByText(/^Codex · Weekly 72% used/).last()).toBeVisible();
});

test("opens the popover, expands Codex, refreshes, and returns focus on Esc (E2E-002)", async ({
  appPage,
  runtime,
}) => {
  await openShell(appPage, runtime);
  const control = usageControl(appPage);
  await control.click();

  const popover = usagePopover(appPage);
  await expect(popover).toBeVisible();
  await expect(control).toHaveAttribute("aria-expanded", "true");
  const codexRow = popover.getByRole("button", { name: /^Codex/ });
  await codexRow.click();
  await expect(popover.getByRole("progressbar", { name: /Session/ })).toBeVisible();
  await expect(popover.getByRole("progressbar", { name: /Weekly/ })).toBeVisible();
  await expect(popover).toContainText("120 credits");

  const refresh = popover.getByRole("button", { name: /provider usage$/ });
  await refresh.click();
  await expect(popover.getByRole("button", { name: "Refresh provider usage" })).toBeEnabled();
  await expect(popover.getByRole("progressbar", { name: /Weekly/ })).toBeVisible();

  await appPage.keyboard.press("Escape");
  await expect(popover).toBeHidden();
  await expect(control).toBeFocused();
});

test("drops the control when provider usage is turned off (E2E-003)", async ({
  appPage,
  runtime,
}) => {
  await openShell(appPage, runtime);

  // The daemon applies the section live even with unrelated restart-required
  // edits (the onboarding provider save) already on disk.
  const set = await runCLI(runtime, [
    "config",
    "set",
    "provider_usage.enabled",
    "false",
    "-o",
    "json",
  ]);
  expect(JSON.parse(set.stdout)).toMatchObject({ lifecycle: "live", applied: true });
  await expect
    .poll(
      async () =>
        (await runtime.requestJSON<{ enabled: boolean }>("/api/provider-usage?profile=default"))
          .enabled,
      { timeout: 30_000 }
    )
    .toBe(false);

  await expect(appPage.locator('[data-slot="os-menubar-usage"]')).toHaveCount(0, {
    timeout: 30_000,
  });
});

test("opens the Usage app from the popover with Codex collapsed until expanded (E2E-004)", async ({
  appPage,
  runtime,
}) => {
  await openShell(appPage, runtime);
  await usageControl(appPage).click();
  await usagePopover(appPage).getByRole("button", { name: "Open Usage" }).click();

  await expect(usagePopover(appPage)).toBeHidden();
  const usageWindow = appWindow(appPage, "usage");
  await expect(usageWindow).toBeVisible();
  await expect(usageWindow.getByRole("region", { name: "Usage" })).toBeVisible();
  const codexRow = usageWindow.getByRole("button", { name: /^Codex/ });
  await expect(codexRow).toHaveAttribute("aria-expanded", "false");
  await expect(usageWindow).not.toContainText("120 credits");
  await codexRow.click();
  await expect(codexRow).toHaveAttribute("aria-expanded", "true");
  await expect(usageWindow).toContainText("120 credits");
});

test("turns Codex off from Settings and its row leaves the popover (E2E-006)", async ({
  appPage,
  runtime,
}) => {
  await openShell(appPage, runtime);
  await usageControl(appPage).click();
  await expect(usagePopover(appPage).getByRole("button", { name: /^Codex/ })).toBeVisible();
  await usagePopover(appPage).getByRole("button", { name: "Settings" }).click();

  const settings = appWindow(appPage, "settings");
  await expect(settings).toBeVisible();
  const codexSwitch = settings.getByTestId("settings-provider-usage-provider-codex");
  await expect(codexSwitch).toBeChecked();
  await codexSwitch.click();
  await settings.getByTestId("settings-page-provider-usage-save").click();
  await expect(settings.getByTestId("settings-page-provider-usage-save-message")).toContainText(
    /applied/i
  );

  // Every other builtin provider is still a target (unsupported or signed
  // out), so the control stays; Codex is gone from it.
  await usageControl(appPage).click();
  await expect(usagePopover(appPage)).toBeVisible();
  await expect(usagePopover(appPage).getByRole("button", { name: /^Codex/ })).toHaveCount(0, {
    timeout: 30_000,
  });
});

test("reads and refreshes usage through the real CLI (E2E-005)", async ({ appPage, runtime }) => {
  await openShell(appPage, runtime);
  // Refresh waits for the daemon-owned collection rather than startup jitter.
  const refreshed = await runCLI(runtime, ["provider", "usage", "refresh", "codex"]);
  expect(refreshed.stdout).toContain("Codex");
  const listed = await runCLI(runtime, ["provider", "usage", "list", "codex", "-o", "json"]);
  const envelope = JSON.parse(listed.stdout);
  expect(envelope.targets).toHaveLength(1);
  expect(envelope.targets[0].provider_id).toBe("codex");
  expect(envelope.targets[0].windows[1].used_percent).toBe(72);
  expect(envelope.tray).toBeNull();
  await expect(runCLI(runtime, ["provider", "usage", "list", "nope"])).rejects.toMatchObject({
    code: 1,
    stderr: expect.stringContaining("provider_not_found"),
  });
});
