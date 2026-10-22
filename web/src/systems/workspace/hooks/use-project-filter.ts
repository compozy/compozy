import { useState } from "react";

import type { FilterFieldConfig } from "@compozy/ui";

import type { ProjectFilterValue } from "../lib/breadth-query";
import type { DataScope } from "../lib/destination";
import { projectFilterField } from "../lib/project-filter";

const NO_FILTER: readonly ProjectFilterValue[] = [];

export interface ProjectFilterModel {
  /** Shown only under All projects. */
  visible: boolean;
  /** Selected scopes; empty means every project plus User. Always empty outside All projects. */
  values: readonly ProjectFilterValue[];
  setValues: (next: readonly ProjectFilterValue[]) => void;
  clear: () => void;
  /** The toolbar field for the listing's filter strip; `null` when hidden. */
  field: FilterFieldConfig<string> | null;
}

/**
 * A listing's transient Project filter over the registered `projects` (switcher
 * order). It lives with the listing (never persisted) and resets whenever the
 * data scope enters or leaves All projects, so a narrowed aggregate never
 * carries into project mode or back.
 */
export function useProjectFilter(
  scope: DataScope | null,
  projects: ReadonlyArray<{ id: string; name: string }>
): ProjectFilterModel {
  const allProjects = scope?.kind === "all";
  const [state, setState] = useState({ allProjects, values: NO_FILTER });
  if (state.allProjects !== allProjects) {
    setState({ allProjects, values: NO_FILTER });
  }
  const values = allProjects && state.allProjects ? state.values : NO_FILTER;
  return {
    visible: allProjects,
    values,
    setValues: next => setState({ allProjects, values: [...new Set(next)] }),
    clear: () => setState({ allProjects, values: NO_FILTER }),
    field: allProjects ? projectFilterField(projects) : null,
  };
}
