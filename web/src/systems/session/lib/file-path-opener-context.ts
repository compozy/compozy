import { createContext } from "react";

import type { OpenInEditor } from "../hooks/use-open-in-editor";

/** The transcript's open action; provided only on desktop shells that can open files. */
export const FilePathOpenerContext = createContext<OpenInEditor | null>(null);

/**
 * True inside a rendered link or path affordance. Inline code there keeps the link's own
 * behavior: no nested open button and no modifier-click takeover.
 */
export const FilePathInsideLinkContext = createContext(false);
