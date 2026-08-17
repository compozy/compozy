export const GLOBAL_SCOPE_COPY = {
  controlName: "Global scope",
  chipLabel: "Global",
  chipMonogram: "~",
  tooltipOff: "Global scope ~",
  tooltipLocked: "Add a workspace to scope down",
  tooltipPickWorkspace: "Pick a workspace to scope down",
  liveOn: "Global scope on",
  liveOff: "Global scope off",
  rememberedWorkspaceRemoved:
    "The remembered workspace was removed — pick a workspace to leave Global scope",
  paletteToggleOn: "Turn on Global scope",
  paletteToggleOff: "Turn off Global scope",
  paletteSwitchTurnsOff: "turns Global scope off",
  skipOnboarding: "Skip",
  skipOnboardingHint: "Start in Global scope — your home folder, ~. Add project folders any time.",
} as const;

export function globalScopeTooltipOn(workspaceName: string): string {
  return `Back to ${workspaceName}`;
}

export function destinationLabel(scope: string, workspaceName: string | undefined | null): string {
  if (scope === "global") return GLOBAL_SCOPE_COPY.chipLabel;
  const name = workspaceName?.trim();
  return name && name.length > 0 ? name : "workspace";
}
