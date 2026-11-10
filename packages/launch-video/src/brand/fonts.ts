/**
 * Brand typefaces, loaded through @remotion/google-fonts so the renderer blocks on font readiness
 * (loadFont wires up delayRender itself). Same families the product loads via fontsource in
 * web/src/styles.css and the site loads via next/font.
 */

import { loadFont as loadFigtree } from "@remotion/google-fonts/Figtree";
import { loadFont as loadGeistMono } from "@remotion/google-fonts/GeistMono";

const figtree = loadFigtree("normal", { weights: ["400", "500", "600"], subsets: ["latin"] });
const geistMono = loadGeistMono("normal", { weights: ["500"], subsets: ["latin"] });

/** --font-sans */
export const FONT_SANS = figtree.fontFamily;

/** --font-mono */
export const FONT_MONO = geistMono.fontFamily;
