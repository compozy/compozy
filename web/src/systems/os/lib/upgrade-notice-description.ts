import type { WindowManagerMigrationSummary } from "./window-manager-types";

function count(value: number, one: string, many: string): string {
  return `${value} ${value === 1 ? one : many}`;
}

/** What the upgrade did, in the counts the migration status reports. */
export function upgradeNoticeDescription(summary: WindowManagerMigrationSummary | null): string {
  const floating = summary?.floatingToTabs ?? 0;
  const minimized = summary?.minimizedToTabs ?? 0;
  const moved = [
    floating > 0 ? count(floating, "floating window", "floating windows") : null,
    minimized > 0 ? count(minimized, "minimized window", "minimized windows") : null,
  ].filter(part => part !== null);
  const sentences = [
    moved.length === 0
      ? "Floating and minimized windows are tabs now."
      : `${moved.join(" and ")} ${floating + minimized === 1 ? "is a tab" : "are tabs"} now.`,
  ];
  const pins = summary?.pinsDropped ?? 0;
  if (pins === 1) sentences.push("1 pin inside a split was removed.");
  if (pins > 1) sentences.push(`${pins} pins inside splits were removed.`);
  return sentences.join(" ");
}
