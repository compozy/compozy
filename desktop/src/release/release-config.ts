export type ReleaseArch = "arm64" | "x64";
export type ReleasePlatform = "linux" | "mac" | "win";
export type ReleaseChannel = "stable" | "beta";

export interface WindowsAzureSigning {
  readonly endpoint: string;
  readonly publisherName: string;
  readonly codeSigningAccountName: string;
  readonly certificateProfileName: string;
}

export interface ReleaseConfigInput {
  readonly windowsAzureSigning?: WindowsAzureSigning;
  readonly arch: ReleaseArch;
  readonly channel: ReleaseChannel;
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
  if (input.channel !== "stable" && input.channel !== "beta") {
    throw new Error("Release channel must be stable or beta.");
  }
  if (input.platform !== "mac" && input.platform !== "linux" && input.platform !== "win") {
    throw new Error("Release platform must be mac, linux or win.");
  }
  if (input.arch !== "arm64" && input.arch !== "x64") {
    throw new Error("Release architecture must be arm64 or x64.");
  }
  if (input.platform !== "mac" && input.arch !== "x64") {
    throw new Error("Linux and Windows desktop releases support x64 only.");
  }
  if (input.windowsAzureSigning && input.platform !== "win") {
    throw new Error("Azure signing applies only to Windows releases.");
  }
  if (input.platform !== "mac" && input.notarize) {
    throw new Error("Notarization applies only to macOS releases.");
  }
}

export function buildReleaseConfig(input: ReleaseConfigInput): Record<string, unknown> {
  validateReleaseConfig(input);
  const target =
    input.platform === "mac"
      ? ["dmg", "zip"]
      : input.platform === "win"
        ? ["nsis"]
        : ["AppImage", "deb"];
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
      : input.platform === "win"
        ? {
            win: {
              artifactName: "CompozyOS-${version}-win-x64.${ext}",
              ...(input.windowsAzureSigning ? { azureSignOptions: input.windowsAzureSigning } : {}),
              detectUpdateChannel: false,
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

/** Signing credentials stay in the runner environment; only public signer metadata enters the package config. */
export function windowsAzureSigningFromEnvironment(
  environment: Readonly<Record<string, string | undefined>>
): WindowsAzureSigning | undefined {
  const fields = {
    endpoint: "AZURE_SIGNING_ENDPOINT",
    publisherName: "AZURE_PUBLISHER_NAME",
    codeSigningAccountName: "AZURE_CODE_SIGNING_ACCOUNT",
    certificateProfileName: "AZURE_CERTIFICATE_PROFILE",
  } as const;
  if (!Object.values(fields).some(name => environment[name]?.trim())) return undefined;
  const required = (name: string): string => {
    const value = environment[name]?.trim();
    if (!value) throw new Error(`Windows Azure signing requires ${name}.`);
    return value;
  };
  const values: WindowsAzureSigning = {
    endpoint: required(fields.endpoint),
    publisherName: required(fields.publisherName),
    codeSigningAccountName: required(fields.codeSigningAccountName),
    certificateProfileName: required(fields.certificateProfileName),
  };
  const endpoint = new URL(values.endpoint);
  if (endpoint.protocol !== "https:" || endpoint.username || endpoint.password) {
    throw new Error(
      "Windows Azure signing requires an HTTPS service endpoint without credentials."
    );
  }
  return values;
}
