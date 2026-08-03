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
  /** Whether `http` is declared in `start[]` — a run form on a loop that does not allow it. */
  thisFormDeclared: boolean;
  /** Every other kind declared in `start[]`, deduplicated, in declaration order. */
  alsoDeclared: string[];
}

/**
 * Read-truth projection of a loop's declared `start[]` allowlist for the run form.
 *
 * Read-only by contract: authoring the allowlist is Spec 3 (ADR-018 §3), so this
 * never proposes a kind the definition does not declare and never offers an edit.
 */
export function describeStartKinds(start: readonly LoopStartBinding[] | undefined): LoopStartKinds {
  const seen = new Set<string>();
  const alsoDeclared: string[] = [];
  let thisFormDeclared = false;
  for (const binding of start ?? []) {
    const kind = binding.kind?.trim();
    if (!kind || seen.has(kind)) continue;
    seen.add(kind);
    if (kind === RUN_FORM_START_KIND) {
      thisFormDeclared = true;
      continue;
    }
    alsoDeclared.push(kind);
  }
  return { thisForm: RUN_FORM_START_KIND, thisFormDeclared, alsoDeclared };
}
