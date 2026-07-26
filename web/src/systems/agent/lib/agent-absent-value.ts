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

/** Client-derived prompt size for at-a-glance / AGENT.md meta. */
export function countPromptWords(prompt: string | null | undefined): number {
  const trimmed = prompt?.trim() ?? "";
  if (trimmed.length === 0) return 0;
  return trimmed.split(/\s+/).filter(Boolean).length;
}

export function formatPromptWordCount(prompt: string | null | undefined): string {
  const words = countPromptWords(prompt);
  return words === 0 ? "0 words" : `~${words} words`;
}

export function formatSkillsPolicyLine(
  skills: { disabled?: string[] | null } | null | undefined
): string {
  const disabled = skills?.disabled ?? [];
  if (disabled.length === 0) return "All skills enabled";
  if (disabled.length === 1) return `1 skill disabled (${disabled[0]})`;
  return `${disabled.length} skills disabled`;
}
