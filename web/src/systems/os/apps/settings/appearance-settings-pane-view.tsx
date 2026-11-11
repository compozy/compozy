import { PillGroup, Switch, type PillGroupItem } from "@compozy/ui";

import { FontPicker } from "./font-picker";
import { WallpaperPicker } from "./wallpaper-picker";
import type { OsWallpaper } from "../../lib/os-types";
import { SettingRow } from "@/systems/settings/components/setting-row";
import { SettingsGroup } from "@/systems/settings/components/settings-group";
import { SettingsPageFrame } from "@/systems/settings/components/settings-page-frame";
import type { FontAppearance, FontRole } from "@/systems/theme/lib/font-appearance";
import {
  LETTER_SPACING_STEPS,
  TEXT_SIZE_STEPS,
  type LetterSpacing,
  type TextSize,
  type TypeAppearance,
} from "@/systems/theme/lib/type-appearance";
import type { ThemePreference } from "@/systems/theme/types";

const THEMES: ReadonlyArray<PillGroupItem<ThemePreference>> = [
  { value: "light", label: "Light", testId: "os-appearance-theme-light" },
  { value: "dark", label: "Dark", testId: "os-appearance-theme-dark" },
  { value: "system", label: "System", testId: "os-appearance-theme-system" },
];

const TEXT_SIZES: ReadonlyArray<PillGroupItem<TextSize>> = TEXT_SIZE_STEPS.map(step => ({
  value: step.value,
  label: step.label,
  testId: `os-appearance-text-size-${step.value}`,
}));

const LETTER_SPACINGS: ReadonlyArray<PillGroupItem<LetterSpacing>> = LETTER_SPACING_STEPS.map(
  step => ({
    value: step.value,
    label: step.label,
    testId: `os-appearance-letter-spacing-${step.value}`,
  })
);

export interface AppearanceSettingsPaneViewProps {
  theme: ThemePreference;
  onThemeChange: (theme: ThemePreference) => void;
  reduceMotion: boolean;
  /** The system's reduced-motion preference, which wins over the toggle while on. */
  systemReducedMotion: boolean;
  onReduceMotionChange: (reduceMotion: boolean) => void;
  fonts: FontAppearance;
  onFontChange: (role: FontRole, family: string) => void;
  type: TypeAppearance;
  /** Receives only the step that changed. */
  onTypeChange: (patch: Partial<TypeAppearance>) => void;
  wallpaper: OsWallpaper;
  onWallpaperChange: (wallpaper: OsWallpaper) => void;
  /** Opens one font list on mount (prototype states that review the open picker). */
  openFontPicker?: FontRole;
}

/**
 * The Appearance pane (US-015): this browser's theme (light, dark, or follow
 * the system), the daemon-wide text settings — Interface and Code fonts
 * (#984), text size and letter spacing — shell-session wallpaper, and the
 * in-product reduced-motion preference. The system reduced-motion preference
 * always wins over the toggle (US-015.EC-1).
 */
export function AppearanceSettingsPaneView({
  theme,
  onThemeChange,
  reduceMotion,
  systemReducedMotion,
  onReduceMotionChange,
  fonts,
  onFontChange,
  type,
  onTypeChange,
  wallpaper,
  onWallpaperChange,
  openFontPicker,
}: AppearanceSettingsPaneViewProps) {
  return (
    <SettingsPageFrame slug="appearance">
      <div className="flex flex-col gap-6" data-testid="os-appearance-pane">
        <SettingsGroup title="Theme and motion">
          <SettingRow
            control={
              <PillGroup<ThemePreference>
                data-testid="os-appearance-theme"
                items={THEMES}
                value={theme}
                onChange={onThemeChange}
              />
            }
            data-testid="os-appearance-theme-row"
            help="System follows your computer's light or dark setting."
            label="Theme"
          />
          <SettingRow
            control={
              <Switch
                checked={reduceMotion}
                data-testid="os-appearance-reduce-motion"
                onCheckedChange={checked => onReduceMotionChange(checked === true)}
              />
            }
            data-testid="os-appearance-reduce-motion-row"
            description={
              systemReducedMotion
                ? "Your system already prefers reduced motion — that preference wins while it is on."
                : undefined
            }
            help={
              systemReducedMotion
                ? undefined
                : "Window, dock, and minimize animations become instant."
            }
            label="Reduce motion"
          />
        </SettingsGroup>
        <SettingsGroup
          description="Text settings apply to every app connected to this CompozyOS."
          title="Text"
        >
          <SettingRow
            control={
              <FontPicker
                data-testid="os-appearance-font-sans"
                defaultOpen={openFontPicker === "sans"}
                label="Interface font"
                role="sans"
                value={fonts.sans}
                onChange={family => onFontChange("sans", family)}
              />
            }
            data-testid="os-appearance-font-sans-row"
            help="Menus, lists, and conversations."
            label="Interface"
          />
          <SettingRow
            control={
              <FontPicker
                data-testid="os-appearance-font-mono"
                defaultOpen={openFontPicker === "mono"}
                label="Code font"
                role="mono"
                value={fonts.mono}
                onChange={family => onFontChange("mono", family)}
              />
            }
            data-testid="os-appearance-font-mono-row"
            help="Code blocks, diffs, paths, and the terminal."
            label="Code"
          />
          <SettingRow
            control={
              <PillGroup<TextSize>
                data-testid="os-appearance-text-size"
                items={TEXT_SIZES}
                value={type.size}
                onChange={size => onTypeChange({ size })}
              />
            }
            data-testid="os-appearance-text-size-row"
            help="Scales all text in the app. The terminal keeps its own size."
            label="Text size"
          />
          <SettingRow
            control={
              <PillGroup<LetterSpacing>
                data-testid="os-appearance-letter-spacing"
                items={LETTER_SPACINGS}
                value={type.spacing}
                onChange={spacing => onTypeChange({ spacing })}
              />
            }
            data-testid="os-appearance-letter-spacing-row"
            help="Space between letters in interface text."
            label="Letter spacing"
          />
        </SettingsGroup>
        <SettingsGroup bare title="Wallpaper">
          <WallpaperPicker value={wallpaper} onChange={onWallpaperChange} />
        </SettingsGroup>
      </div>
    </SettingsPageFrame>
  );
}
