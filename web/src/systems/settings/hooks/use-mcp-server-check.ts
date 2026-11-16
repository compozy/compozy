import { useState } from "react";

import { toast } from "@compozy/ui";

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
 * new runtime status (the list cache takes it in place); a server that works now closes the
 * panel, one that still fails stays open and says so.
 */
export function useMCPServerCheck() {
  const [inspecting, setInspecting] = useState<MCPIssueTarget | null>(null);
  const [stillFailing, setStillFailing] = useState(false);
  const mutation = useCheckMCPServer();

  const close = () => {
    setInspecting(null);
    setStillFailing(false);
  };

  const check = () => {
    if (!inspecting || mutation.isPending) return;
    const filter = deriveMCPAuthFilter(inspecting.entry);
    if (!filter) {
      toast.error(`Couldn't check ${inspecting.entry.name}. Open it to see its definition.`);
      return;
    }
    setStillFailing(false);
    mutation.mutate(
      { entry: inspecting.entry, filter },
      {
        onSuccess: item => {
          if (!isMCPServerNotWorking(item)) {
            close();
            return;
          }
          setInspecting(current => (current ? { ...current, entry: item } : current));
          setStillFailing(true);
        },
        onError: error => toast.error(error.message),
      }
    );
  };

  return {
    inspecting,
    open: (entry: SettingsMCPServerEntry, trigger: HTMLElement) => {
      setStillFailing(false);
      setInspecting({ entry, anchor: mcpPanelAnchor(trigger) });
    },
    close,
    check,
    checking: mutation.isPending,
    stillFailing,
  };
}

export type MCPServerCheck = ReturnType<typeof useMCPServerCheck>;
