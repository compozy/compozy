import { useState } from "react";

import { DetailInspector, cn } from "@compozy/ui";

import {
  deriveFileReads,
  type InspectorFileEntry,
  type ThreadMessageState,
} from "./session-inspector.logic";
import { SessionInspectorMemorySection } from "./session-inspector-memory";
import {
  SessionInspectorFilesSection,
  SessionInspectorUsageSection,
} from "./session-inspector-sections";
import type { InspectorMemoryState, InspectorUsage } from "./session-inspector-types";
import { SessionVaultPanel, type VaultSecret } from "@/systems/vault";

import {
  SESSION_INSPECTOR_TAB_TESTIDS,
  SESSION_INSPECTOR_TABS,
  isInspectorTabId,
  type InspectorTabId,
} from "../lib/session-inspector-tabs";
import { useSessionInspectorState } from "../hooks/use-session-inspector-state";

import { SessionCallsSection } from "./session-calls-section";

export type {
  InspectorMemoryState,
  InspectorSessionLedger,
  InspectorUsage,
} from "./session-inspector-types";

const EMPTY_VAULT_SECRETS: readonly VaultSecret[] = [];
const EMPTY_MEMORY_STATE: InspectorMemoryState = Object.freeze({});
const DEFAULT_INSPECTOR_TAB: InspectorTabId = "usage";

export interface SessionInspectorProps {
  messages: readonly ThreadMessageState[];
  sessionId?: string;
  liveDataEnabled?: boolean;
  usage?: InspectorUsage | null;
  memory?: InspectorMemoryState;
  vaultSecrets?: readonly VaultSecret[];
  vaultIsLoading?: boolean;
  vaultError?: Error | null;
  files?: InspectorFileEntry[];
  drawerOpen?: boolean;
  onDrawerOpenChange?: (open: boolean) => void;
  className?: string;
}

interface InspectorTabRendererProps {
  activeTab: InspectorTabId;
  usage: InspectorUsage | null | undefined;
  memory: InspectorMemoryState;
  sessionId?: string;
  liveDataEnabled: boolean;
  vaultSecrets: readonly VaultSecret[];
  vaultIsLoading: boolean;
  vaultError: Error | null;
  files: InspectorFileEntry[];
}

function InspectorTabRenderer(props: InspectorTabRendererProps) {
  switch (props.activeTab) {
    case "usage":
      return <SessionInspectorUsageSection usage={props.usage} />;
    case "memory":
      return <SessionInspectorMemorySection memory={props.memory} />;
    case "files":
      return <SessionInspectorFilesSection files={props.files} />;
    case "vault":
      return (
        <SessionVaultPanel
          error={props.vaultError}
          isLoading={props.vaultIsLoading}
          secrets={props.vaultSecrets}
          sessionId={props.sessionId}
        />
      );
    case "calls":
      return (
        <SessionCallsSection liveDataEnabled={props.liveDataEnabled} sessionId={props.sessionId} />
      );
  }
}

export function SessionInspector({
  messages,
  sessionId,
  liveDataEnabled = true,
  usage,
  memory,
  vaultSecrets = EMPTY_VAULT_SECRETS,
  vaultIsLoading = false,
  vaultError = null,
  files,
  drawerOpen,
  onDrawerOpenChange,
  className,
}: SessionInspectorProps) {
  const inspector = useSessionInspectorState(sessionId);
  const [localTab, setLocalTab] = useState<InspectorTabId>(DEFAULT_INSPECTOR_TAB);
  const activeTab = sessionId ? inspector.tab : localTab;
  const handleTabChange = (id: string) => {
    if (!isInspectorTabId(id)) return;
    if (sessionId) {
      inspector.selectTab(id);
      return;
    }
    setLocalTab(id);
  };
  const tabs = SESSION_INSPECTOR_TABS.map(tab => ({
    id: tab.id,
    label: <span data-testid={SESSION_INSPECTOR_TAB_TESTIDS[tab.id]}>{tab.label}</span>,
  }));

  return (
    <DetailInspector
      activeTab={activeTab}
      className={cn("min-w-0", className)}
      data-testid="session-inspector"
      onOpenChange={onDrawerOpenChange}
      onTabChange={handleTabChange}
      open={drawerOpen}
      tabs={tabs}
    >
      <div
        className="flex min-h-full flex-col gap-4 p-4"
        data-active-tab={activeTab}
        data-testid="session-inspector-panel"
      >
        <InspectorTabRenderer
          activeTab={activeTab}
          files={files ?? deriveFileReads(messages)}
          memory={memory ?? EMPTY_MEMORY_STATE}
          liveDataEnabled={liveDataEnabled}
          sessionId={sessionId}
          usage={usage}
          vaultError={vaultError}
          vaultIsLoading={vaultIsLoading}
          vaultSecrets={vaultSecrets}
        />
      </div>
    </DetailInspector>
  );
}
