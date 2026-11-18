import { useAppearanceSettingsPane } from "../../hooks/use-appearance-settings-pane";
import { AppearanceSettingsPaneView } from "./appearance-settings-pane-view";
import { useSettingsTopbar } from "@/systems/settings";

/** The Appearance pane bound to the theme store, the daemon text settings and the desktop doc. */
export function AppearanceSettingsPane() {
  useSettingsTopbar();
  const appearance = useAppearanceSettingsPane();

  return (
    <AppearanceSettingsPaneView
      fonts={appearance.fonts}
      onFontChange={appearance.setFont}
      onReduceMotionChange={appearance.setReduceMotion}
      onThemeChange={appearance.setTheme}
      onTypeChange={appearance.setType}
      onWallpaperChange={appearance.setWallpaper}
      reduceMotion={appearance.reduceMotion}
      systemReducedMotion={appearance.systemReducedMotion}
      theme={appearance.theme}
      type={appearance.type}
      wallpaper={appearance.wallpaper}
    />
  );
}
