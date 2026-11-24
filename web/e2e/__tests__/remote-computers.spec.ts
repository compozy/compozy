import type { Locator, Page } from "@playwright/test";

import { appWindow, openAppWindow, windowID } from "../fixtures/os-navigation";
import { cliComputerStatus, expect, test } from "../fixtures/remote-computers";

async function openSettings(page: Page, section: "computers" | "account"): Promise<Locator> {
  await page.goto(`/settings/${section}`, { waitUntil: "domcontentloaded" });
  const settings = appWindow(page, "settings");
  await expect(settings).toBeVisible();
  return settings;
}

async function sessionSidebar(session: Locator): Promise<Locator> {
  const sidebar = session.getByTestId("session-sidebar");
  await expect(sidebar).toBeAttached();
  if ((await sidebar.getAttribute("data-state")) === "closed") {
    await session.getByRole("button", { name: "Open sessions sidebar" }).click();
  }
  await expect(sidebar).toHaveAttribute("data-state", "open");
  return sidebar;
}

test.describe("SSH", () => {
  test.use({ scenario: "ssh-add" });

  test("E2E-002 trust an SSH computer and run a session on it", async ({ page, lab }) => {
    const host = lab.computers[0];
    const settings = await openSettings(page, "computers");
    await settings.getByTestId("computers-add-button").click();
    const dialog = page.getByRole("dialog", { name: "Add computer" });
    await dialog.getByRole("radio", { name: /^SSH/ }).click();
    await dialog.locator("#add-computer-ssh-host").click();
    await page.getByRole("option", { name: new RegExp(host.name) }).click();
    await dialog.getByRole("button", { name: "Add computer", exact: true }).click();

    await expect(dialog.getByText("Trust this host?")).toBeVisible();
    await dialog.getByRole("button", { name: "Trust and continue" }).click();
    await dialog.getByRole("button", { name: `Open ${host.name}` }).click({ timeout: 45_000 });
    const computer = settings
      .getByTestId("computers-list")
      .locator("[data-computer-id]")
      .filter({ has: page.getByText(host.name, { exact: true }) })
      .filter({ hasText: "via SSH" });
    await expect(computer).toContainText("Connected");
    await settings.getByRole("button", { name: "Close window" }).click();

    await page.getByRole("menuitem", { name: "Session", exact: true }).click();
    await page.getByRole("menuitem", { name: /^New session/ }).click();
    const create = page.getByTestId("session-create-dialog");
    await create.getByTestId("session-create-run-on").click();
    await page.getByRole("option", { name: new RegExp(host.name) }).click();
    await expect(create.getByTestId("session-create-run-on")).toHaveAccessibleName(
      `Run on: ${host.name}`
    );
    await create.getByTestId("session-create-mode-advanced").click();
    await create.getByTestId("session-create-agent-select").click();
    await page.getByTestId("agent-command-item-ssh-fixture-agent").click();
    await create.getByTestId("session-create-submit").click();

    const session = appWindow(page, "session");
    const prompt = session.getByRole("textbox", { name: "Session prompt" });
    await prompt.fill("echo");
    await prompt.press("Enter");
    await expect(session.getByText("echo from testhost", { exact: true })).toBeVisible();
    const sidebar = await sessionSidebar(session);
    await expect(
      sidebar
        .locator('[data-slot="session-list-row"][data-computer-id]')
        .getByText(host.name, { exact: true })
    ).toBeVisible();
  });
});

test.describe("Account-linked computer", () => {
  test.use({ scenario: "account-add" });

  test("E2E-003 add an account computer and find its session in the sidebar and Search", async ({
    page,
    lab,
  }) => {
    const host = lab.computers[0];
    const { session_id: sessionID, session_title: title } = lab;
    if (!host.id || !sessionID || !title)
      throw new Error("The linked computer has no seeded session.");
    const settings = await openSettings(page, "computers");
    const onAccount = settings.getByTestId("computers-on-account");
    await expect(onAccount.getByText(host.name, { exact: true })).toBeVisible();
    await onAccount.getByRole("button", { name: "Add", exact: true }).click();
    await expect(settings.locator(`[data-computer-id="${host.id}"]`)).toContainText("Connected");
    await settings.getByRole("button", { name: "Close window" }).click();

    const sidebar = await sessionSidebar(await openAppWindow(page, "Session", "session"));
    const remoteRow = sidebar
      .locator(`[data-slot="session-list-row"][data-computer-id="${host.id}"]`)
      .filter({ hasText: title });
    await expect(remoteRow.getByText(title, { exact: true })).toBeVisible();
    await expect(remoteRow.getByText(host.name, { exact: true })).toBeVisible();

    await page.keyboard.press("ControlOrMeta+K");
    const palette = page.getByTestId("os-command-palette");
    await palette.getByPlaceholder("Search apps, sessions, and actions…").fill(title);
    const result = palette.getByTestId(`os-palette-session-${sessionID}`);
    await expect(result).toContainText(title);
    await expect(result).toContainText(host.name);
  });
});

