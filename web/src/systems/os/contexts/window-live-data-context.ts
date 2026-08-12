import { createContext } from "react";

/** Defaults to live for surfaces rendered outside the retained-window OS shell. */
export const WindowLiveDataContext = createContext(true);
