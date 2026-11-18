import { isForwardedBootMethod, type ForwardedBootMethod } from "./boot-contract";

const RELEASES_URL = "https://github.com/compozy/compozy/releases/tag/v";
/** A plain release version, so a snapshot can never steer the opened URL elsewhere. */
const RELEASE_VERSION = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?$/u;

export interface BootControlDependencies {
  readonly openLogs: () => Promise<string>;
  /** Opens the global config file; resolves to a failure message, or "" once it opened. */
  readonly openConfig: () => Promise<string>;
  readonly openExternal: (url: string) => Promise<void>;
  /** The version the current boot moves to, if the shell state names one. */
  readonly targetVersion: () => string | undefined;
  readonly quit: () => void;
  readonly forward: (method: ForwardedBootMethod, params: unknown) => Promise<unknown>;
}

/** The release notes page of one runtime version, or null when the version is not a release. */
export function releaseNotesURL(version: string | undefined): string | null {
  const normalized = version?.trim().replace(/^v/u, "");
  return normalized && RELEASE_VERSION.test(normalized) ? `${RELEASES_URL}${normalized}` : null;
}

/**
 * Answers the boot page's `boot:control` calls. Shell-local actions (logs, config, release notes,
 * quit) run here; diagnostics and retry go to the app control handler the CLI uses too.
 */
export function createBootControl(dependencies: BootControlDependencies) {
  return async (method: unknown, params: unknown): Promise<unknown> => {
    if (method === "open_logs") {
      const failure = await dependencies.openLogs();
      if (failure) throw new Error(failure);
      return { opened: true };
    }
    if (method === "open_config") {
      const failure = await dependencies.openConfig();
      if (failure) throw new Error(failure);
      return { opened: true };
    }
    if (method === "open_release_notes") {
      const url = releaseNotesURL(dependencies.targetVersion());
      if (!url) throw new Error("There are no release notes for this version.");
      await dependencies.openExternal(url);
      return { opened: true };
    }
    if (method === "quit") {
      dependencies.quit();
      return { quitting: true };
    }
    if (typeof method !== "string") throw new Error("The boot action is invalid.");
    if (isForwardedBootMethod(method)) return await dependencies.forward(method, params);
    throw new Error("The boot action is not supported.");
  };
}
