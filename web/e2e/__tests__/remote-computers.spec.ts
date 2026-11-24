import { chromium, type BrowserContext, type Locator, type Page } from "@playwright/test";

import { appWindow, openCommandPalette, windowID } from "../fixtures/os-navigation";
import {
  cliComputerStatus,
  cliOnboardingComplete,
  cliSessionPrompt,
  expect,
  test,
} from "../fixtures/remote-computers";

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

async function installPurchaseTurnstile(context: BrowserContext) {
  await context.route("https://challenges.cloudflare.com/turnstile/v0/api.js**", route =>
    route.fulfill({
      contentType: "text/javascript",
      body: `let verification;
        window.turnstile = {
          ready: (ready) => ready(),
          render: (_, options) => { verification = options; return "fixture"; },
          reset: () => {},
          execute: () => verification.callback("pass-" + verification.action)
        };`,
    })
  );
}

test.describe("Purchase from the cloud", () => {
  test.use({ scenario: "browser-purchase" });

  test("E2E-013 and IT-109 buy annual Pro before installing, then use the same account in CompozyOS", async ({
    page,
    context,
    browser,
    lab,
  }) => {
    // Safety Invariant 34. An account in a blocked state receives no Checkout Session.
    await installPurchaseTurnstile(context);
    const upgrade = lab.cloud_url + "/billing/upgrade?interval=year";
    await page.goto(upgrade);
    await expect(page.getByRole("button", { name: "Continue to checkout" })).toBeDisabled();
    await page.getByRole("button", { name: "Continue with Google" }).click();
    await expect(page.getByText(lab.email)).toBeVisible();
    expect(
      (await context.cookies(lab.cloud_url)).some(cookie => cookie.name === "__Host-compozy_ws")
    ).toBe(true);
    await expect(page.getByRole("radio", { name: /Pro annual/ })).toBeChecked();
    await page.getByRole("button", { name: "Continue to checkout" }).click();
    await page.getByRole("button", { name: "Complete payment" }).click();
    await expect(page.getByRole("heading")).toHaveText(
      "You're on Pro. Open CompozyOS to use your computers."
    );
    await expect(page.getByRole("link", { name: "Open CompozyOS", exact: true })).toHaveAttribute(
      "href",
      "compozyos://open/"
    );

    const evidence = await context.request.get(lab.cloud_url + "/fixture/billing");
    expect(evidence.ok()).toBe(true);
    expect(await evidence.json()).toMatchObject({
      plan: "pro",
      devices: 0,
      installations: 0,
      checkouts: 1,
      price: "price_pro_annual",
      promotion_codes: true,
      owned_checkout: true,
    });
    await page.goto(upgrade);
    await expect(page.getByRole("heading")).toHaveText("You're already on Pro.");
    await expect(page.getByRole("button", { name: "Manage billing" })).toBeVisible();
    if (!lab.hosted_url) throw new Error("Missing foreign computer in purchase fixture");
    await page.goto(upgrade + "&hostname=" + new URL(lab.hosted_url).hostname);
    await expect(page.getByRole("heading")).toHaveText("This computer belongs to another account.");

    const suspended = await browser.newContext();
    try {
      await installPurchaseTurnstile(suspended);
      const blocked = await suspended.newPage();
      await blocked.goto(upgrade);
      await blocked.getByRole("button", { name: "Continue with GitHub" }).click();
      await expect(blocked.locator("main")).toHaveAttribute("data-error-code", "account_suspended");
      await expect(blocked.getByRole("button", { name: "Continue to checkout" })).toHaveCount(0);
    } finally {
      await suspended.close();
    }

    await page.goto(lab.controller.web_url + "/settings/account");
    const login = page.waitForResponse(
      response =>
        new URL(response.url()).pathname === "/api/account/login" &&
        response.request().method() === "POST"
    );
    await page.getByRole("button", { name: "Continue with Google" }).click();
    const { verification_uri_complete: activationURL } = (await (await login).json()) as {
      verification_uri_complete: string;
    };
    const activation = await context.newPage();
    await activation.goto(activationURL);
    await activation.getByRole("button", { name: "Continue with Google" }).click();
    await expect(activation.getByRole("heading")).toHaveText(
      "You're signed in. Return to CompozyOS."
    );
    await activation.close();
    await expect
      .poll(async () => {
        const status = await context.request.get(lab.controller.web_url + "/api/account");
        expect(status.ok()).toBe(true);
        return (await status.json()) as { email?: string; plan?: string };
      })
      .toMatchObject({ email: lab.email, plan: "pro" });
    const settings = await openSettings(page, "account");
    await expect(settings.getByText("Pro", { exact: true }).first()).toBeVisible();
    const current = await context.request.get(lab.cloud_url + "/fixture/billing");
    expect(await current.json()).toMatchObject({
      plan: "pro",
      checkouts: 1,
      devices: 1,
      installations: 1,
    });
  });
});

