/** In-draft navigation: any link inside a page opens another page through this. */
import { createContext } from "react";

export const DocsNavContext = createContext<(slug: string) => void>(() => {});