test.describe("Device revocation", () => {
  test.use({ scenario: "revoke-device" });

  test("E2E-007 revoke the second CLI device and observe device_revoked within 3 s", async ({
    page,
    lab,
  }) => {
    const device = lab.cli_controller;
    if (!device) throw new Error("The scenario has no second CLI controller.");
    const host = lab.computers[0].name;
    expect((await cliComputerStatus(lab, host)).status).toBe("connected");

    const settings = await openSettings(page, "account");
    const row = settings
      .getByTestId("account-devices")
      .locator(`[data-device-id="${device.device_id}"]`);
    await expect(row.getByText("CLI", { exact: true })).toBeVisible();
    await row.getByRole("button", { name: "Revoke" }).click();
    const acknowledged = page.waitForResponse(
      response =>
        response.request().method() === "DELETE" &&
        new URL(response.url()).pathname === `/api/account/devices/${device.device_id}`
    );
    const confirm = page.getByRole("alertdialog").or(page.getByRole("dialog"));
    await confirm.getByRole("button", { name: "Revoke device" }).click();
    expect((await acknowledged).status()).toBe(204);
    const deadline = performance.now() + 3_000;

    await expect
      .poll(
        async () => {
          const status = await cliComputerStatus(lab, host);
          return performance.now() <= deadline ? (status.diagnosis?.code ?? status.code) : "late";
        },
        { timeout: 3_000, intervals: [50, 100, 200] }
      )
      .toBe("device_revoked");
    await expect(row).toBeHidden();
  });
});

test.describe("Remote session terminal and files", () => {
  test.use({ scenario: "remote-session" });

  test("E2E-004 open Terminal and Files on a remote session", async ({ page, lab }) => {
    const host = lab.computers[0];
    const {
      session_title: title,
      hostname,
      project_file: projectFile,
      project_file_content: projectFileContent,
    } = lab;
    if (!host.id || !title || !hostname || !projectFile || !projectFileContent) {
      throw new Error("The remote-session scenario did not seed required fixture fields.");
    }

    const session = await openAppWindow(page, "Session", "session");
    const sidebar = await sessionSidebar(session);
    const remoteRow = sidebar
      .locator(`[data-slot="session-list-row"][data-computer-id="${host.id}"]`)
      .filter({ hasText: title });
    await expect(remoteRow).toBeVisible({ timeout: 20_000 });
    await remoteRow.click();

    await expect(session.getByTestId(`session-computer-chip-${host.id}`)).toBeVisible();
    const sessionWindowId = await windowID(session);

    await session.getByTestId("session-remote-terminal-action").click();
    const terminal = appWindow(page, "terminal");
    await expect(terminal).toBeVisible();
    const log = terminal.locator('[role="log"]:visible').last();
    await expect(log).toBeVisible({ timeout: 20_000 });
    await expect(log).toHaveAttribute("data-readonly", "false", { timeout: 20_000 });
    await log.click();
    await page.keyboard.type("uname -n");
    await page.keyboard.press("Enter");
    await expect(log).toContainText(hostname, { timeout: 15_000 });

    await page
      .getByTestId("os-tab-bar")
      .locator(`[role="tab"][data-lead-window-id="${sessionWindowId}"]`)
      .click();
    await expect(session).toBeVisible();

    await session.getByTestId("session-remote-files-action").click();
    const files = appWindow(page, "files");
    await expect(files).toBeVisible();
    const tree = files.getByTestId("files-tree");
    await expect(tree).toBeVisible({ timeout: 20_000 });
    await tree.locator(`[data-type="item"][data-item-path="${projectFile}"]`).click();
    await expect(files.getByTestId("file-viewer-header")).toContainText(projectFile);
    await expect(files.getByTestId("file-viewer")).toContainText(projectFileContent, {
      timeout: 15_000,
    });
  });
});
