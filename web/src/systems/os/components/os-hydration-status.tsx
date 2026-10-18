import { AlertTriangle, CloudOff, RefreshCw } from "lucide-react";

import { Icon, Pill, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { WindowManagerApiError } from "../adapters/window-manager-api";
import type { OsHydration } from "../lib/os-types";
import type { WindowManagerConnectionStatus } from "../lib/window-manager-types";
import type { WindowManagerDiagnostic } from "../stores/window-manager-store-types";

export interface OsHydrationStatusProps {
  hydration: OsHydration;
  loadError?: Error | null;
  connectionStatus?: WindowManagerConnectionStatus;
  /** Latest refused-command notice or stream diagnostic, if any. */
  diagnostic?: WindowManagerDiagnostic | null;
  /** No project is bound, so there is no layout stream to report on. */
  unbound?: boolean;
}

interface StatusView {
  tone: "neutral" | "warning";
  icon: typeof CloudOff;
  label: string;
  /** Technical detail kept one step deeper than the visible label. */
  detail: string | null;
}

function statusView({
  hydration,
  loadError = null,
  connectionStatus = "connected",
  diagnostic = null,
  unbound = false,
}: OsHydrationStatusProps): StatusView | null {
  if (unbound || hydration === "pending") return null;
  if (
    loadError instanceof WindowManagerApiError &&
    loadError.payload?.code === "profile_unavailable"
  ) {
    return {
      tone: "warning",
      icon: AlertTriangle,
      label: "Profile needs recovery — choose another profile",
      detail: loadError.payload.diagnostics[0]?.message ?? loadError.message,
    };
  }
  if (hydration === "degraded" || connectionStatus === "disconnected") {
    return {
      tone: "warning",
      icon: CloudOff,
      label: "Can't save window layout — retrying",
      detail: diagnostic?.message ?? null,
    };
  }
  // `idle`: the stream has not been attempted yet (startup, registration) —
  // not a failure, so it only yields to a refused-command notice below.
  if (connectionStatus !== "connected" && connectionStatus !== "idle") {
    return { tone: "neutral", icon: RefreshCw, label: "Reconnecting…", detail: null };
  }
  // A refused command's notice is already written for people; it keeps the
  // rollback from disappearing without a word while the stream is healthy.
  if (diagnostic) {
    return {
      tone: diagnostic.severity === "info" ? "neutral" : "warning",
      icon: AlertTriangle,
      label: diagnostic.message,
      detail: null,
    };
  }
  return null;
}

/**
 * The one window-layout status indicator: stream health and refused-command
 * notices share a single menubar pill so the desk never shows a second one.
 */
export function OsHydrationStatus(props: OsHydrationStatusProps) {
  const view = statusView(props);
  if (!view) return null;

  const pill = (
    <Pill
      role="status"
      aria-atomic="true"
      aria-live="polite"
      data-testid="os-window-manager-status"
      form="plain"
      tone={view.tone}
      size="sm"
      className="shrink-0 gap-1.5"
    >
      <Icon
        as={view.icon}
        size="sm"
        aria-hidden="true"
        className={view.tone === "warning" ? "text-warning" : undefined}
      />
      {view.label}
    </Pill>
  );
  if (!view.detail) return pill;
  return (
    <Tooltip>
      <TooltipTrigger render={pill} />
      <TooltipContent side="bottom">{view.detail}</TooltipContent>
    </Tooltip>
  );
}
