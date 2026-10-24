import type { ReactNode } from "react";

import { CommandEmpty, CommandItem, CommandList, CommandSelectGroup, Eyebrow } from "@compozy/ui";

import { AgentCommandListSkeleton } from "./agent-command-list-skeleton";
import { AgentIcon } from "./agent-icon";
import {
  buildAgentCategoryTree,
  formatCategoryLabel,
  type AgentCategoryFolderNode,
  type AgentCategoryNode,
} from "../lib/agent-category";
import type { AgentPayload } from "../types";

const ROOT_GROUP_HEADING = "Agents";

interface AgentCategoryGroup {
  key: string;
  heading: string;
  agents: AgentPayload[];
}

function collectGroups(nodes: AgentCategoryNode[]): AgentCategoryGroup[] {
  const groups: AgentCategoryGroup[] = [];
  const rootAgents: AgentPayload[] = [];

  const visitFolder = (folder: AgentCategoryFolderNode) => {
    const direct: AgentPayload[] = [];
    for (const child of folder.children) {
      if (child.kind === "leaf") direct.push(child.agent);
      else visitFolder(child);
    }
    if (direct.length > 0) {
      groups.push({
        key: `category:${folder.segments.join("/")}`,
        heading: formatCategoryLabel(folder.segments),
        agents: direct,
      });
    }
  };

  for (const node of nodes) {
    if (node.kind === "folder") visitFolder(node);
    else rootAgents.push(node.agent);
  }
  if (rootAgents.length > 0) {
    groups.unshift({ key: "agents:root", heading: ROOT_GROUP_HEADING, agents: rootAgents });
  }
  return groups;
}

export interface AgentCommandListProps {
  agents: AgentPayload[];
  isSelected: (agent: AgentPayload) => boolean;
  onSelect: (agent: AgentPayload) => void;
  emptyState?: ReactNode;
  itemTestId?: (agent: AgentPayload) => string;
  /** Items rendered above the catalog groups, inside the same command list. */
  leadingItems?: ReactNode;
  /**
   * The catalog has not answered: row-shaped placeholders stand where the groups will be, and
   * the list never claims that nothing matches. Agents already in hand are listed as usual.
   */
  loading?: boolean;
}

export function AgentCommandList({
  agents,
  isSelected,
  onSelect,
  emptyState = "No agents match your search.",
  itemTestId,
  leadingItems,
  loading = false,
}: AgentCommandListProps) {
  const groups = collectGroups(buildAgentCategoryTree(agents));
  const awaitingCatalog = loading && groups.length === 0;

  return (
    <CommandList>
      {awaitingCatalog ? null : (
        <CommandEmpty data-testid="agent-command-empty">{emptyState}</CommandEmpty>
      )}
      {leadingItems}
      {awaitingCatalog ? <AgentCommandListSkeleton /> : null}
      {groups.map(group => (
        <CommandSelectGroup
          key={group.key}
          heading={group.heading}
          data-testid={`agent-command-group-${group.key}`}
        >
          {group.agents.map(agent => {
            const selected = isSelected(agent);
            const categoryLabel = formatCategoryLabel(agent.category_path);
            return (
              <CommandItem
                key={agent.name}
                value={agentSearchKey(agent, categoryLabel)}
                onSelect={() => onSelect(agent)}
                data-checked={selected ? "true" : "false"}
                data-testid={itemTestId ? itemTestId(agent) : `agent-command-item-${agent.name}`}
              >
                <div className="flex min-w-0 flex-1 items-center gap-2">
                  <AgentIcon
                    className="shrink-0"
                    provider={agent.provider}
                    size="xs"
                    tone="muted"
                  />
                  <span className="truncate text-small-body text-fg">{agent.name}</span>
                  <Eyebrow
                    className="text-muted"
                    data-testid={`agent-command-provider-${agent.name}`}
                  >
                    {agent.provider}
                  </Eyebrow>
                  {categoryLabel ? (
                    <Eyebrow
                      className="text-muted ml-auto truncate"
                      data-testid={`agent-command-category-${agent.name}`}
                    >
                      {categoryLabel}
                    </Eyebrow>
                  ) : null}
                </div>
              </CommandItem>
            );
          })}
        </CommandSelectGroup>
      ))}
    </CommandList>
  );
}

function agentSearchKey(agent: AgentPayload, categoryLabel: string): string {
  const segments = [agent.name, agent.provider];
  if (categoryLabel) segments.push(categoryLabel);
  return segments.join(" ");
}
