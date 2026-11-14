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

  // The state's fixture rebuilds `copied` on every render, so the effect keys on the name and the
  // toast carries a fixed id: re-renders and strict-mode replays update one toast, never stack it.
  const copiedName = copied?.name;
  useEffect(() => {
    if (!copiedName) return;
    toast.success(`${copiedName} copied. Sessions can use it from their next message.`, {
      id: `mcp-copied-${copiedName}`,
    });
  }, [copiedName]);

  return [copying, setCopying] as const;
}
