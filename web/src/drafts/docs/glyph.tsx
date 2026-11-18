/** Draws a registry icon by name, so templates never build a component while rendering. */
import { createElement } from "react";

import { iconOf } from "./icons";

export function DocsGlyph({ name, className }: { name: string | undefined; className?: string }) {
  return createElement(iconOf(name), { className, "aria-hidden": true });
}
