import { ChevronUp } from "lucide-react";

import { Button, cn, Dock, Kbd } from "@compozy/ui";

// Narrow entry: the dock must not pull the emulator into every session bundle.
import { TerminalApprovalDetail, terminalPermissionDetail } from "@/systems/terminal/parts";

import { usePermissionDock } from "../hooks/use-permission-dock";
import { useRejectMenuElements } from "../hooks/use-reject-menu-elements";
import type { PermissionRequest } from "../types";

const DANGER_GHOST_CLASS = "text-muted hover:bg-danger-tint hover:text-danger";

export interface PermissionDockProps {
  enabled?: boolean;
  permission: PermissionRequest;
  sessionId: string;
  workspaceId: string;
  /** "1/N" when more decisions wait behind this one. */
  countLabel: string | null;
  onResolved: () => void;
}

/**
 * The pending permission as a composer-docked decision panel. Buttons render
 * only for the decisions the runtime offers; reject-always lives behind the
 * reject split menu; digit keys 1–4 decide directly (ignoring focused inputs).
 */
export function PermissionDock({
  enabled = true,
  permission,
  sessionId,
  workspaceId,
  countLabel,
  onResolved,
}: PermissionDockProps) {
  const { menuItemRef, menuOpen, setMenuOpen, splitElement, splitRef, triggerRef } =
    useRejectMenuElements();
  // The runtime classification governs both the visible actions and keyboard
  // shortcuts. A hidden remembered decision must not remain reachable by key.
  const terminalDetail = terminalPermissionDetail(
    permission.toolId ?? permission.toolName,
    permission.toolInput
  );
  const blockedDecisions =
    terminalDetail?.kind === "exec" && terminalDetail.risk !== "ordinary"
      ? (["allow-always"] as const)
      : [];
  const { decide, decisionOptions, isResolved, isSubmitting, subject } = usePermissionDock({
    enabled,
    permission,
    sessionId,
    workspaceId,
    onResolved,
    rejectSplitElement: splitElement,
    menuOpen,
    setMenuOpen,
    blockedDecisions,
  });

  if (isResolved) {
    return null;
  }

  const offersRejectOnce = decisionOptions.includes("reject-once");
  // A terminal ask carries facts the generic subject line cannot show: the exact
  // command, where it would run, and why the runtime is asking.
  const offersRejectAlways =
    decisionOptions.includes("reject-always") && terminalDetail?.kind !== "typing";

  return (
    <Dock data-testid="permission-dock" role="region" aria-label="Permission required">
      <Dock.Head>
        <Dock.Eyebrow data-testid="permission-dock-eyebrow">Permission</Dock.Eyebrow>
        <Dock.Title data-testid="permission-dock-title">{permission.toolName}</Dock.Title>
        {countLabel ? (
          <Dock.Count data-testid="permission-dock-count">{countLabel}</Dock.Count>
        ) : null}
      </Dock.Head>
      <Dock.Body>
        {terminalDetail ? (
          <TerminalApprovalDetail detail={terminalDetail} />
        ) : subject ? (
          <Dock.Pre data-testid="permission-dock-subject">{subject}</Dock.Pre>
        ) : null}
        {permission.action || permission.resource ? (
          <Dock.Meta data-testid="permission-dock-meta">
            {permission.action}
            {permission.action && permission.resource ? " · " : ""}
            {permission.resource ? <code>{permission.resource}</code> : null}
          </Dock.Meta>
        ) : null}
      </Dock.Body>
      <Dock.Actions>
        {decisionOptions.includes("allow-once") ? (
          <Button
            size="sm"
            variant="primary"
            disabled={isSubmitting}
            onClick={() => decide("allow-once")}
            data-testid="permission-allow-once"
          >
            Allow once
            <Dock.Key>1</Dock.Key>
          </Button>
        ) : null}
        {/* No remembered decision covers the fixed irreversible set (US-022),
            so the option is absent here rather than offered and refused. */}
        {decisionOptions.includes("allow-always") ? (
          <Button
            size="sm"
            variant="outline"
            disabled={isSubmitting}
            onClick={() => decide("allow-always")}
            data-testid="permission-allow-always"
          >
            {terminalDetail?.kind === "typing" ? "This terminal" : "Always allow"}
            <Dock.Key>2</Dock.Key>
          </Button>
        ) : null}
        <span className="flex-1" />
        {offersRejectOnce ? (
          <div ref={splitRef} className="relative inline-flex gap-px" data-open={menuOpen}>
            <Button
              size="sm"
              variant="ghost"
              className={DANGER_GHOST_CLASS}
              disabled={isSubmitting}
              onClick={() => decide("reject-once")}
              data-testid="permission-reject-once"
            >
              Reject
              <Dock.Key>3</Dock.Key>
            </Button>
            {offersRejectAlways ? (
              <>
                <Button
                  ref={triggerRef}
                  size="sm"
                  variant="ghost"
                  className={cn(DANGER_GHOST_CLASS, "px-1")}
                  disabled={isSubmitting}
                  aria-expanded={menuOpen}
                  aria-label="More reject options"
                  onClick={() => setMenuOpen(open => !open)}
                  data-testid="permission-reject-menu-trigger"
                >
                  <ChevronUp
                    aria-hidden="true"
                    className={cn(
                      "size-3 transition-transform duration-slow ease-out motion-reduce:transition-none",
                      menuOpen ? "rotate-180" : null
                    )}
                  />
                </Button>
                {menuOpen ? (
                  <div
                    role="menu"
                    data-testid="permission-reject-menu"
                    className="absolute right-0 bottom-[calc(100%+5px)] z-10 min-w-[158px] rounded-md border border-line-strong bg-elevated p-[3px] shadow-[var(--shadow-overlay)]"
                  >
                    <button
                      ref={menuItemRef}
                      type="button"
                      role="menuitem"
                      data-testid="permission-reject-always"
                      disabled={isSubmitting}
                      className="flex min-h-7 w-full items-center gap-2 rounded-xs px-2 py-transcript-meta-gap text-left text-transcript-body text-danger transition-colors hover:bg-hover"
                      onClick={() => decide("reject-always")}
                    >
                      Reject always
                      <Kbd className="ml-auto">4</Kbd>
                    </button>
                  </div>
                ) : null}
              </>
            ) : null}
          </div>
        ) : offersRejectAlways ? (
          <Button
            size="sm"
            variant="ghost"
            className={DANGER_GHOST_CLASS}
            disabled={isSubmitting}
            onClick={() => decide("reject-always")}
            data-testid="permission-reject-always"
          >
            Reject always
            <Dock.Key>4</Dock.Key>
          </Button>
        ) : null}
      </Dock.Actions>
      {isSubmitting ? (
        <Dock.Status role="status" data-testid="permission-dock-status">
          Submitting decision…
        </Dock.Status>
      ) : null}
    </Dock>
  );
}
