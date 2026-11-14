import { useEffect, useState } from "react";

import { toast } from "@compozy/ui";

import type {
  RuntimeMCPServer,
  RuntimeMCPSnapshot,
  SettingsMCPServerEntry,
} from "@/systems/settings/types";

import type { CopyDialogCase } from "../mcp-page-fixtures";

/** The copy dialog state: opened on a secret-carrying server for the `copy` states, toast once copied. */
export function useCopyDialog(
  runtimes: readonly RuntimeMCPSnapshot[],
  copy: CopyDialogCase | undefined,
  copied: SettingsMCPServerEntry | undefined
) {
  const firstGroup = runtimes[0];
  const initialCopy = copy
    ? firstGroup?.servers.find(
        server => server.copyable && server.env_keys.length + server.header_keys.length > 0
      )
    : undefined;
  const [copying, setCopying] = useState<{
    snapshot: RuntimeMCPSnapshot;
    server: RuntimeMCPServer;
  } | null>(firstGroup && initialCopy ? { snapshot: firstGroup, server: initialCopy } : null);

  useEffect(() => {
    if (!copied) return;
    toast.success(`${copied.name} copied. Sessions can use it from their next message.`);
  }, [copied]);

  return [copying, setCopying] as const;
}
