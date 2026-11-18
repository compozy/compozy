// A kit template: one React component sized only in cqi, plus the default values of its
// editable fields. Size, margins and print data live in ../kit-specs.ts.
import type { ComponentType } from "react";

/** Editable text fields; `/kit/<id>?field=value` overrides any of them before export. */
export type KitFields = Record<string, string | undefined>;

export interface KitTemplate {
  component: ComponentType<KitFields>;
  /** fields this entry passes on top of the component defaults (variants of one component) */
  fields?: Record<string, string>;
}
