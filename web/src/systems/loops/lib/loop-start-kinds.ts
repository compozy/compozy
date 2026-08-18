import type { LoopStartBinding } from "../types";

/**
 * The start kind this browser form actually uses. The run form posts
 * `POST /api/workspaces/{ws}/loops/{name}/run`, so every run it creates is admitted
 * as `http` — never `manual`, and never the kind an automation would carry.
 */
export const RUN_FORM_START_KIND = "http";

export interface LoopStartKinds {
  /** The kind this form starts the loop as. */
  thisForm: typeof RUN_FORM_START_KIND;
  /** Every other kind declared in `start[]`, deduplicated, in declaration order. */
  alsoDeclared: string[];
}

export function describeStartKinds(start: readonly LoopStartBinding[] | undefined): LoopStartKinds {
  const seen = new Set<string>();
  const alsoDeclared: string[] = [];
  for (const binding of start ?? []) {
    const kind = binding.kind?.trim();
    if (!kind || seen.has(kind)) continue;
    seen.add(kind);
    if (kind === RUN_FORM_START_KIND) continue;
    alsoDeclared.push(kind);
  }
  return { thisForm: RUN_FORM_START_KIND, alsoDeclared };
}