test.describe("Purchase from a lapsed hosted route", () => {
  test.use({ scenario: "hosted-purchase", viewport: { width: 390, height: 844 } });

  test("E2E-012 return to the original hostname after buying Pro as its owner", async ({
    page,
    context,
    browser,
    lab,
  }) => {
    // IT-074 browser assertion. The real retained hosted listener serves the plan-required SPA state.
    const hostedURL = lab.hosted_url;
    const proxy = lab.hosted_proxy;
    if (!hostedURL || !proxy) throw new Error("Missing retained hostname in purchase fixture");
    const hostname = new URL(hostedURL).hostname;
    await installPurchaseTurnstile(context);
    await context.route(hostedURL + "/**", async route => {
      const target = new URL(route.request().url());
      const response = await route.fetch({
        url: proxy + target.pathname + target.search,
        maxRedirects: 0,
      });
      await route.fulfill({ response });
    });
    await context.route("https://cloud.compozy.com/billing/upgrade**", route => {
      const target = new URL(route.request().url());
      return route.fulfill({
        status: 302,
        headers: { location: lab.cloud_url + target.pathname + target.search },
      });
    });
    await page.goto(hostedURL);
    await expect(page.getByText("This computer's hosted route needs Pro")).toBeVisible();
    const launched = context.waitForEvent("page", { timeout: 2_000 }).catch(() => null);
    await page.getByRole("button", { name: "Upgrade to Pro" }).click();
    const purchase = (await launched) ?? page;
    await expect(purchase).toHaveURL(new RegExp("hostname=" + hostname.replaceAll(".", "\\.")));
    await purchase.getByRole("button", { name: "Continue with Google" }).click();
    await expect(purchase.getByText(lab.email)).toBeVisible();
    await purchase.getByRole("button", { name: "Continue to checkout" }).click();
    await purchase.getByRole("button", { name: "Complete payment" }).click();
    await expect(purchase).toHaveURL(hostedURL + "/");
    const evidence = await context.request.get(lab.cloud_url + "/fixture/billing");
    expect(await evidence.json()).toMatchObject({
      plan: "pro",
      checkouts: 1,
      owned_checkout: true,
    });

    const other = await browser.newContext();
    try {
      await installPurchaseTurnstile(other);
      const denied = await other.newPage();
      await denied.goto(lab.cloud_url + "/billing/upgrade?hostname=" + hostname);
      await denied.getByRole("button", { name: "Continue with GitHub" }).click();
      await expect(denied.getByRole("heading")).toHaveText(
        "This computer belongs to another account."
      );
    } finally {
      await other.close();
    }
  });
});

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

    await page.goto("/sessions", { waitUntil: "domcontentloaded" });
    const sidebar = await sessionSidebar(appWindow(page, "session"));
    const remoteRow = sidebar
      .locator(`[data-slot="session-list-row"][data-computer-id="${host.id}"]`)
      .filter({ hasText: title });
    await expect(remoteRow.getByText(title, { exact: true })).toBeVisible();
    await expect(remoteRow.getByText(host.name, { exact: true })).toBeVisible();

    const palette = await openCommandPalette(page);
    await palette.getByPlaceholder("Search apps, sessions, and actions…").fill(title);
    const result = palette.getByTestId(`os-palette-session-${sessionID}`);
    await expect(result).toBeVisible();
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

    await page.goto("/sessions", { waitUntil: "domcontentloaded" });
    const session = appWindow(page, "session");
    await expect(session).toBeVisible();
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

