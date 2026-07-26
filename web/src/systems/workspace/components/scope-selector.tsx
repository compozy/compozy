import { Box, Globe } from "lucide-react";

import { PillGroup, cn } from "@agh/ui";

import { ScopeSelectorProvider } from "../contexts/scope-selector-context";
import {
  WorkspaceCommandSelect,
  type WorkspaceCommandSelectOption,
} from "./workspace-command-select";

export type ScopeSelectorScope = "global" | "workspace";

export interface ScopeSelectorProps {
  scope: ScopeSelectorScope;
  workspaceId: string | null | undefined;
  workspaces: ReadonlyArray<WorkspaceCommandSelectOption> | undefined;
  onScopeChange: (scope: ScopeSelectorScope) => void;
  onWorkspaceChange: (workspaceId: string) => void;
  ariaLabel?: string;
  className?: string;
  disabled?: boolean;
  testIdPrefix?: string;
  workspaceDisabled?: boolean;
}

export function ScopeSelector({
  scope,
  workspaceId,
  workspaces,
  onScopeChange,
  onWorkspaceChange,
  ariaLabel = "Scope",
  className,
  disabled,
  testIdPrefix = "scope",
  workspaceDisabled,
}: ScopeSelectorProps) {
  const selectGlobalScope = () => {
    onScopeChange("global");
  };

  const scopeSelectorContext = { selectGlobalScope };

  return (
    <ScopeSelectorProvider value={scopeSelectorContext}>
      <div className={cn("flex min-w-0 flex-wrap items-center gap-2", className)}>
        <PillGroup
          aria-label={ariaLabel}
          data-testid={`${testIdPrefix}-scope-group`}
          items={[
            {
              value: "global",
              label: (
                <span className="flex items-center gap-1.5">
                  <Globe aria-hidden="true" className="size-3" />
                  Global
                </span>
              ),
              disabled,
              testId: `${testIdPrefix}-scope-global`,
            },
            {
              value: "workspace",
              label: (
                <span className="flex items-center gap-1.5">
                  <Box aria-hidden="true" className="size-3" />
                  Workspace
                </span>
              ),
              disabled: disabled || workspaceDisabled,
              testId: `${testIdPrefix}-scope-workspace`,
            },
          ]}
          onChange={onScopeChange}
          size="md"
          value={scope}
        />
        {scope === "workspace" && !workspaceDisabled ? (
          <div className="min-w-40 w-auto max-w-xs shrink-0">
            <WorkspaceCommandSelect
              disabled={disabled}
              onChange={onWorkspaceChange}
              size="compact"
              testIdPrefix={`${testIdPrefix}-workspace`}
              triggerTestId={`${testIdPrefix}-workspace-select`}
              value={workspaceId ?? null}
              workspaces={workspaces}
            />
          </div>
        ) : null}
      </div>
    </ScopeSelectorProvider>
  );
}
