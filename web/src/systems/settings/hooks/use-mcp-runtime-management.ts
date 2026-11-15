import { useEffect, useState } from "react";

import { toast } from "@compozy/ui";

import { useOpenFileInEditor } from "@/systems/os";

import { SettingsApiError } from "../adapters/settings-api";
import { mcpDefinitionKey } from "../lib/mcp-management-target";
import {
  MCP_FILTER_ALL,
  mcpApprovalReason,
  mcpApprovalTarget,
  type MCPApprovalNote,
  type MCPCopyFailure,
} from "../lib/mcp-runtime-view-model";
import type {
  RuntimeMCPImportRequest,
  RuntimeMCPInventoryFilter,
  RuntimeMCPServer,
  RuntimeMCPSnapshot,
  SettingsMCPServerEntry,
} from "../types";
import { useAllowMCPServer, useImportRuntimeMCPServer } from "./use-mcp-runtime-mutations";
import { useRuntimeMCPInventory } from "./use-settings-collections";

const DEFINITION_CHANGED = "mcp_approval_definition_changed";
/** How long the row a copy created stays marked before it reads as any other row. */
export const MCP_COPY_HIGHLIGHT_MS = 2_400;

interface UseMCPRuntimeManagementOptions {
  enabled: boolean;
  filter: RuntimeMCPInventoryFilter;
}

export interface MCPCopyTarget {
  snapshot: RuntimeMCPSnapshot;
  server: RuntimeMCPServer;
}

function failureOf(error: unknown): MCPCopyFailure {
  if (error instanceof SettingsApiError) return { code: error.code, message: error.message };
  return { message: error instanceof Error ? error.message : "Couldn't copy the server." };
}

function withoutKey<T>(record: Record<string, T>, key: string): Record<string, T> {
  const next = { ...record };
  delete next[key];
  return next;
}

/**
 * The runtime half of MCP management: every runtime's own servers for the lens, the runtime
 * filter, Allow for waiting project servers, and Copy to CompozyOS. Approval, copy and the runtime
 * switch reread the CompozyOS list and the runtime groups through the shared MCP root key.
 */
export function useMCPRuntimeManagement({ enabled, filter }: UseMCPRuntimeManagementOptions) {
  const inventory = useRuntimeMCPInventory(filter, { enabled });
  const [runtimeFilter, setRuntimeFilter] = useState<string>(MCP_FILTER_ALL);
  const [approvalNotes, setApprovalNotes] = useState<Record<string, MCPApprovalNote>>({});
  const [copying, setCopying] = useState<MCPCopyTarget | null>(null);
  const [copyFailure, setCopyFailure] = useState<MCPCopyFailure | null>(null);
  const [highlightedKey, setHighlightedKey] = useState<string | null>(null);
  const [allowingKey, setAllowingKey] = useState<string | null>(null);
  const allowMutation = useAllowMCPServer();
  const importMutation = useImportRuntimeMCPServer();
  const showFile = useOpenFileInEditor();

  useEffect(() => {
    if (highlightedKey === null) return;
    const timer = setTimeout(() => setHighlightedKey(null), MCP_COPY_HIGHLIGHT_MS);
    return () => clearTimeout(timer);
  }, [highlightedKey]);

  const allow = (entry: SettingsMCPServerEntry) => {
    const target = mcpApprovalTarget(entry);
    if (!target || allowMutation.isPending) return;
    const key = mcpDefinitionKey(entry);
    setAllowingKey(key);
    allowMutation.mutate(
      {
        name: entry.name,
        body: { ...target, definition_digest: entry.approval.definition_digest },
      },
      {
        onSuccess: () => {
          setApprovalNotes(current => withoutKey(current, key));
          toast.success(
            `${entry.name} allowed. Sessions in this project can use it from their next message.`
          );
        },
        onError: error =>
          setApprovalNotes(current => ({
            ...current,
            [key]: {
              reason:
                error instanceof SettingsApiError && error.code === DEFINITION_CHANGED
                  ? "changed"
                  : (current[key]?.reason ?? mcpApprovalReason(entry)),
              error: error.message,
            },
          })),
        onSettled: () => setAllowingKey(null),
      }
    );
  };

  const openCopy = (snapshot: RuntimeMCPSnapshot, server: RuntimeMCPServer) => {
    importMutation.reset();
    setCopyFailure(null);
    setCopying({ snapshot, server });
  };

  const closeCopy = () => {
    if (!importMutation.isPending) setCopying(null);
  };

  const submitCopy = (request: RuntimeMCPImportRequest) => {
    if (!copying || importMutation.isPending) return;
    const { server } = copying;
    setCopyFailure(null);
    importMutation.mutate(
      { runtime: server.runtime, name: server.name, body: request },
      {
        onSuccess: result => {
          const name = result.mcp_server.name;
          setCopying(null);
          setHighlightedKey(mcpDefinitionKey(result.mcp_server));
          if (result.partial) {
            toast.warning(
              result.warnings?.[0] ??
                `${name} was copied, but CompozyOS couldn't confirm its secrets were cleaned up. Check Settings › MCP servers › ${name}.`
            );
          } else {
            toast.success(`${name} copied. Sessions can use it from their next message.`);
          }
        },
        onError: error => setCopyFailure(failureOf(error)),
      }
    );
  };

  return {
    inventory,
    snapshots: inventory.data?.runtimes ?? [],
    runtimeFilter,
    setRuntimeFilter,
    allow,
    allowingKey: allowingKey ?? undefined,
    approvalNotes,
    copying,
    openCopy,
    closeCopy,
    submitCopy,
    copySubmitting: importMutation.isPending,
    copyFailure,
    highlightedKey: highlightedKey ?? undefined,
    showFile,
  };
}

export type MCPRuntimeManagement = ReturnType<typeof useMCPRuntimeManagement>;
