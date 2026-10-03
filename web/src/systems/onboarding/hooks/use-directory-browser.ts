import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { directoryBrowseOptions } from "../lib/query-options";
import type { DirectoryBrowseQuery } from "../types";

interface BrowseLocation {
  path: string;
  parent: string | null;
  home: string | null;
  roots: string[];
}

export function useDirectoryBrowser(query: Omit<DirectoryBrowseQuery, "path">, enabled = true) {
  const [location, setLocation] = useState<BrowseLocation>({
    path: "",
    parent: null,
    home: null,
    roots: [],
  });
  const browse = useQuery(
    directoryBrowseOptions({ ...query, path: location.path || undefined }, enabled)
  );
  const data = browse.data;
  const parent = data ? (data.parent ?? null) : location.parent;
  const home = data?.home ?? location.home;
  const roots = data?.roots ?? location.roots;

  const navigateTo = (path: string) => {
    setLocation({
      path,
      // The listing proves a child's parent even if that child's read fails.
      parent: data?.entries.some(entry => entry.path === path) ? data.path : null,
      home,
      roots,
    });
  };

  return {
    ...browse,
    currentPath: data?.path ?? location.path,
    parent,
    home,
    roots,
    navigateTo,
    goToParent: () => {
      if (parent) navigateTo(parent);
    },
    goHome: () => {
      if (home) navigateTo(home);
    },
  };
}
