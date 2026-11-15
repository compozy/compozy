import { PillGroup } from "@compozy/ui";

import type { MCPRuntimeFilterOption } from "../lib/mcp-runtime-view-model";
import { ProviderLogo } from "./provider-logo";

export interface MCPRuntimeFilterProps {
  options: readonly MCPRuntimeFilterOption[];
  value: string;
  onChange: (value: string) => void;
}

/**
 * "All · CompozyOS · <runtime> …" with counts. Only runtimes with servers or an unreadable file are
 * offered (the options come from `runtimeFilterOptions`); arrow keys move along the group.
 */
export function MCPRuntimeFilter({ options, value, onChange }: MCPRuntimeFilterProps) {
  return (
    <PillGroup
      aria-label="Filter by where the server comes from"
      data-testid="settings-mcp-runtime-filter"
      items={options.map(option => ({
        value: option.value,
        badge: option.count,
        testId: `settings-mcp-runtime-filter-${option.value}`,
        label: option.runtime ? (
          <span className="inline-flex items-center gap-1.5">
            <ProviderLogo className="size-3.5" provider={option.runtime} />
            {option.label}
          </span>
        ) : (
          option.label
        ),
      }))}
      onChange={onChange}
      size="sm"
      value={value}
    />
  );
}
