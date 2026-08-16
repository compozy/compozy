import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

/**
 * Browser journeys for the attention program (E2E-009..E2E-015).
 *
 * These walk the operator-facing half of attention against the daemon-served
 * SPA: the bell's two sections and its cross-workspace jump, notification
 * delivery and its suppression rules, the tab-title count, the Settings
 * round-trip, the tri-state scope, and the sidebar's badge vocabulary.
 *
 * E2E-020 (system notifications) is deliberately absent: it belongs to the
 * desktop lane, which this task does not ship.
 */

const bell = {
  trigger: '[data-slot="os-menubar-bell"]',
  popover: "os-bell-popover",
  needsYou: "os-bell-needs-you",
  finished: "os-bell-finished",
  empty: "os-bell-empty",
  disconnected: "os-bell-disconnected",
} as const;

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await completeOnboardingIfPrompted(page);
});

test.describe("E2E-009 attention bell", () => {
  test("Should separate Needs you from Finished and count only needs-you", async ({ page }) => {
    await page.locator(bell.trigger).click();
    const popover = page.getByTestId(bell.popover);
    await expect(popover).toBeVisible();

    const needsYou = popover.getByTestId(bell.needsYou);
    const finished = popover.getByTestId(bell.finished);
    await expect(needsYou).toBeVisible();

    // Finished-unseen work is listed but never inflates the badge: the badge is
    // "how many are blocked on me", and a finished session blocks nobody.
    const needsYouRows = await needsYou.getByRole("button").count();
    const badge = page.locator(`${bell.trigger} [class*="rounded-full"]`).first();
    await expect(badge).toHaveText(String(needsYouRows));
    if (await finished.isVisible()) {
      await expect(finished.getByRole("button").first()).toBeVisible();
    }
  });

  test("Should offer no manual dismiss anywhere in the bell", async ({ page }) => {
    // Viewing a session is the only thing that clears its marker; a dismiss
    // control would promise something the runtime cannot honour.
    await page.locator(bell.trigger).click();
    const popover = page.getByTestId(bell.popover);
    await expect(popover.getByRole("button", { name: /mark .*seen/i })).toHaveCount(0);
    await expect(popover.getByRole("button", { name: /^dismiss/i })).toHaveCount(0);
  });

  test("Should land on the session a row names, switching workspace when needed", async ({
    page,
  }) => {
    await page.locator(bell.trigger).click();
    const row = page
      .getByTestId(bell.popover)
      .getByTestId(/^os-attention-session-/)
      .first();
    const sessionId = (await row.getAttribute("data-testid"))?.replace("os-attention-session-", "");
    await row.click();

    await expect(page.getByTestId(bell.popover)).toBeHidden();
    await expect(page.getByTestId(`os-window-session:${sessionId}`)).toBeVisible();
  });

  test("Should render the quiet state rather than a blank popover at zero", async ({ page }) => {
    await page.route("**/api/sessions?*", async route => {
      await route.fulfill({
        json: { sessions: [], page: { total: 0, has_more: false, next_cursor: null } },
      });
    });
    await page.route("**/api/sessions/attention-summary", async route => {
      await route.fulfill({ json: { needs_you: 0, finished: 0, by_workspace: [] } });
    });
    await page.reload();
    await completeOnboardingIfPrompted(page);

    await page.locator(bell.trigger).click();
    await expect(page.getByTestId(bell.empty)).toContainText("All quiet");
  });

  test("Should state that a disconnected source is frozen and uncounted", async ({ page }) => {
    await page.route("**/api/sessions/catalog-stream", route => route.abort());
    await page.reload();
    await completeOnboardingIfPrompted(page);

    await page.locator(bell.trigger).click();
    await expect(page.getByTestId(bell.disconnected)).toContainText("frozen");
  });
});

test.describe("E2E-010 needs-you toasts", () => {
  test("Should toast an unfocused session and land on it when activated", async ({ page }) => {
    const toast = page.getByTestId("os-attention-toast-needs-you").first();
    await expect(toast).toBeVisible();
    await toast.click();
    await expect(page.getByTestId(/^os-window-session:/).first()).toBeVisible();
  });

  test("Should stay silent for the session already on screen", async ({ page }) => {
    // Focus suppression: a toast for the window the operator is looking at
    // reports what is already in front of them.
    const sessionWindow = page.getByTestId(/^os-window-session:/).first();
    await sessionWindow.click();
    await expect(page.getByTestId("os-attention-toast-needs-you")).toHaveCount(0);
  });
});

test.describe("E2E-011 coalesced completions", () => {
  test("Should group near-simultaneous completions into one toast opening the bell", async ({
    page,
  }) => {
    const grouped = page.getByTestId("os-attention-toast-finished").first();
    await expect(grouped).toContainText(/sessions finished/);
    await grouped.getByRole("button", { name: "Review finished" }).click();
    await expect(page.getByTestId(bell.finished)).toBeVisible();
  });
});

