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

  const currentPageIsReady = async (): Promise<boolean> => {
    try {
      if (new URL(page.url()).pathname !== pathname) return false;
      if (options.readyTestId) {
        await page.getByTestId(options.readyTestId).waitFor({
          state: "visible",
          timeout: 500,
        });
      }
      return true;
    } catch {
      return false;
    }
  };

  await expect
    .poll(
      async () => {
        // A restart can make goto time out after Chromium has already committed
        // and rendered the destination. Observe that state before issuing another
        // navigation, otherwise each poll can abort the successful in-flight load.
        if (await currentPageIsReady()) return pathname;
        try {
          const response = await page.goto(targetURL, {
            waitUntil: "domcontentloaded",
            timeout: 2_000,
          });
          if (response && !response.ok()) {
            return "";
          }
          return (await currentPageIsReady()) ? pathname : "";
        } catch {
          return (await currentPageIsReady()) ? pathname : "";
        }
      },
      {
        timeout,
        intervals: [250, 500, 1_000, 2_000],
      }
    )
    .toBe(pathname);
}
