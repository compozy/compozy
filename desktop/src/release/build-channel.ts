import type { ReleaseChannel } from "./release-config";

export type DesktopBuildChannel = ReleaseChannel | "development";

export function resolveDesktopBuildChannel(value: string | undefined): DesktopBuildChannel {
  const channel = value?.trim() || "development";
  if (channel !== "development" && channel !== "stable" && channel !== "beta") {
    throw new Error("COMPOZY_RELEASE_CHANNEL must be development, stable, or beta.");
  }
  return channel;
}
