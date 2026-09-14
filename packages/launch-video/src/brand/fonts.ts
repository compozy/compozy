/**
 * Brand typefaces, loaded through @remotion/google-fonts so the renderer blocks on font readiness
 * (loadFont wires up delayRender itself). Same two families the product loads via fontsource in
 * web/src/styles.css and the site loads via next/font.
 */

import { loadFont as loadGeist } from "@remotion/google-fonts/Geist";
import { loadFont as loadJetBrainsMono } from "@remotion/google-fonts/JetBrainsMono";

const geist = loadGeist("normal", { weights: ["400", "500", "600"], subsets: ["latin"] });
const jetBrainsMono = loadJetBrainsMono("normal", { weights: ["500"], subsets: ["latin"] });

/** --font-sans */
export const FONT_SANS = geist.fontFamily;

/** --font-mono */
export const FONT_MONO = jetBrainsMono.fontFamily;
