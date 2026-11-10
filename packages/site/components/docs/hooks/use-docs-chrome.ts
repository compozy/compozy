"use client";

import { use } from "react";

import { DocsChromeContext } from "@/components/docs/docs-chrome-context";

/** The docs overlays: whether the drawer or the search palette is open, and their controls. */
export function useDocsChrome() {
  return use(DocsChromeContext);
}
