import type { BrowserWindowConstructorOptions } from "electron";

const PRODUCT_TITLE_BAR_HEIGHT = 44;

const LINUX_OVERLAY_COLOR = "#00000000";
const LINUX_OVERLAY_SYMBOL_COLOR = "#ececef";

type ProductWindowChrome = Pick<
  BrowserWindowConstructorOptions,
  "titleBarOverlay" | "titleBarStyle"
>;

export function productWindowChrome(platform: NodeJS.Platform): ProductWindowChrome {
  if (platform === "darwin") {
    return {
      titleBarStyle: "hidden",
      titleBarOverlay: { height: PRODUCT_TITLE_BAR_HEIGHT },
    };
  }

  if (platform === "linux") {
    return {
      titleBarStyle: "hidden",
      titleBarOverlay: {
        color: LINUX_OVERLAY_COLOR,
        symbolColor: LINUX_OVERLAY_SYMBOL_COLOR,
        height: PRODUCT_TITLE_BAR_HEIGHT,
      },
    };
  }

  return {};
}