test.describe("Hosted browser", () => {
  test.use({ scenario: "hosted-browser" });

  test("E2E-006 open a computer from a hosted browser, stream a live turn, and revoke within 3 s", async ({
    page,
    lab,
  }) => {
    const { session_id: sessionID, session_title: title, edge_port: edgePort } = lab;
    if (!sessionID || !title || !edgePort) {
      throw new Error("The hosted-browser scenario requires a seeded session and TLS edge port.");
    }

    // Context 1 (desktop host): turn on Hosted route and wait until ready.
    const settings = await openSettings(page, "computers");
    const thisComputer = settings.getByTestId("computers-this-computer");
    await thisComputer.getByRole("switch", { name: "Hosted route" }).click();
    await expect(thisComputer.getByText("Hosted route ready", { exact: true })).toBeVisible({
      timeout: 30_000,
    });
    const hostLabel = await thisComputer
      .locator("span.font-mono")
      .filter({ hasText: /\.compozy\.run$/ })
      .first()
      .textContent();
    const hostname = hostLabel?.trim() ?? "";
    expect(hostname).toMatch(/^[a-z0-9-]+\.compozy\.run$/);

    // Context 2 (remote browser): resolve cloud.compozy.com and *.compozy.run to the fixture edge.
    const hostedBrowser = await chromium.launch({
      args: [
        `--host-resolver-rules=MAP cloud.compozy.com 127.0.0.1:${edgePort}, MAP *.compozy.run 127.0.0.1:${edgePort}, MAP * ~NOTFOUND`,
        "--ignore-certificate-errors",
        "--no-proxy-server",
      ],
    });
    try {
      const hostedContext = await hostedBrowser.newContext({
        ignoreHTTPSErrors: true,
        viewport: { width: 1280, height: 800 },
      });
      const hostedPage = await hostedContext.newPage();

      await hostedPage.goto(`https://${hostname}/`, { waitUntil: "domcontentloaded" });
      await expect(hostedPage.getByRole("heading", { name: "Sign in to CompozyOS" })).toBeVisible();
      await hostedPage.getByRole("button", { name: /Continue with Google/i }).click();

      await expect(hostedPage.getByTestId("root-shell")).toBeVisible({ timeout: 30_000 });
      await hostedPage.goto(`https://${hostname}/sessions`, { waitUntil: "domcontentloaded" });
      const hostedSession = appWindow(hostedPage, "session");
      const sidebar = await sessionSidebar(hostedSession);
      await sidebar
        .locator('[data-slot="session-list-row"]')
        .filter({ hasText: title })
        .first()
        .click();

      // Prompt the session via the host CLI and assert the live transcript line streams in.
      await cliSessionPrompt(lab, sessionID, "echo");
      await expect(hostedSession.getByText("echo from testhost", { exact: true })).toBeVisible({
        timeout: 15_000,
      });

      // Verify the installable PWA manifest on the hosted origin.
      const manifestResponse = await hostedContext.request.get(
        `https://${hostname}/manifest.webmanifest`
      );
      expect(manifestResponse.status()).toBe(200);
      const manifest = (await manifestResponse.json()) as { display?: string };
      expect(manifest.display).toBe("standalone");

      // Context 1: revoke the browser device in Settings -> Account -> Devices.
      const accountSettings = await openSettings(page, "account");
      const browserRow = accountSettings
        .getByTestId("account-devices")
        .locator('[data-device-id]:not([data-this-device="true"])')
        .filter({ hasText: "Browser" })
        .first();
      await expect(browserRow).toBeVisible({ timeout: 15_000 });
      const browserDeviceId = await browserRow.getAttribute("data-device-id");
      if (!browserDeviceId) throw new Error("Browser device row did not expose data-device-id.");

      await browserRow.getByRole("button", { name: "Revoke" }).click();
      const revoked = page.waitForResponse(
        response =>
          response.request().method() === "DELETE" &&
          new URL(response.url()).pathname === `/api/account/devices/${browserDeviceId}`
      );
      const confirm = page.getByRole("alertdialog").or(page.getByRole("dialog"));
      await confirm.getByRole("button", { name: "Revoke device" }).click();
      expect((await revoked).status()).toBe(204);

      // Context 2: transitions to device_removed within 3 s.
      await expect(
        hostedPage.getByRole("heading", { name: "This device was removed from your account" })
      ).toBeVisible({ timeout: 3_000 });
    } finally {
      await hostedBrowser.close();
    }
  });
});

