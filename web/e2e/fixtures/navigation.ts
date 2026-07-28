import { expect, type Page } from "@playwright/test";

interface ReloadDaemonServedPageOptions {
  readyTestId?: string;
  timeout?: number;
}

export async function reloadDaemonServedPage(
  page: Page,
  runtime: { url(pathname?: string): string },
  pathname: string,
  options: ReloadDaemonServedPageOptions = {}
): Promise<void> {
  const targetURL = runtime.url(pathname);
  const timeout = options.timeout ?? 45_000;

  await expect
    .poll(
      async () => {
        try {
          const response = await page.goto(targetURL, {
            waitUntil: "domcontentloaded",
            timeout: 2_000,
          });
          if (response && !response.ok()) {
            return "";
          }
          if (new URL(page.url()).pathname !== pathname) {
            return "";
          }
          if (options.readyTestId) {
            await page.getByTestId(options.readyTestId).waitFor({
              state: "visible",
              timeout: 500,
            });
          }
          return pathname;
        } catch {
          return "";
        }
      },
      {
        timeout,
        intervals: [250, 500, 1_000, 2_000],
      }
    )
    .toBe(pathname);
}
