import { useState } from "react";
import { ChevronsUpDown, Home, Plus } from "lucide-react";

import {
  cn,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  CommandSeparator,
} from "@agh/ui";

import { useUserHomeDir } from "../hooks/use-user-home-dir";
import { useScopeSelectorContext } from "../hooks/use-scope-selector-context";
import { isHomeWorkspace, splitHomeWorkspace } from "../lib/home-workspace";

function workspaceInitial(name: string): string {
  return name.charAt(0).toUpperCase() || "·";
}

export interface WorkspaceCommandSelectOption {
  id: string;
  name: string;
  root_dir?: string | null;
}

export interface WorkspaceCommandSelectProps {
  workspaces: ReadonlyArray<WorkspaceCommandSelectOption> | undefined;
  value: string | null;
  onChange: (id: string) => void;
  onAddWorkspace?: () => void;
  disabled?: boolean;
  className?: string;
  triggerTestId?: string;
  testIdPrefix?: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  size?: "default" | "compact";
}

export function WorkspaceCommandSelect({
  workspaces,
  value,
  onChange,
  onAddWorkspace,
  disabled,
  className,
  triggerTestId = "workspace-switcher",
  testIdPrefix = "workspace-command",
  open: openProp,
  onOpenChange,
  size = "default",
}: WorkspaceCommandSelectProps) {
  const userHomeDir = useUserHomeDir();
  const scopeSelector = useScopeSelectorContext();
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const open = openProp ?? uncontrolledOpen;
  const setOpen = onOpenChange ?? setUncontrolledOpen;
  const selected = workspaces?.find(workspace => workspace.id === value) ?? null;
  const { homeWorkspace, projectWorkspaces } = splitHomeWorkspace(workspaces, userHomeDir);
  const orderedWorkspaces = homeWorkspace
    ? [homeWorkspace, ...projectWorkspaces]
    : projectWorkspaces;
  const selectedIsHome = Boolean(selected && isHomeWorkspace(selected, userHomeDir));
  const label = selected?.name ?? "No workspace";
  const hasWorkspaces = (workspaces?.length ?? 0) > 0;
  const isDisabled = disabled || !hasWorkspaces;

  const handleSelect = (workspace: WorkspaceCommandSelectOption) => {
    if (scopeSelector && isHomeWorkspace(workspace, userHomeDir)) {
      scopeSelector.selectGlobalScope();
      setOpen(false);
      return;
    }
    onChange(workspace.id);
    setOpen(false);
  };

  const handleAddWorkspace = () => {
    onAddWorkspace?.();
    setOpen(false);
  };

  return (
    <CommandSelect open={open} onOpenChange={next => setOpen(next)}>
      <CommandSelectTrigger
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={
          hasWorkspaces
            ? selectedIsHome
              ? `Home workspace: ${label}`
              : `Workspace: ${label}`
            : "No workspace"
        }
        data-size={size}
        data-testid={triggerTestId}
        disabled={isDisabled}
        selected={Boolean(selected)}
        className={cn(
          size === "compact"
            ? // Ghost, not a button: this sits inline beside the borderless scope
              // PillGroup, and a bordered filled trigger reads as a second, taller
              // control on that line. `border-transparent` rather than `border-0`
              // — the border is inside the 28 px box, so dropping it would shrink
              // the trigger to 26 px and re-break the alignment.
              "h-[calc(var(--height-pill-group-segment-md)+2*var(--space-pill-group-track-padding))] min-w-0 gap-1.5 rounded-md border-transparent bg-transparent px-(--space-pill-group-segment-md-x) py-0 text-subtle shadow-none hover:bg-row-hover hover:text-fg-strong focus-visible:border-transparent focus-visible:shadow-focus-ring [&>svg:last-child]:hidden"
            : "h-12 w-full gap-2.5 border-0 bg-transparent px-2 py-0 shadow-none hover:bg-hover focus-visible:border-0 focus-visible:shadow-none [&>svg:last-child]:hidden",
          className
        )}
      >
        <span
          className={cn(
            "flex min-w-0 flex-1 items-center text-left",
            size === "compact" ? "gap-1.5" : "gap-2.5"
          )}
        >
          <span
            aria-hidden="true"
            data-testid="workspace-switcher-avatar"
            data-home={selectedIsHome ? "true" : undefined}
            className={cn(
              "inline-flex shrink-0 items-center justify-center rounded-sm font-mono text-eyebrow font-medium tracking-mono text-fg",
              // A ghost trigger holds no filled sub-surface.
              size === "compact" ? "size-4 bg-canvas-tint" : "size-button-icon-xs bg-elevated"
            )}
          >
            {selectedIsHome ? (
              <Home aria-hidden="true" className={size === "compact" ? "size-3" : "size-3.5"} />
            ) : (
              workspaceInitial(label)
            )}
          </span>
          <span
            data-testid="workspace-switcher-name"
            className={cn(
              "min-w-0 flex-1 truncate font-medium text-fg",
              size === "compact"
                ? "text-form-label tracking-eyebrow"
                : "text-small-body tracking-normal"
            )}
          >
            {label}
          </span>
          <ChevronsUpDown
            aria-hidden="true"
            data-testid="workspace-switcher-chevron"
            className="size-3 shrink-0 text-subtle"
          />
        </span>
      </CommandSelectTrigger>
      <CommandSelectShell
        className={cn(size === "compact" ? "min-w-56" : "min-w-64")}
        inputPlaceholder="Search workspaces..."
        inputProps={{ "data-testid": `${testIdPrefix}-input` }}
      >
        <CommandList>
          <CommandEmpty data-testid={`${testIdPrefix}-empty`}>
            No workspaces match your search.
          </CommandEmpty>
          <CommandSelectGroup heading="Workspaces" data-testid={`${testIdPrefix}-group`}>
            {orderedWorkspaces.map(workspace => {
              const isActive = workspace.id === value;
              const isHome = isHomeWorkspace(workspace, userHomeDir);
              return (
                <CommandItem
                  key={workspace.id}
                  value={isHome ? `home workspace ${workspace.name}` : workspace.name}
                  onSelect={() => handleSelect(workspace)}
                  data-checked={isActive ? "true" : "false"}
                  data-home={isHome ? "true" : undefined}
                  data-testid={`${testIdPrefix}-item-${workspace.id}`}
                >
                  <span
                    aria-hidden="true"
                    data-home={isHome ? "true" : undefined}
                    data-testid={`${testIdPrefix}-item-avatar-${workspace.id}`}
                    className="inline-flex size-button-icon-xs shrink-0 items-center justify-center rounded-sm bg-elevated font-mono text-eyebrow font-medium tracking-mono text-fg"
                  >
                    {isHome ? (
                      <Home aria-hidden="true" className="size-3.5" />
                    ) : (
                      workspaceInitial(workspace.name)
                    )}
                  </span>
                  <span className="flex min-w-0 flex-1 items-center gap-2">
                    <span className="truncate text-small-body text-fg">{workspace.name}</span>
                    {isHome ? (
                      <span className="shrink-0 text-badge text-muted">Home workspace</span>
                    ) : null}
                  </span>
                </CommandItem>
              );
            })}
          </CommandSelectGroup>
          {onAddWorkspace ? (
            <>
              <CommandSeparator />
              <CommandSelectGroup>
                <CommandItem
                  value="add workspace"
                  onSelect={handleAddWorkspace}
                  data-testid={`${testIdPrefix}-add`}
                >
                  <Plus aria-hidden="true" className="size-4 shrink-0 text-subtle" />
                  <span className="text-small-body text-fg">Add workspace</span>
                </CommandItem>
              </CommandSelectGroup>
            </>
          ) : null}
        </CommandList>
      </CommandSelectShell>
    </CommandSelect>
  );
}
