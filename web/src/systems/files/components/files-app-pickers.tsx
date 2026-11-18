import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@compozy/ui";
import { FolderGit2, GitBranch } from "@compozy/ui/icons";

import type { WorkspacePayload } from "@/systems/workspace";

import type { FilesAppRootOption } from "../lib/files-app-roots";

const CHECKOUT_VALUE = "__checkout__";

/** The Files window's project switcher (US-002.AC-3): live projects only. */
export function FilesProjectSwitcher({
  workspaceId,
  workspaceName,
  projects,
  onSwitch,
}: {
  workspaceId: string;
  /** Shown when the current project left the list (archived). */
  workspaceName: string;
  projects: readonly WorkspacePayload[];
  onSwitch: (workspaceId: string) => void;
}) {
  return (
    <Select
      value={workspaceId}
      onValueChange={next => {
        if (typeof next === "string" && next !== "" && next !== workspaceId) onSwitch(next);
      }}
    >
      <SelectTrigger
        size="sm"
        aria-label="Project"
        className="min-w-0 shrink"
        data-testid="files-project-switcher"
      >
        <FolderGit2 aria-hidden className="size-3.5 text-subtle" />
        <SelectValue className="min-w-0">
          <span className="min-w-0 truncate">{workspaceName}</span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {projects.map(project => (
          <SelectItem
            key={project.id}
            value={project.id}
            data-testid={`files-project-option-${project.id}`}
          >
            {project.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** The root picker: the project checkout plus every ready worktree, with its branch. */
export function FilesRootPicker({
  options,
  worktreeId,
  onSelect,
}: {
  options: readonly FilesAppRootOption[];
  worktreeId: string | null;
  onSelect: (worktreeId: string | null) => void;
}) {
  const value = worktreeId ?? CHECKOUT_VALUE;
  const current = options.find(option => (option.worktreeId ?? CHECKOUT_VALUE) === value);
  return (
    <Select
      value={value}
      onValueChange={next => {
        if (typeof next !== "string" || next === value) return;
        onSelect(next === CHECKOUT_VALUE ? null : next);
      }}
    >
      <SelectTrigger
        size="sm"
        aria-label="Root"
        className="min-w-0 shrink-[2]"
        data-testid="files-root-picker"
      >
        <GitBranch aria-hidden className="size-3.5 text-subtle" />
        <SelectValue className="min-w-0">
          <span className="min-w-0 truncate">{current?.label ?? "Worktree unavailable"}</span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent className="min-w-64">
        {options.map(option => {
          const optionValue = option.worktreeId ?? CHECKOUT_VALUE;
          return (
            <SelectItem
              key={optionValue}
              value={optionValue}
              data-testid={`files-root-option-${optionValue}`}
            >
              <span className="truncate">{option.label}</span>
              {option.branch ? (
                <span className="truncate font-mono text-xs text-muted-foreground">
                  {option.branch}
                </span>
              ) : null}
            </SelectItem>
          );
        })}
      </SelectContent>
    </Select>
  );
}
