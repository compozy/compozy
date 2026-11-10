/**
 * Product captures for the releases they show. GitHub release notes carry no images, so the
 * site keeps this map by version; a release without an entry shows no image.
 */
export interface ReleaseCapture {
  src: string;
  alt: string;
}

const CAPTURES: Record<string, ReleaseCapture> = {
  "v0.5.5": {
    src: "/images/changelog/v0.5.5.jpg",
    alt: "The Marketplace by category, with one-step Add",
  },
  "v0.5.1": {
    src: "/images/changelog/v0.5.1.jpg",
    alt: "An always-on agent's check-in schedule and sources",
  },
  "v0.5.0": {
    src: "/images/changelog/v0.5.0.jpg",
    alt: "A grid of agents that keep watch over projects",
  },
  "v0.4.0": {
    src: "/images/changelog/v0.4.0.jpg",
    alt: "The sessions board, sorted by who owns the next step",
  },
  "v0.3.0-beta.38": {
    src: "/images/changelog/v0.3.0-beta.38.jpg",
    alt: "Sessions across every project",
  },
};

export function releaseCapture(version: string): ReleaseCapture | undefined {
  return CAPTURES[version];
}
