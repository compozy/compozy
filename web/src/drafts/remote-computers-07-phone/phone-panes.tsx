import { Bell, RefreshCw, WifiOff } from "@compozy/ui/icons";

import { Button, cn, CodeBlock, Empty, Pill, StateGlyph, Surface, toast } from "@compozy/ui";

import { KeyFingerprint } from "@/systems/computers/components/key-fingerprint";
import type {
  ComputerFixture,
  PendingApprovalFixture,
  UnifiedSessionFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { PHONE_TERMINAL_OUTPUT } from "@/systems/computers/mocks/remote-computers-fixtures-phone";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

function sessionStateLabel(state: UnifiedSessionFixture["state"]): string {
  switch (state) {
    case "attention":
      return "Needs you";
    case "running":
      return "Working";
    case "done":
      return "Done";
    case "idle":
      return "Idle";
  }
}

export function PhoneSessionList({
  sessions,
  filteredSessions,
  projects,
  projectFilter,
  offline,
  onSelectProject,
  onOpenSession,
}: {
  sessions: readonly UnifiedSessionFixture[];
  filteredSessions: readonly UnifiedSessionFixture[];
  projects: readonly string[];
  projectFilter: string;
  offline: boolean;
  onSelectProject: (project: string) => void;
  onOpenSession: (sessionId: string) => void;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center gap-1.5 overflow-x-auto border-b border-line-soft px-3.5 py-2">
        <Pill
          active={projectFilter === "all"}
          onClick={() => onSelectProject("all")}
          render={<button type="button" />}
          size="sm"
          tone="neutral"
        >
          All · {sessions.length}
        </Pill>
        {projects.map(project => (
          <Pill
            active={projectFilter === project}
            key={project}
            onClick={() => onSelectProject(project)}
            render={<button type="button" />}
            size="sm"
            tone="neutral"
          >
            {project}
          </Pill>
        ))}
      </div>

      <div
        className={cn(
          "flex min-h-0 flex-1 flex-col divide-y divide-line-soft overflow-y-auto",
          offline && "opacity-70"
        )}
      >
        {filteredSessions.map(item => {
          const needsYou = item.state === "attention";
          return (
            <button
              className={cn(
                "flex min-h-15 w-full items-start gap-3 px-4 py-3 text-left transition-colors hover:bg-surface-2 focus-visible:shadow-focus-inset focus-visible:outline-none"
              )}
              key={item.id}
              onClick={() => onOpenSession(item.id)}
              type="button"
            >
              <span className="mt-1 shrink-0">
                <StateGlyph size="md" state={offline ? "idle" : item.state} still={offline} />
              </span>
              <div className="flex min-w-0 flex-1 flex-col gap-1">
                <div className="flex items-baseline justify-between gap-2">
                  <span
                    className={cn(
                      "min-w-0 truncate text-body text-fg",
                      needsYou && "font-medium text-fg-strong"
                    )}
                  >
                    {item.title}
                  </span>
                  <span className="shrink-0 font-mono text-micro text-subtle tabular-nums">
                    {item.updatedLabel}
                  </span>
                </div>
                <div className="flex items-center gap-1.5 text-eyebrow text-subtle">
                  <WorkspaceBadge identity={{ kind: "project", name: item.project }} />
                  <span aria-hidden="true">·</span>
                  <span className={cn(needsYou && !offline && "font-medium text-attn")}>
                    {offline ? "Offline" : sessionStateLabel(item.state)}
                  </span>
                  <span aria-hidden="true">·</span>
                  <span className="truncate">{item.model}</span>
                </div>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

export function PhoneTerminalPane({
  computer,
  offline,
}: {
  computer: ComputerFixture;
  offline: boolean;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-3 px-3.5 py-3.5">
        <div className="flex items-center justify-between gap-2">
          <div className="flex flex-col">
            <span className="text-body font-medium text-fg-strong">zsh on {computer.name}</span>
            <span className="font-mono text-micro text-subtle">/Users/pedro/code/api</span>
          </div>
          <Pill form="plain" tone={offline ? "warning" : "success"}>
            <Pill.Dot />
            {offline ? "Disconnected" : "Live"}
          </Pill>
        </div>
        <CodeBlock
          code={PHONE_TERMINAL_OUTPUT.join("\n")}
          copyable
          density="compact"
          language="bash"
          wrapLines
        />
        <p className="text-eyebrow text-subtle">
          Commands run directly on {computer.name} under your user account. Output stays buffered if
          your connection drops briefly.
        </p>
      </div>
    </div>
  );
}

export function PhoneApprovalsPane({
  computer,
  approvals,
  onDecide,
}: {
  computer: ComputerFixture;
  approvals: readonly PendingApprovalFixture[];
  onDecide: (approvalId: string, decision: "approved" | "denied", deviceName: string) => void;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div className="mx-auto flex w-full max-w-xl flex-col gap-3 px-3.5 py-3.5">
        {approvals.length === 0 ? (
          <Empty
            description={`New device requests for ${computer.name} appear here.`}
            icon={Bell}
            size="compact"
            title="No pending approvals"
          />
        ) : (
          approvals.map(item => (
            <Surface className="flex flex-col gap-3 p-4" key={item.id} variant="card">
              <div className="flex items-start justify-between gap-2">
                <div className="flex min-w-0 flex-col gap-0.5">
                  <span className="truncate text-body font-medium text-fg-strong">
                    {item.deviceName}
                  </span>
                  <span className="text-eyebrow text-subtle">
                    Wants to access {item.computerName} · {item.createdLabel}
                  </span>
                </div>
                <Pill
                  form="tint"
                  tone={
                    item.state === "approved"
                      ? "success"
                      : item.state === "denied"
                        ? "destructive"
                        : "warning"
                  }
                >
                  {item.state === "approved"
                    ? "Approved"
                    : item.state === "denied"
                      ? "Denied"
                      : "Waiting"}
                </Pill>
              </div>

              <KeyFingerprint
                columns={3}
                label="Device fingerprint"
                layout="grid"
                value={item.keyFingerprint}
              />

              {item.canDecide ? (
                <div className="grid grid-cols-2 gap-2">
                  <Button
                    disabled={item.state !== "pending"}
                    onClick={() => {
                      onDecide(item.id, "denied", item.deviceName);
                      toast.message(`Denied ${item.deviceName}`);
                    }}
                    size="lg"
                    variant="outline"
                  >
                    Deny
                  </Button>
                  <Button
                    disabled={item.state !== "pending"}
                    onClick={() => {
                      onDecide(item.id, "approved", item.deviceName);
                      toast.success(`Approved ${item.deviceName}`);
                    }}
                    size="lg"
                  >
                    Approve
                  </Button>
                </div>
              ) : (
                <p className="text-eyebrow text-subtle">
                  Approve on {item.computerName}, or from a device it already trusts.
                </p>
              )}
            </Surface>
          ))
        )}
      </div>
    </div>
  );
}

export function PhoneOfflineStrip({
  computerName,
  onRetry,
}: {
  computerName: string;
  onRetry?: () => void;
}) {
  return (
    <div
      className="flex shrink-0 items-center justify-between gap-2 border-b border-line-soft bg-warning-tint px-3.5 py-2"
      data-slot="phone-offline-strip"
      role="status"
    >
      <div className="flex min-w-0 items-center gap-2 text-eyebrow text-fg">
        <WifiOff aria-hidden="true" className="size-3.5 shrink-0 text-warning" />
        <span className="truncate">{computerName} is offline. Reconnecting…</span>
      </div>
      <Button className="shrink-0" onClick={onRetry} size="xs" variant="secondary">
        <RefreshCw className="size-3" />
        Retry now
      </Button>
    </div>
  );
}
