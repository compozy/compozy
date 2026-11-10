import { ASSET } from "@/components/landing/assets";
import appMarketplace from "@/components/marketplace/assets/app-marketplace.jpg";

/**
 * Product captures for the releases they show. GitHub release notes carry no images, so the
 * site keeps this map by version; a release without an entry shows no image. Each entry reuses
 * the landing or marketplace copy of its capture, so one file serves every page that shows it.
 */
export interface ReleaseCapture {
  src: string;
  alt: string;
}

const CAPTURES: Record<string, ReleaseCapture> = {
  "v0.5.5": {
    src: appMarketplace.src,
    alt: "The Marketplace by category, with one-step Add",
  },
  "v0.5.1": {
    src: ASSET.heartbeat,
    alt: "An always-on agent's check-in schedule and sources",
  },
  "v0.5.0": {
    src: ASSET.agents,
    alt: "A grid of agents that keep watch over projects",
  },
  "v0.4.0": {
    src: ASSET.board,
    alt: "The sessions board, sorted by who owns the next step",
  },
  "v0.3.0-beta.38": {
    src: ASSET.workspaces,
    alt: "Sessions across every project",
  },
};

export function releaseCapture(version: string): ReleaseCapture | undefined {
  return CAPTURES[version];
}
