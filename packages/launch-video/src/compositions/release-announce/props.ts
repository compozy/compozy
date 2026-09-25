import type { ReleaseAnnounceProps } from "./schema";
import release from "@/releases/beta27.json";

export const DURATION_IN_FRAMES = 192;
export const FPS = 60;

export const releaseAnnounceDefaults: ReleaseAnnounceProps = {
  version: release.version,
  features: release.features,
};
