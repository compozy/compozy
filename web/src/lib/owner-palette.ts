/**
 * Owner-avatar palette resolution lives in `@agh/ui` so primitives + web consumers share
 * a single source. This module is a thin re-export — runtime callsites
 * import from here (or directly from `@agh/ui`) and receive `var(--color-avatar-*)` strings.
 */
export {
  AGENT_SLOT_COUNT,
  HUMAN_SLOT_COUNT,
  SYSTEM_SLOT_COUNT,
  colorsFor,
  seed,
  type OwnerColors,
  type OwnerKind,
} from "@agh/ui";
