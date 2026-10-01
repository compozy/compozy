import type { BrowserWindowConstructorOptions, TitleBarOverlay } from "electron";

/** The web topbar height (`--height-menubar`); native controls share its band. */
const PRODUCT_TITLE_BAR_HEIGHT = 52;

/**
 * macOS traffic lights: a ~54×14 group centred inside the topbar's 84px leading
 * reserve (`--width-traffic-lights`) and vertically centred in the 52px bar.
 */
const MAC_TRAFFIC_LIGHT_POSITION = { x: 15, y: 19 } as const;

/**
 * Linux window controls paint on the topbar's rail surface with the theme's
 * foreground symbols (`--color-rail` / `--color-fg` per resolved theme).
 */
const LINUX_OVERLAY_COLORS = {
  dark: { color: "#0a0a0a", symbolColor: "#f5f5f5" },
  light: { color: "#f2f2f3", symbolColor: "#1a1a1a" },
} as const;

type ProductWindowChrome = Pick<
  BrowserWindowConstructorOptions,
  "titleBarOverlay" | "titleBarStyle" | "trafficLightPosition"
>;

/** Linux title-bar overlay for the resolved theme; other platforms have none to recolor. */
export function productTitleBarOverlay(
  platform: NodeJS.Platform,
  dark: boolean
): TitleBarOverlay | undefined {
  if (platform !== "linux") return undefined;
  return { ...LINUX_OVERLAY_COLORS[dark ? "dark" : "light"], height: PRODUCT_TITLE_BAR_HEIGHT };
}

export function productWindowChrome(platform: NodeJS.Platform, dark: boolean): ProductWindowChrome {
  if (platform === "darwin") {
    return {
      titleBarStyle: "hidden",
      titleBarOverlay: { height: PRODUCT_TITLE_BAR_HEIGHT },
      trafficLightPosition: MAC_TRAFFIC_LIGHT_POSITION,
    };
  }

  const overlay = productTitleBarOverlay(platform, dark);
  if (overlay) {
    return { titleBarStyle: "hidden", titleBarOverlay: overlay };
  }

  return {};
}
