import { Plus, Shield, Trash2 } from "lucide-react";

import { Button, ConfirmDialog, DataSurface, ListGroup, Section } from "@compozy/ui";

// Narrow entry: this section must not pull the emulator into settings.
import { TerminalGrantRow, terminalGrantFromToolGrant } from "@/systems/terminal/parts";

import { useToolApprovalGrantsPanel } from "../hooks/use-tool-approval-grants-panel";
import type { ToolApprovalGrant } from "../types";
import { ToolApprovalGrantSetDialog } from "./tool-approval-grant-set-dialog";
import { ToolApprovalGrantRow } from "./tool-approval-grant-row";

const TEST_ID = "settings-page-general-tool-approvals";

/**
 * Permissions → "Remembered decisions": the active workspace's remembered native-tool
 * approval decisions, with explicit wider set and per-row revoke. All states are local so
 * they never block editing the rest of general settings.
 */
export function ToolApprovalGrantsSection() {
  const { hasWorkspace, state, grants, total, error, onRetry, set, revoke } =
    useToolApprovalGrantsPanel();
  const target = revoke.target;

  return (
    <Section
      divided
      label="Remembered decisions"
      count={state === "ready" ? total : undefined}
      data-testid={`${TEST_ID}-section`}
      right={
        <Button
          data-testid={`${TEST_ID}-set-open`}
          disabled={!hasWorkspace}
          onClick={set.open}
          size="sm"
          type="button"
          variant="outline"
        >
          <Plus aria-hidden="true" className="size-3" />
          Set broader decision
        </Button>
      }
    >
      <DataSurface state={state}>
        <DataSurface.Loading
          data-testid={`${TEST_ID}-loading`}
          label="Loading remembered decisions"
        />
        <DataSurface.Error
          action={
            <Button onClick={onRetry} size="sm" type="button" variant="outline">
              Retry
            </Button>
          }
          data-testid={`${TEST_ID}-error`}
          description={error?.message}
          icon={Shield}
          title="Couldn't load remembered decisions"
        />
        <DataSurface.Empty
          data-testid={`${TEST_ID}-empty`}
          description="Set a broader decision, or choose Allow always or Reject always on a native-tool prompt in this workspace."
          icon={Shield}
          title="No remembered decisions yet"
        />
        <DataSurface.Content
          className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
          data-testid={`${TEST_ID}-list`}
        >
          {renderGrantRows(grants, revoke.open)}
        </DataSurface.Content>
      </DataSurface>
      <ToolApprovalGrantSetDialog
        canSubmit={set.canSubmit}
        draft={set.draft}
        error={set.error}
        isPending={set.isPending}
        onChange={set.change}
        onOpenChange={open => {
          if (!open) set.close();
        }}
        onSubmit={set.submit}
        open={set.isOpen}
      />
      <ConfirmDialog
        cancelButtonProps={{
          "data-testid": `${TEST_ID}-revoke-cancel`,
          disabled: revoke.isPending,
        }}
        cancelLabel="Cancel"
        confirmButtonProps={{ "data-testid": `${TEST_ID}-revoke-confirm` }}
        confirmIcon={Trash2}
        confirmLabel="Revoke decision"
        contentProps={{ "data-testid": `${TEST_ID}-revoke` }}
        description={revokeDescription(target)}
        error={revoke.error}
        errorProps={{ "data-testid": `${TEST_ID}-revoke-error` }}
        isPending={revoke.isPending}
        note={
          target
            ? `${target.decision}${target.input_digest ? " · exact input" : ""}${
                target.agent_name ? ` · agent ${target.agent_name}` : ""
              }`
            : undefined
        }
        onConfirm={revoke.confirm}
        onOpenChange={next => {
          if (!next) revoke.close();
        }}
        open={revoke.isOpen}
        title="Revoke this decision?"
        tone="danger"
      />
    </Section>
  );
}

function renderGrantRows(
  grants: ToolApprovalGrant[],
  onRevoke: (grant: ToolApprovalGrant) => void
) {
  const terminalGrants: {
    source: ToolApprovalGrant;
    grant: NonNullable<ReturnType<typeof terminalGrantFromToolGrant>>;
  }[] = [];
  const other: ToolApprovalGrant[] = [];
  for (const grant of grants) {
    const terminalGrant = terminalGrantFromToolGrant(grant);
    if (terminalGrant) {
      terminalGrants.push({ source: grant, grant: terminalGrant });
    } else {
      other.push(grant);
    }
  }
  return (
    <>
      {terminalGrants.length > 0 ? (
        <ListGroup
          className={other.length > 0 ? "border-b border-line" : undefined}
          data-testid={`${TEST_ID}-terminal-group`}
          label="Terminal"
        >
          {terminalGrants.map(({ source, grant }) => (
            <TerminalGrantRow grant={grant} key={source.id} onRevoke={() => onRevoke(source)} />
          ))}
        </ListGroup>
      ) : null}
      {other.map(grant => (
        <ToolApprovalGrantRow grant={grant} key={grant.id} onRevoke={onRevoke} />
      ))}
    </>
  );
}

function revokeDescription(target: ToolApprovalGrant | null): string | null {
  if (!target) return null;
  const terminal = terminalGrantFromToolGrant(target);
  if (terminal) {
    return terminal.kind === "typing"
      ? "CompozyOS will forget this typing permission in this project. The next keystroke in that terminal will ask again."
      : "CompozyOS will forget this exact command in this project. The next matching run will ask again.";
  }
  return `CompozyOS will forget this remembered approval for "${target.tool_id}" in this workspace. The next matching tool call will prompt for approval again.`;
}