test.describe("E2E-012 tab title count", () => {
  test("Should carry the cross-workspace needs-you total and clear at zero", async ({ page }) => {
    await expect(page).toHaveTitle(/^\(\d+\) /);

    await page.route("**/api/sessions/attention-summary", async route => {
      await route.fulfill({ json: { needs_you: 0, finished: 0, by_workspace: [] } });
    });
    await expect(page).not.toHaveTitle(/^\(/);
  });

  test("Should survive route navigation", async ({ page }) => {
    const before = await page.title();
    await page.goto("/settings/attention");
    await expect(page).toHaveTitle(before);
  });
});

test.describe("E2E-013 Settings → Attention", () => {
  test("Should round-trip the delivery toggles without a save bar", async ({ page }) => {
    await page.goto("/settings/attention");
    const sound = page.getByTestId("settings-attention-sound");
    await expect(sound).toBeVisible();
    // Everything applies live: a save bar would be a second source of truth for
    // a value the CLI can change underneath it.
    await expect(page.getByTestId("settings-page-attention-save-bar")).toHaveCount(0);

    await sound.click();
    await page.reload();
    await expect(page.getByTestId("settings-attention-sound")).toHaveAttribute(
      "aria-checked",
      "false"
    );
  });

  test("Should render the system channel's real platform state", async ({ page }) => {
    await page.goto("/settings/attention");
    // Chromium under Playwright has no granted notification permission, so the
    // channel must not claim to be armed.
    await expect(
      page.getByTestId(/^settings-attention-system-(default|denied|unsupported)$/)
    ).toBeVisible();
    await expect(page.getByTestId("settings-attention-system")).toHaveAttribute(
      "aria-checked",
      "false"
    );
  });

  test("Should keep a muted workspace's bell rows while silencing delivery", async ({ page }) => {
    await page.goto("/settings/attention");
    await page.getByTestId("settings-attention-mute-picker").selectOption({ index: 1 });

    await page.locator(bell.trigger).click();
    await expect(
      page.getByTestId(bell.popover).locator("[data-muted='true']").first()
    ).toBeVisible();
  });
});

test.describe("E2E-014 tri-state Show all", () => {
  test("Should widen to every workspace, grouped and labelled", async ({ page }) => {
    await page.keyboard.press("Meta+KeyK");
    await page.getByTestId("os-sessions-modal-scope-all-workspaces").click();
    await expect(page.getByTestId(/^os-sessions-modal-workspace-/).first()).toBeVisible();
  });

  test("Should isolate a failing workspace instead of blanking the list", async ({ page }) => {
    let failed = false;
    await page.route("**/api/sessions?*workspace=*", async route => {
      if (failed) return route.continue();
      failed = true;
      return route.fulfill({ status: 500, json: { error: "workspace unreachable" } });
    });

    await page.getByTestId("os-sessions-modal-scope-all-workspaces").click();
    await expect(page.getByText("Couldn’t load sessions")).toBeVisible();
    await expect(page.getByTestId(/^os-sessions-modal-workspace-.*-retry$/).first()).toBeVisible();
  });

  test("Should persist the scope across a reload", async ({ page }) => {
    await page.getByTestId("os-sessions-modal-scope-all").click();
    await page.reload();
    await completeOnboardingIfPrompted(page);
    await expect(page.getByTestId("os-sessions-modal-scope-all")).toHaveAttribute(
      "data-active",
      "true"
    );
  });
});

test.describe("E2E-015 sidebar badges and sort", () => {
  test("Should render each badge with a distinct glyph and an accessible label", async ({
    page,
  }) => {
    // No colour-only assertions: the accessible label is the contract.
    const rows = page.getByTestId(/^os-sessions-modal-session-/);
    const marks = rows.first().getByRole("img");
    await expect(marks.first()).toHaveAttribute("aria-label", /^Session badge: /);
  });

  test("Should float needs-you sessions when attention-first is chosen", async ({ page }) => {
    await page.getByTestId("os-sessions-modal-sort-trigger").click();
    await page.getByTestId("os-sessions-modal-sort-attention").click();

    const first = page.getByTestId(/^os-sessions-modal-session-/).first();
    await expect(first.getByRole("img")).toHaveAttribute(
      "aria-label",
      /waiting-for-input|waiting-for-auth|failed/
    );
  });

  test("Should render an unreporting session honestly as unknown", async ({ page }) => {
    const unknown = page.locator('[data-badge="unknown"]').first();
    if ((await unknown.count()) > 0) {
      await expect(unknown).toHaveAttribute("aria-label", "Session badge: unknown");
    }
  });
});
