import { OnboardingApiError } from "../adapters/onboarding-api";
import type { FSBrowseResponse } from "../types";
import {
  isAbsolutePath,
  isWindowsPath,
  normalizeDirectoryPath,
  suggestionValue,
} from "./directory-path";

/** `incomplete`: the text is a place to browse from (`~/`), not yet a choice. */
export type DirectoryPathStatus = "empty" | "incomplete" | "checking" | "invalid" | "valid";

export const DIRECTORY_PATH_COPY = {
  relative: "Use an absolute path, like ~/code/app.",
  missing: "No folder at this path.",
  notDirectory: "This path is a file, not a folder.",
  unreadable: "This folder can't be read. Check its permissions.",
  alreadyProject: "This folder is already a project.",
  home: "Your home folder can't be a project. Choose a folder inside it.",
  homeUnavailable: "Couldn't read your home folder. Use a full path, like /code/app.",
  browseFailed: "Couldn't open the folder dialog.",
} as const;

type FSEntry = FSBrowseResponse["entries"][number];

/** Folders under the listed directory whose name continues what was typed. */
export function directorySuggestions({
  entries,
  prefix,
  typed,
  windows,
  limit,
}: {
  entries: readonly FSEntry[] | undefined;
  prefix: string;
  typed: string;
  /** Whether the typed text is a Windows-style path once expanded. */
  windows: boolean;
  limit: number;
}) {
  const match = prefix.toLowerCase();
  return (entries ?? [])
    .filter(entry => {
      const name = entry.name.toLowerCase();
      // The folder already typed in full is not a completion; `/` drills into it.
      // Case-sensitive filesystems can hold `Code` beside `code`, so only an exact name is excluded.
      return entry.is_dir && name.startsWith(match) && entry.name !== prefix;
    })
    .slice(0, limit)
    .map(entry => ({ value: suggestionValue(typed, entry.name, windows), label: entry.name }));
}

function browseErrorMessage(error: unknown): string {
  if (error instanceof OnboardingApiError) {
    if (error.status === 404) return DIRECTORY_PATH_COPY.missing;
    if (error.status === 400) return DIRECTORY_PATH_COPY.notDirectory;
    if (error.status === 403) return DIRECTORY_PATH_COPY.unreadable;
  }
  return error instanceof Error && error.message.trim() !== ""
    ? error.message
    : DIRECTORY_PATH_COPY.missing;
}

function judgeListedFolder(
  typed: string,
  candidate: string,
  resolved: string,
  home: string | null,
  isProject: (path: string) => boolean
): { status: DirectoryPathStatus; message: string | null } {
  if (home !== null && (candidate === normalizeDirectoryPath(home) || resolved === home)) {
    // `~/` is where browsing starts; only `~` itself reads as choosing home.
    const trailingSeparator = isWindowsPath(home) ? /[\\/]$/ : /\/$/;
    return trailingSeparator.test(typed)
      ? { status: "incomplete", message: null }
      : { status: "invalid", message: DIRECTORY_PATH_COPY.home };
  }
  if (isProject(candidate) || isProject(resolved)) {
    return { status: "invalid", message: DIRECTORY_PATH_COPY.alreadyProject };
  }
  return { status: "valid", message: null };
}

/**
 * Judges the typed folder. `validation` is the daemon's listing of the settled
 * candidate, or `null` while typing has not paused yet.
 */
export function judgeDirectoryPath({
  typed,
  expanded,
  candidate,
  home,
  homeUnavailable,
  validation,
  isProject,
}: {
  typed: string;
  expanded: string;
  candidate: string;
  home: string | null;
  /** The daemon's home listing failed, so `~` cannot be expanded. */
  homeUnavailable: boolean;
  validation: { error: unknown; resolved: string | null } | null;
  isProject: (path: string) => boolean;
}): { status: DirectoryPathStatus; message: string | null } {
  if (typed === "") return { status: "empty", message: null };
  if (!isAbsolutePath(expanded)) {
    if (typed.startsWith("~") && home === null) {
      // `~` waits for the daemon's home, unless reading it failed.
      return homeUnavailable
        ? { status: "invalid", message: DIRECTORY_PATH_COPY.homeUnavailable }
        : { status: "checking", message: null };
    }
    return { status: "invalid", message: DIRECTORY_PATH_COPY.relative };
  }
  if (validation?.error)
    return { status: "invalid", message: browseErrorMessage(validation.error) };
  if (validation?.resolved) {
    return judgeListedFolder(typed, candidate, validation.resolved, home, isProject);
  }
  return { status: "checking", message: null };
}
