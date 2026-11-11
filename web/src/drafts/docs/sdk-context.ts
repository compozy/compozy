/** The language the SDK reference is being read in, shared by every SDK page. */
import { createContext } from "react";

import type { SdkLang } from "./sdk";

export const SdkLangContext = createContext<SdkLang>("ts");
export const SdkSetLangContext = createContext<(lang: SdkLang) => void>(() => {});
