import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import type { DirectoryPickerSuggestion } from "@compozy/ui";

import { useDebouncedValue } from "@/hooks/use-debounced-input";
import {
  completionTarget,
  contractHome,
  expandHome,
  isAbsolutePath,
  isWindowsPath,
  normalizeDirectoryPath,
} from "../lib/directory-path";
import {
  DIRECTORY_PATH_COPY,
  directorySuggestions,
  judgeDirectoryPath,
  type DirectoryPathStatus,
} from "../lib/directory-path-field";
import { directoryBrowseOptions } from "../lib/query-options";
import type { FSBrowseResponse } from "../types";

/** Most completions the list shows; typing narrows the rest. */
const MAX_SUGGESTIONS = 50;
const VALIDATE_DELAY_MS = 150;

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

/**
 * The desktop shell's native folder dialog, when the shell offers one. It opens
 * at `defaultPath` and reports the chosen folder in the field's `~` spelling.
 */
function useNativeDirectoryDialog(
  defaultPath: string | null,
  home: string | null,
  onChoose: (value: string) => void
) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const dialogs = typeof window === "undefined" ? undefined : window.compozyShell?.dialogs;

  const open = (shell: NonNullable<typeof dialogs>) => {
    setPending(true);
    setError(null);
    void shell
      .pickDirectory(defaultPath ? { default_path: defaultPath } : {})
      .then(
        chosen => {
          if (chosen) onChoose(contractHome(chosen, home));
        },
        () => setError(DIRECTORY_PATH_COPY.browseFailed)
      )
      .then(() => setPending(false));
  };

  return {
    browse: dialogs ? () => open(dialogs) : undefined,
    pending,
    error,
    clearError: () => setError(null),
  };
}

/** Folder to list for completions and the name prefix to match, once typing settles. */
function completionQuery(settled: string) {
  const target = isAbsolutePath(settled) ? completionTarget(settled) : null;
  const showHidden = target?.prefix.startsWith(".") ?? false;
  return {
    target,
    query: { path: target?.dir, dirsOnly: true, ...(showHidden ? { showHidden } : {}) },
  };
}

/**
 * Completions for the current text. The listing follows the settled text, so it
 * serves the current text only while both name the same folder; a stale list
 * never completes newer typing. An empty field suggests the folders in home.
 */
function fieldSuggestions({
  typed,
  expanded,
  home,
  homeEntries,
  listedDir,
  listedEntries,
}: {
  typed: string;
  expanded: string;
  home: string | null;
  homeEntries: FSBrowseResponse["entries"] | undefined;
  listedDir: string | undefined;
  listedEntries: FSBrowseResponse["entries"] | undefined;
}): DirectoryPickerSuggestion[] {
  if (typed === "") {
    return directorySuggestions({
      entries: homeEntries,
      prefix: "",
      typed: "~/",
      windows: isWindowsPath(home ?? ""),
      limit: MAX_SUGGESTIONS,
    });
  }
  const current = isAbsolutePath(expanded) ? completionTarget(expanded) : null;
  if (current === null || current.dir !== listedDir) return [];
  return directorySuggestions({
    entries: listedEntries,
    prefix: current.prefix,
    typed,
    windows: isWindowsPath(expanded),
    limit: MAX_SUGGESTIONS,
  });
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
  // The unscoped listing names the daemon's home and doubles as the empty-field suggestions.
  const homeListing = useQuery(directoryBrowseOptions({ dirsOnly: true }, enabled));
  const home = homeListing.data?.home ?? null;

  const typed = value.trim();
  // Listing and validation follow the text once typing pauses.
  const settledTyped = useDebouncedValue(typed, VALIDATE_DELAY_MS);
  const settled = expandHome(settledTyped, home);

  const completion = completionQuery(settled);
  const listing = useQuery(
    directoryBrowseOptions(completion.query, enabled && completion.target !== null)
  );

  const candidate = isAbsolutePath(settled) ? normalizeDirectoryPath(settled) : "";
  const validation = useQuery(
    directoryBrowseOptions({ path: candidate, dirsOnly: true }, enabled && candidate !== "")
  );

  const expanded = expandHome(typed, home);
  const suggestions = fieldSuggestions({
    typed,
    expanded,
    home,
    homeEntries: homeListing.data?.entries,
    listedDir: completion.target?.dir,
    listedEntries: listing.data?.entries,
  });

  const isSettled = settledTyped === typed;
  const { status, message } = judgeDirectoryPath({
    typed,
    expanded,
    candidate,
    home,
    homeUnavailable: homeListing.isError,
    validation: isSettled
      ? { error: validation.error, resolved: validation.data?.path ?? null }
      : null,
    isProject,
  });
  const valid = status === "valid";
  // Browse opens at the typed folder whenever it exists, even if it cannot be added.
  const folderExists = isSettled && validation.data !== undefined;
  const dialog = useNativeDirectoryDialog(folderExists ? candidate : home, home, onValueChange);

  return {
    value,
    onValueChange: next => {
      dialog.clearError();
      onValueChange(next);
    },
    suggestions,
    status,
    path: valid ? candidate : null,
    message: message ?? dialog.error,
    browse: dialog.browse,
    browsePending: dialog.pending,
  };
}
