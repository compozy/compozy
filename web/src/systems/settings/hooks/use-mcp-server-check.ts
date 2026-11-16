import { useState } from "react";

import { toast } from "@compozy/ui";

import { mcpDefinitionKey } from "../lib/mcp-management-target";
import { mcpPanelAnchor, type MCPPanelAnchor } from "../lib/mcp-panel-anchor";
import { deriveMCPAuthFilter, summarizeMCPStatus } from "../lib/mcp-status-view-model";
import type { SettingsMCPServerEntry } from "../types";
import { useCheckMCPServer } from "./use-mcp-runtime-mutations";

/** A server that is not working, open in the issue panel beside its row button. */
export interface MCPIssueTarget {
  entry: SettingsMCPServerEntry;
  anchor: MCPPanelAnchor;
}

/** Whether a Settings row reads as not working: its one status is destructive. */
export function isMCPServerNotWorking(entry: SettingsMCPServerEntry): boolean {
  return summarizeMCPStatus(entry).tone === "destructive";
}

/**
 * The Settings "Not working" panel and its Check again. The check answers with the definition's
 * new runtime status (merged into the listed row); a server that works now closes the panel, one
 * that still fails stays open and says so. Pending and result state belong to the server the
 * check was sent for, so a slow answer for one server never reaches another one's panel.
 */
export function useMCPServerCheck() {
  const [inspecting, setInspecting] = useState<MCPIssueTarget | null>(null);
  const [stillFailingKey, setStillFailingKey] = useState<string | null>(null);
  const mutation = useCheckMCPServer();
  const inspectingKey = inspecting ? mcpDefinitionKey(inspecting.entry) : null;
  const pendingKey =
    mutation.isPending && mutation.variables ? mcpDefinitionKey(mutation.variables.entry) : null;
  const checking = pendingKey !== null && pendingKey === inspectingKey;

  const close = () => {
    setInspecting(null);
    setStillFailingKey(null);
  };

  const check = () => {
    if (!inspecting || checking) return;
    // The row's own lens: a profile row carries its profile, and the daemon refuses one elsewhere.
    const filter = deriveMCPAuthFilter(inspecting.entry);
    if (!filter) {
      toast.error(`Couldn't check ${inspecting.entry.name}. Open it to see its definition.`);
      return;
    }
    const key = mcpDefinitionKey(inspecting.entry);
    setStillFailingKey(null);
    mutation.mutate(
      { entry: inspecting.entry, filter },
      {
        onSuccess: item => {
          setInspecting(current => {
            if (!current || mcpDefinitionKey(current.entry) !== key) return current;
            if (!isMCPServerNotWorking(item)) return null;
            return { ...current, entry: { ...current.entry, runtime_status: item.runtime_status } };
          });
          setStillFailingKey(isMCPServerNotWorking(item) ? key : null);
        },
        onError: error => toast.error(error.message),
      }
    );
  };

  return {
    inspecting,
    open: (entry: SettingsMCPServerEntry, trigger: HTMLElement) => {
      setStillFailingKey(null);
      setInspecting({ entry, anchor: mcpPanelAnchor(trigger) });
    },
    close,
    check,
    checking,
    stillFailing: stillFailingKey !== null && stillFailingKey === inspectingKey,
  };
}

export type MCPServerCheck = ReturnType<typeof useMCPServerCheck>;
