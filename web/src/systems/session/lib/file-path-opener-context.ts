import { createContext } from "react";

import type { OpenInEditor } from "../hooks/use-open-in-editor";

/** The transcript's open action; provided only on desktop shells that can open files. */
export const FilePathOpenerContext = createContext<OpenInEditor | null>(null);
