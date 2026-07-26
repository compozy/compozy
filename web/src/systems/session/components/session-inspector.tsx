import { useState } from "react";

import { DetailInspector, cn } from "@agh/ui";
import { SessionVaultPanel, type VaultSecret } from "@/systems/vault";

import {
  deriveFileReads,
  deriveTraceEvents,
  TRACE_LIMIT_DEFAULT,
  type InspectorFileEntry,
  type InspectorTraceEvent,
  type ThreadMessageState,
} from "./session-inspector.logic";
import { SessionInspectorMemorySection } from "./session-inspector-memory";
import {
  SessionInspectorFilesSection,
  SessionInspectorTraceSection,
  SessionInspectorUsageSection,
} from "./session-inspector-sections";
import type { InspectorMemoryState, InspectorUsage } from "./session-inspector-types";

export type {
  InspectorMemoryState,
  InspectorSessionLedger,
  InspectorUsage,
} from "./session-inspector-types";

const EMPTY_VAULT_SECRETS: readonly VaultSecret[] = [];
const EMPTY_MEMORY_STATE: InspectorMemoryState = Object.freeze({});

type InspectorTabId = "trace" | "usage" | "memory" | "files" | "vault";

const SESSION_INSPECTOR_TABS = [
  { id: "trace", label: "Trace" },
  { id: "usage", label: "Usage" },
  { id: "memory", label: "Memory" },
  { id: "files", label: "Files" },
  { id: "vault", label: "Vault" },
] as const satisfies ReadonlyArray<{ id: InspectorTabId; label: string }>;

const SESSION_INSPECTOR_TAB_TESTIDS: Record<InspectorTabId, string> = {
  trace: "session-inspector-tab-trace",
  usage: "session-inspector-tab-usage",
  memory: "session-inspector-tab-memory",
  files: "session-inspector-tab-files",
  vault: "session-inspector-tab-vault",
};

export interface SessionInspectorProps {
  messages: readonly ThreadMessageState[];
  sessionId?: string;
  usage?: InspectorUsage | null;
  memory?: InspectorMemoryState;
  vaultSecrets?: readonly VaultSecret[];
  vaultIsLoading?: boolean;
  vaultError?: Error | null;
  files?: InspectorFileEntry[];
  totalTraceEvents?: number;
  traceLimit?: number;
  onViewAllTrace?: () => void;
  drawerOpen?: boolean;
  onDrawerOpenChange?: (open: boolean) => void;
  className?: string;
}

interface InspectorTabRendererProps {
  activeTab: InspectorTabId;
  traceEvents: InspectorTraceEvent[];
  traceTotal: number;
  traceLimit: number;
  onViewAllTrace?: () => void;
  usage: InspectorUsage | null | undefined;
  memory: InspectorMemoryState;
  sessionId?: string;
  vaultSecrets: readonly VaultSecret[];
  vaultIsLoading: boolean;
  vaultError: Error | null;
  files: InspectorFileEntry[];
}

function InspectorTabRenderer(props: InspectorTabRendererProps) {
  switch (props.activeTab) {
    case "trace":
      return (
        <SessionInspectorTraceSection
          events={props.traceEvents}
          total={props.traceTotal}
          limit={props.traceLimit}
          onViewAll={props.onViewAllTrace}
        />
      );
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
  }
}

export function SessionInspector({
  messages,
  sessionId,
  usage,
  memory,
  vaultSecrets = EMPTY_VAULT_SECRETS,
  vaultIsLoading = false,
  vaultError = null,
  files,
  totalTraceEvents,
  traceLimit = TRACE_LIMIT_DEFAULT,
  onViewAllTrace,
  drawerOpen,
  onDrawerOpenChange,
  className,
}: SessionInspectorProps) {
  const [activeTab, setActiveTab] = useState<InspectorTabId>("trace");
  const handleTabChange = (id: string) => {
    if (SESSION_INSPECTOR_TABS.some(tab => tab.id === id)) setActiveTab(id as InspectorTabId);
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
          onViewAllTrace={onViewAllTrace}
          sessionId={sessionId}
          traceEvents={deriveTraceEvents(messages, traceLimit)}
          traceLimit={traceLimit}
          traceTotal={totalTraceEvents ?? messages.length}
          usage={usage}
          vaultError={vaultError}
          vaultIsLoading={vaultIsLoading}
          vaultSecrets={vaultSecrets}
        />
      </div>
    </DetailInspector>
  );
}
