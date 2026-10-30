import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import type { DirectoryPickerSuggestion } from "@compozy/ui";

import { useDebouncedValue } from "@/hooks/use-debounced-input";
import { OnboardingApiError } from "../adapters/onboarding-api";
import {
  completionTarget,
  contractHome,
  expandHome,
  isAbsolutePath,
  normalizeDirectoryPath,
  suggestionValue,
} from "../lib/directory-path";
import { directoryBrowseOptions } from "../lib/query-options";

/** Most completions the list shows; typing narrows the rest. */
const MAX_SUGGESTIONS = 50;
const VALIDATE_DELAY_MS = 150;

/** `incomplete`: the text is a place to browse from (`~/`), not yet a choice. */
export type DirectoryPathStatus = "empty" | "incomplete" | "checking" | "invalid" | "valid";

export const DIRECTORY_PATH_COPY = {
  relative: "Use an absolute path, like ~/code/app.",
  missing: "No folder at this path.",
  notDirectory: "This path is a file, not a folder.",
  unreadable: "This folder can't be read. Check its permissions.",
  alreadyProject: "This folder is already a project.",
  home: "Your home folder can't be a project. Choose a folder inside it.",
  browseFailed: "Couldn't open the folder dialog.",
} as const;

interface UseDirectoryPathFieldOptions {
  value: string;
  onValueChange: (value: string) => void;
  /** Whether a normalized absolute path is already a registered project. */
  isProject?: (path: string) => boolean;
  enabled?: boolean;
}

export interface DirectoryPathField {
  value: string;
  onValueChange: (value: string) => void;
  suggestions: DirectoryPickerSuggestion[];
  status: DirectoryPathStatus;
  /** The validated absolute folder, set only when `status` is `valid`. */
  path: string | null;
  /** Inline validation for a non-empty value; `null` while checking or valid. */
  message: string | null;
  /** Opens the native folder dialog; absent outside the desktop shell. */
  browse?: () => void;
  browsePending: boolean;
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

/**
 * State for a directory picker field: `~` expansion, completions listed from
 * the daemon's directory endpoint, inline validation of the typed folder, and
 * the desktop shell's native folder dialog when one is available.
 */
export function useDirectoryPathField({
  value,
  onValueChange,
  isProject = () => false,
  enabled = true,
}: UseDirectoryPathFieldOptions): DirectoryPathField {
  const [browsePending, setBrowsePending] = useState(false);
  const [browseError, setBrowseError] = useState<string | null>(null);

  // The unscoped listing names the daemon's home and doubles as the empty-field suggestions.
  const homeListing = useQuery(directoryBrowseOptions({ dirsOnly: true }, enabled));
  const home = homeListing.data?.home ?? null;

  const typed = value.trim();
  const expanded = expandHome(typed, home);
  // Listing and validation follow the text once typing pauses.
  const settledTyped = useDebouncedValue(typed, VALIDATE_DELAY_MS);
  const settled = expandHome(settledTyped, home);
  const isSettled = settledTyped === typed;

  const target = isAbsolutePath(settled) ? completionTarget(settled) : null;
  const showHidden = target?.prefix.startsWith(".") ?? false;
  const listing = useQuery(
    directoryBrowseOptions(
      { path: target?.dir, dirsOnly: true, ...(showHidden ? { showHidden } : {}) },
      enabled && target !== null
    )
  );

  const candidate = isAbsolutePath(settled) ? normalizeDirectoryPath(settled) : "";
  const validation = useQuery(
    directoryBrowseOptions({ path: candidate, dirsOnly: true }, enabled && candidate !== "")
  );

  const entries = typed === "" ? homeListing.data?.entries : target ? listing.data?.entries : [];
  const prefix = (target?.prefix ?? "").toLowerCase();
  const suggestions = (entries ?? [])
    // The folder already typed in full is not a completion; `/` drills into it.
    .filter(entry => {
      const name = entry.name.toLowerCase();
      return entry.is_dir && name.startsWith(prefix) && name !== prefix;
    })
    .slice(0, MAX_SUGGESTIONS)
    .map(entry => ({
      value: suggestionValue(typed === "" ? "~/" : settledTyped, entry.name),
      label: entry.name,
    }));

  let status: DirectoryPathStatus = "checking";
  let message: string | null = null;
  if (typed === "") {
    status = "empty";
  } else if (!isAbsolutePath(expanded)) {
    // `~` waits for the daemon's home before it can be judged.
    if (!(typed.startsWith("~") && home === null)) {
      status = "invalid";
      message = DIRECTORY_PATH_COPY.relative;
    }
  } else if (isSettled && validation.error) {
    status = "invalid";
    message = browseErrorMessage(validation.error);
  } else if (isSettled && validation.data) {
    const resolved = validation.data.path;
    if (home !== null && (candidate === normalizeDirectoryPath(home) || resolved === home)) {
      // `~/` is where browsing starts; only `~` itself reads as choosing home.
      const browsing = /[\\/]$/.test(typed);
      status = browsing ? "incomplete" : "invalid";
      message = browsing ? null : DIRECTORY_PATH_COPY.home;
    } else if (isProject(candidate) || isProject(resolved)) {
      status = "invalid";
      message = DIRECTORY_PATH_COPY.alreadyProject;
    } else {
      status = "valid";
    }
  }

  const pickDirectory = typeof window === "undefined" ? undefined : window.compozyShell?.dialogs;
  const browse = pickDirectory
    ? async () => {
        setBrowsePending(true);
        setBrowseError(null);
        try {
          const chosen = await pickDirectory.pickDirectory(
            status === "valid" ? { default_path: candidate } : home ? { default_path: home } : {}
          );
          if (chosen) onValueChange(contractHome(chosen, home));
        } catch {
          setBrowseError(DIRECTORY_PATH_COPY.browseFailed);
        } finally {
          setBrowsePending(false);
        }
      }
    : undefined;

  return {
    value,
    onValueChange: next => {
      setBrowseError(null);
      onValueChange(next);
    },
    suggestions,
    status,
    path: status === "valid" ? candidate : null,
    message: message ?? browseError,
    browse: browse ? () => void browse() : undefined,
    browsePending,
  };
}
