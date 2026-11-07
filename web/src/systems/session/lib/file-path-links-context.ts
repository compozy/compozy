import { createContext } from "react";

import type { FilePathLinks } from "../hooks/use-file-path-links";

/** The transcript's path actions; `null` outside a session transcript or without any action. */
export const FilePathLinksContext = createContext<FilePathLinks | null>(null);

/**
 * True inside a rendered link or path affordance. Inline code there keeps the link's own
 * behavior: no nested affordance and no click takeover.
 */
export const FilePathInsideLinkContext = createContext(false);
