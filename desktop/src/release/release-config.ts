export type ReleaseArch = "arm64" | "x64";
export type ReleasePlatform = "linux" | "mac";

export interface ReleaseConfigInput {
  readonly arch: ReleaseArch;
  readonly channel: "beta";
  readonly notarize: boolean;
  readonly platform: ReleasePlatform;
  readonly version: string;
}

const strictSemver =
  /^\d+\.\d+\.\d+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/u;

export function validateReleaseConfig(input: ReleaseConfigInput): void {
  if (!strictSemver.test(input.version)) {
    throw new Error("Release version must be strict unprefixed SemVer.");
  }
  if (input.platform !== "mac" && input.platform !== "linux") {
    throw new Error("Release platform must be mac or linux.");
  }
  if (input.arch !== "arm64" && input.arch !== "x64") {
    throw new Error("Release architecture must be arm64 or x64.");
  }
  if (input.platform === "linux" && input.arch !== "x64") {
    throw new Error("Linux desktop releases support x64 only.");
  }
  if (input.platform === "linux" && input.notarize) {
    throw new Error("Notarization applies only to macOS releases.");
  }
}

export function buildReleaseConfig(input: ReleaseConfigInput): Record<string, unknown> {
  validateReleaseConfig(input);
  const target = input.platform === "mac" ? ["dmg", "zip"] : ["AppImage", "deb"];
  return {
    extends: "./electron-builder.yml",
    extraMetadata: { version: input.version },
    publish: {
      provider: "generic",
      url: `https://raw.githubusercontent.com/compozy/compozy/channel-${input.channel}/desktop/`,
    },
    ...(input.platform === "mac"
      ? {
          dmg: { sign: true },
          mac: {
            detectUpdateChannel: false,
            hardenedRuntime: true,
            notarize: input.notarize,
            target,
          },
        }
      : {
          linux: {
            artifactName: "CompozyOS-${version}-linux-x64.${ext}",
            detectUpdateChannel: false,
            executableName: "compozyos",
            maintainer: "Compozy",
            syncDesktopName: true,
            target,
          },
        }),
  };
}