test.describe("Free limit upgrade", () => {
  test.use({ scenario: "free-upgrade" });

  test("E2E-008 stop at Checking your plan on Free, upgrade via Stripe Checkout, and proceed with the SSH add", async ({
    page,
    context,
    lab,
  }) => {
    const host = lab.computers[0];
    const settings = await openSettings(page, "computers");
    await settings.getByTestId("computers-add-button").click();

    const dialog = page.getByRole("dialog", { name: "Add computer" });
    await dialog.getByRole("radio", { name: /^SSH/ }).click();
    await dialog.locator("#add-computer-ssh-host").click();
    await page.getByRole("option", { name: new RegExp(host.name) }).click();
    await dialog.getByRole("button", { name: "Add computer", exact: true }).click();

    // Stops at Checking your plan before touching the second host (US-003 AC-1 / EC-3).
    await expect(dialog.getByText("Checking your plan")).toBeVisible();
    await expect(dialog.getByText(/Free includes 1 remote computer/i).first()).toBeVisible();

    // Open the U9 Upgrade sheet with annual preselected (US-002 AC-1).
    await dialog.getByRole("button", { name: /Upgrade/i }).click();
    const sheet = page.getByTestId("account-upgrade-sheet");
    await expect(sheet).toBeVisible();
    await expect(sheet.getByRole("radio", { name: /Pro annual/ })).toBeChecked();
    await expect(
      sheet.getByText("Prices in USD. Plus applicable taxes, calculated at checkout.")
    ).toBeVisible();

    // Continue to checkout opens fake Stripe Checkout in a new browser tab.
    const checkoutTabPromise = context.waitForEvent("page");
    await sheet.getByRole("button", { name: "Continue to checkout" }).click();
    await expect(sheet).toHaveAttribute("data-phase", "waiting");
    await expect(sheet.getByText("Waiting for checkout…")).toBeVisible();

    const checkoutTab = await checkoutTabPromise;
    await expect(checkoutTab).toHaveURL(/\/fixture-checkout\//);
    await checkoutTab.getByRole("button", { name: "Complete payment" }).click();

    // Account stream completes checkout with remote access off (US-002 AC-2 / AC-5).
    await expect(sheet).toHaveAttribute("data-phase", "done", { timeout: 10_000 });
    await expect(sheet.getByText("You're on Pro").first()).toBeVisible();
    if (!checkoutTab.isClosed()) {
      await checkoutTab.close();
    }
    await sheet.getByRole("button", { name: "Done", exact: true }).click();
    await expect(sheet).toBeHidden();

    // Add computer retries past Checking your plan and completes SSH setup.
    await expect(dialog.getByText("Trust this host?")).toBeVisible({ timeout: 20_000 });
    await dialog.getByRole("button", { name: "Trust and continue" }).click();
    await dialog.getByRole("button", { name: `Open ${host.name}` }).click({ timeout: 45_000 });
    const computer = settings
      .getByTestId("computers-list")
      .locator("[data-computer-id]")
      .filter({ has: page.getByText(host.name, { exact: true }) })
      .filter({ hasText: "via SSH" });
    await expect(computer).toContainText("Connected");

    // Settings → Account shows Pro and Manage billing without a reload.
    const accountSettings = await openSettings(page, "account");
    await expect(
      accountSettings.getByTestId("account-identity").getByText("Pro", { exact: true })
    ).toBeVisible();
    await expect(accountSettings.getByRole("button", { name: "Manage billing" })).toBeVisible();
  });
});

test.describe("Mandatory sign-in first run", () => {
  test.use({ scenario: "sign-in-first-run" });

  test("E2E-010 fresh home shows U0, completes Google sign-in with marketing consent, and opens the app shell", async ({
    page,
    context,
    lab,
  }) => {
    await installPurchaseTurnstile(context);
    await page.goto("/", { waitUntil: "domcontentloaded" });
    await expect(page).toHaveURL(/\/sign-in/);
    await expect(page.getByRole("heading", { name: "Sign in to CompozyOS" })).toBeVisible();
    await expect(page.getByTestId("os-desktop")).toHaveCount(0);

    const consent = page.getByRole("checkbox", { name: "Email me product news and offers" });
    await consent.click();
    await expect(consent).toBeChecked();

    const loginPromise = page.waitForResponse(
      response =>
        new URL(response.url()).pathname === "/api/account/login" &&
        response.request().method() === "POST"
    );
    const activationTabPromise = context.waitForEvent("page");
    await page.getByRole("button", { name: "Continue with Google" }).click();

    const loginResponse = await loginPromise;
    expect(loginResponse.ok()).toBe(true);
    const { user_code: userCode } = (await loginResponse.json()) as { user_code: string };
    expect(userCode).toBeTruthy();

    await expect(page.locator('[data-slot="sign-in-verification-code"]')).toContainText(userCode);

    const activationTab = await activationTabPromise;
    await expect(activationTab.getByText(userCode)).toBeVisible({ timeout: 15_000 });
    await activationTab.getByRole("button", { name: "Continue with Google" }).click();
    await expect(activationTab.getByRole("heading")).toHaveText(
      "You're signed in. Return to CompozyOS."
    );
    await activationTab.close();

    await expect(page).not.toHaveURL(/\/sign-in/, { timeout: 20_000 });
    await expect(page.getByTestId("os-desktop")).toBeVisible({ timeout: 20_000 });

    await cliOnboardingComplete(lab);
    const settings = await openSettings(page, "account");
    await expect(
      settings.getByTestId("account-email").getByRole("switch", { name: "Product news and offers" })
    ).toBeChecked({ timeout: 15_000 });
  });
});

test.describe("Signed-in account and sign out", () => {
  test.use({ scenario: "sign-in-sign-out" });

  test("E2E-001 Settings → Account shows identity and Free tier, and signing out returns to U0", async ({
    page,
  }) => {
    const settings = await openSettings(page, "account");
    const identity = settings.getByTestId("account-identity");
    await expect(identity.getByText("Pedro", { exact: true })).toBeVisible();
    await expect(identity.getByText("Free", { exact: true })).toBeVisible();
    await expect(identity.getByText("pedro@example.com · Signed in with Google")).toBeVisible();

    await identity.getByRole("button", { name: "Sign out" }).click();
    const confirm = page.getByRole("alertdialog").or(page.getByRole("dialog"));
    await expect(confirm.getByText("Sign out of CompozyOS?")).toBeVisible();
    await confirm.getByRole("button", { name: "Sign out", exact: true }).click();

    await expect(page).toHaveURL(/\/sign-in/, { timeout: 15_000 });
    await expect(page.getByRole("heading", { name: "Sign in to CompozyOS" })).toBeVisible();
    await expect(page.getByTestId("os-desktop")).toHaveCount(0);
  });
});

test.describe("Account deletion on Pro", () => {
  test.use({ scenario: "account-delete-pro" });

  test("E2E-014 delete Pro account inside the 14-day refund window, return to U0, and confirm Stripe cancellation and refund", async ({
    page,
    context,
    lab,
  }) => {
    const settings = await openSettings(page, "account");
    await settings
      .getByTestId("account-delete")
      .getByRole("button", { name: "Delete account" })
      .click();

    const dialog = page.getByRole("alertdialog").or(page.getByRole("dialog"));
    await expect(dialog.getByText("Deleting your account also cancels Pro.")).toBeVisible();
    await expect(dialog.getByText(/is refunded in full\./)).toBeVisible();

    await dialog.getByRole("textbox", { name: "Confirm your email" }).fill(lab.email);
    await dialog.getByRole("button", { name: "Delete account and cancel Pro" }).click();

    await expect(page).toHaveURL(/\/sign-in/, { timeout: 15_000 });
    await expect(page.getByRole("heading", { name: "Sign in to CompozyOS" })).toBeVisible();

    await expect
      .poll(
        async () => {
          const response = await context.request.get(lab.cloud_url + "/fixture/deletion");
          expect(response.ok()).toBe(true);
          return (await response.json()) as { deletes: string[]; refunds: number };
        },
        { timeout: 15_000 }
      )
      .toMatchObject({
        deletes: ["sub_e2e_delete_pro"],
        refunds: 1,
      });
  });
});

test.describe("Remote Gateway upgrade", () => {
  test.use({ scenario: "gateway-upgrade" });

  test("E2E-009 shows the upgrade notice after sign-in and remembers Dismiss", async ({ page }) => {
    await page.goto("/settings/gateway", { waitUntil: "domcontentloaded" });
    await expect(page).toHaveURL(/\/settings\/computers/);
    const settings = appWindow(page, "settings");
    const notice = settings
      .getByRole("alert")
      .filter({ hasText: "Remote access was replaced by Computers" });
    await expect(notice).toBeVisible();
    await expect(notice).toContainText(
      "Paired devices and connection profiles were removed. Set up remote computers again."
    );
    await expect(notice).toContainText("2 webhook triggers need confirmation");
    await expect(notice.getByRole("link", { name: "Learn more" })).toHaveAttribute(
      "href",
      "https://compozy.com/blog/compozy-0-8-remote-computers"
    );
    await expect(notice.getByRole("link", { name: "Review" })).toBeVisible();

    const dismissed = page.waitForResponse(
      response =>
        new URL(response.url()).pathname === "/api/migration-notices/gateway_removed/dismiss" &&
        response.request().method() === "POST"
    );
    await notice.getByRole("button", { name: "Dismiss" }).click();
    expect((await dismissed).status()).toBe(204);
    await expect(notice).toHaveCount(0);

    const reread = page.waitForResponse(
      response =>
        new URL(response.url()).pathname === "/api/migration-notices" &&
        response.request().method() === "GET"
    );
    await page.reload({ waitUntil: "domcontentloaded" });
    const response = await reread;
    expect(response.status()).toBe(200);
    expect(await response.json()).toEqual({ notices: [] });
    await expect(settings).toBeVisible();
    await expect(notice).toHaveCount(0);
  });
});
