import { Plus, Shield, Trash2 } from "lucide-react";

import { Button, ConfirmDialog, DataSurface, Section } from "@agh/ui";

import { useToolApprovalGrantsPanel } from "../hooks/use-tool-approval-grants-panel";
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
      note="Native-tool approvals AGH remembers for this workspace"
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
          {grants.map(grant => (
            <ToolApprovalGrantRow grant={grant} key={grant.id} onRevoke={revoke.open} />
          ))}
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
        description={
          target
            ? `AGH will forget this remembered approval for "${target.tool_id}" in this workspace. The next matching tool call will prompt for approval again.`
            : null
        }
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
