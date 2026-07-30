/**
 * Maturity vocabulary shared by the frontmatter schema (`source.config.ts`) and the masthead that
 * renders it. One fixed set of words keeps COPY.md's claim standards checkable: a page either
 * carries one of these labels or documents shipped behavior and carries none.
 *
 * Kept out of `source.config.ts` because that module may only export collections.
 */
export const MATURITY_LEVELS = ["shipped", "beta", "alpha"] as const;

export type MaturityLevel = (typeof MATURITY_LEVELS)[number];

const MATURITY_LABELS: Record<MaturityLevel, string> = {
  shipped: "Shipped",
  beta: "Beta",
  alpha: "Alpha",
};

export function maturityLabel(level: MaturityLevel): string {
  return MATURITY_LABELS[level];
}

export function isMaturityLevel(value: unknown): value is MaturityLevel {
  return typeof value === "string" && (MATURITY_LEVELS as readonly string[]).includes(value);
}
