import { Settings } from "lucide-react";

import { Icon, ThemeToggle, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { useThemePreference } from "@/systems/theme";
import { useIntentPreload } from "@/hooks/use-intent-preload";

import { usePaletteCommand } from "../hooks/use-palette-registry";
import { OsRailButton } from "./os-dock";

export interface OsRailFootProps {
  /** Profile switcher, supplied by the shell; it owns its own reads. */
  profileSwitcher?: React.ReactNode;
  onOpenSettings: () => void;
  onPreloadSettings?: () => void | Promise<unknown>;
  /** Tooltip side: beside the rail, or above the compact tab bar. */
  tipSide?: "right" | "top";
}

/**
 * The rail foot (shell-rail v2 `.rail-foot`, D6): profile switcher, then the
 * light/dark toggle, then Settings — each a 40px rail item. Compact renders the
 * same controls at the end of the bottom tab bar.
 */
export function OsRailFoot({
  profileSwitcher,
  onOpenSettings,
  onPreloadSettings,
  tipSide = "right",
}: OsRailFootProps) {
  const theme = useThemePreference();
  const settingsCommand = usePaletteCommand("settings.general");
  const intent = useIntentPreload(
    onPreloadSettings ?? skipPreload,
    !!onPreloadSettings && settingsCommand?.available === true
  );
  return (
    <>
      {profileSwitcher}
      <Tooltip>
        <TooltipTrigger
          render={
            <ThemeToggle resolvedTheme={theme.resolvedTheme} onClick={theme.toggleResolved} />
          }
        />
        <TooltipContent side={tipSide} sideOffset={10}>
          {theme.resolvedTheme === "dark" ? "Light mode" : "Dark mode"}
        </TooltipContent>
      </Tooltip>
      <OsRailButton
        {...intent}
        data-slot="os-rail-settings"
        label="Settings"
        tipSide={tipSide}
        disabled={settingsCommand?.available !== true}
        onClick={onOpenSettings}
      >
        <Icon as={Settings} className="size-4" />
      </OsRailButton>
    </>
  );
}

function skipPreload(): void {}
