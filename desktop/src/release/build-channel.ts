export type DesktopBuildChannel = "beta" | "development";

export function resolveDesktopBuildChannel(value: string | undefined): DesktopBuildChannel {
  const channel = value?.trim() || "development";
  if (channel !== "development" && channel !== "beta") {
    throw new Error("COMPOZY_RELEASE_CHANNEL must be development or beta.");
  }
  return channel;
}
