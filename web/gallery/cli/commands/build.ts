import { resolve } from "node:path";
import { build as viteBuild } from "vite";

import { galleryRoot } from "../../node/paths.ts";

/** `proto build`: the static gallery in web/dist-gallery. */
export async function build() {
  await viteBuild({ configFile: resolve(galleryRoot, "vite.config.ts") });
}
