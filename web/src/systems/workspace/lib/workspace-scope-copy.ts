export const GLOBAL_SCOPE_COPY = {
  controlName: "Global scope",
  chipLabel: "Global",
  chipMonogram: "~",
  tooltipOff: "Global — every project",
  tooltipLocked: "Add a project to focus on it",
  tooltipPickWorkspace: "Pick a project to focus on it",
  liveOn: "Global scope on",
  liveOff: "Global scope off",
  paletteToggleOn: "Turn on Global scope",
  paletteToggleOff: "Turn off Global scope",
  paletteSwitchTurnsOff: "turns Global scope off",
  skipOnboarding: "Skip — use my home folder",
} as const;

export function globalScopeTooltipOn(workspaceName: string): string {
  return `Back to ${workspaceName}`;
}

export function destinationLabel(scope: string, workspaceName: string | undefined | null): string {
  if (scope === "global") return GLOBAL_SCOPE_COPY.chipLabel;
  const name = workspaceName?.trim();
  return name && name.length > 0 ? name : "project";
}
