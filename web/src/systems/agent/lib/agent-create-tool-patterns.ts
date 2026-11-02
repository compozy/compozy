export function validateToolPatternList(values: readonly string[], label: string): string | null {
  for (const value of values) {
    const trimmed = value.trim();
    if (trimmed.length === 0) return label + " entries cannot be blank.";
    if (!isValidToolPattern(trimmed)) {
      return (
        label +
        " entries must use canonical IDs such as compozy__skill_view or namespace wildcards such as compozy__task_*."
      );
    }
  }
  return null;
}

export function validateToolsetList(values: readonly string[]): string | null {
  for (const value of values) {
    const trimmed = value.trim();
    if (trimmed.length === 0) return "Toolset entries cannot be blank.";
    if (!isValidCanonicalRef(trimmed) || trimmed.includes("*")) {
      return "Toolset entries must use canonical IDs such as compozy__catalog.";
    }
  }
  return null;
}

function isValidToolPattern(value: string): boolean {
  if (value.includes("*")) {
    if (value.split("*").length !== 2 || !value.endsWith("*")) return false;
    const prefix = value.slice(0, -1);
    if (prefix.length === 0 || (!prefix.endsWith("_") && !prefix.endsWith("__"))) {
      return false;
    }
    return isValidCanonicalRef(prefix + "x");
  }
  return isValidCanonicalRef(value);
}

function isValidCanonicalRef(value: string): boolean {
  if (!value.includes("__")) return false;
  if (/\s/.test(value)) return false;
  if (value.includes("/") || value.includes("\\")) return false;
  return /^[A-Za-z0-9][A-Za-z0-9_.-]*__[A-Za-z0-9][A-Za-z0-9_.-]*$/.test(value);
}
