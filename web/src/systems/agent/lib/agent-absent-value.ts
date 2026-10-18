/** Absent overridable scalar → muted "Default" (never em dash). */
export function formatAbsentOverride(value: string | null | undefined): string {
  const trimmed = value?.trim() ?? "";
  return trimmed.length > 0 ? trimmed : "Default";
}

/** Absent list → muted "None". */
export function formatAbsentList(count: number): string {
  return count > 0 ? String(count) : "None";
}

export function formatAbsentListLabels(labels: readonly string[]): string {
  if (labels.length === 0) return "None";
  return labels.join(" · ");
}
