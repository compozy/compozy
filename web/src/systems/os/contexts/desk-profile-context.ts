import { createContext } from "react";

/**
 * The profile whose desk the shell presents in this render. `null` outside the OS shell, where no
 * runtime binding exists to lag behind it.
 */
export const DeskProfileContext = createContext<string | null>(null);
